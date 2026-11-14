//go:build linux

package process

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unicode/utf8"

	"golang.org/x/sys/unix"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// These are immutable metadata observations, not durable ownership or a
// claim that a caller's native discovery inputs form a complete closure.
type readonlyObjectFacts struct {
	Device, Inode, Links uint64
	Mode, UID, GID       uint32
	Size                 int64
	Mtime, Ctime         syscall.Timespec
}

type readonlyOriginalObject struct {
	path      string
	file      *os.File
	facts     readonlyObjectFacts
	parent    directoryidentity.Authority
	directory bool
}

type readonlyOriginalInputs struct {
	home, worktree, project, executable readonlyOriginalObject
}

func readonlyFacts(info os.FileInfo) (readonlyObjectFacts, error) {
	if info == nil {
		return readonlyObjectFacts{}, errors.New("readonly original metadata is required")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil || stat.Ino == 0 || stat.Nlink == 0 || stat.Size < 0 {
		return readonlyObjectFacts{}, errors.New("readonly original metadata is unavailable")
	}
	return readonlyObjectFacts{Device: stat.Dev, Inode: stat.Ino, Links: stat.Nlink,
		Mode: stat.Mode, UID: stat.Uid, GID: stat.Gid, Size: stat.Size, Mtime: stat.Mtim, Ctime: stat.Ctim}, nil
}

func readonlyAbsolute(path string) bool {
	return path != "/" && filepath.IsAbs(path) && filepath.Clean(path) == path && len(path) <= 4096 && utf8.ValidString(path) &&
		!strings.ContainsAny(path, "\x00\r\n") && strings.Count(path, "/") <= 32
}

func readonlyWithin(path, root string) bool {
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

func admitReadonlyOriginal(ctx context.Context, original ports.ReadonlyOriginal, directory bool, uid, gid uint32) (readonlyOriginalObject, error) {
	if err := ctx.Err(); err != nil {
		return readonlyOriginalObject{}, err
	}
	if original.File == nil || !readonlyAbsolute(original.Path) {
		return readonlyOriginalObject{}, errors.New("readonly original handle and canonical path are required")
	}
	want, err := readonlyFacts(original.Info)
	if err != nil {
		return readonlyOriginalObject{}, err
	}
	if err := admitReadonlyFacts(want, directory, uid, gid); err != nil {
		return readonlyOriginalObject{}, err
	}
	object := readonlyOriginalObject{path: original.Path, file: original.File, facts: want, directory: directory}
	if err := object.joinMetadata(); err != nil {
		return readonlyOriginalObject{}, err
	}
	if directory {
		object.parent, err = directoryidentity.CaptureHeld(ctx, original.Path, original.File)
	} else {
		object.parent, err = directoryidentity.Capture(ctx, filepath.Dir(original.Path))
	}
	if err != nil || object.parent.Facts().CanonicalRoot != object.parentPath() {
		return readonlyOriginalObject{}, errors.Join(err, errors.New("readonly original ancestry differs"))
	}
	if err := object.revalidate(ctx); err != nil {
		return readonlyOriginalObject{}, err
	}
	return object, nil
}

func admitReadonlyFacts(facts readonlyObjectFacts, directory bool, uid, gid uint32) error {
	if facts.Inode == 0 || facts.Links == 0 || facts.Size < 0 || facts.Mtime.Nsec < 0 || facts.Mtime.Nsec >= 1e9 ||
		facts.Ctime.Nsec < 0 || facts.Ctime.Nsec >= 1e9 {
		return errors.New("readonly original metadata is incomplete")
	}
	kind := uint32(unix.S_IFREG)
	if directory {
		kind = unix.S_IFDIR
	}
	if facts.Mode&unix.S_IFMT != kind || facts.Mode&07022 != 0 || facts.UID != uid || facts.GID != gid ||
		!directory && (facts.Links != 1 || facts.Size > 512<<20) {
		return errors.New("readonly original type, mode or ownership differs")
	}
	return nil
}

func (object readonlyOriginalObject) parentPath() string {
	if object.directory {
		return object.path
	}
	return filepath.Dir(object.path)
}

func (object readonlyOriginalObject) joinMetadata() error {
	held, heldErr := object.file.Stat()
	named, namedErr := os.Lstat(object.path)
	if err := errors.Join(heldErr, namedErr); err != nil {
		return err
	}
	heldFacts, heldErr := readonlyFacts(held)
	namedFacts, namedErr := readonlyFacts(named)
	if err := errors.Join(heldErr, namedErr); err != nil {
		return err
	}
	if heldFacts != object.facts || namedFacts != object.facts {
		return directoryidentity.ErrChanged
	}
	return nil
}

func (object readonlyOriginalObject) revalidate(ctx context.Context) error {
	if err := directoryidentity.Revalidate(ctx, object.parent); err != nil {
		return err
	}
	if err := object.joinMetadata(); err != nil {
		return err
	}
	return ctx.Err()
}

func readonlyCallerIDs() (uint32, uint32, error) {
	uid, gid := os.Getuid(), os.Getgid()
	if uid < 0 || gid < 0 || uint64(uid) > math.MaxUint32 || uint64(gid) > math.MaxUint32 {
		return 0, 0, errors.New("readonly caller identity exceeds its kernel bound")
	}
	return uint32(uid), uint32(gid), nil
}

func admitReadonlyInputs(ctx context.Context, command ports.Command, inputs ports.ReadonlyDuplexInputs) (readonlyOriginalInputs, error) {
	if err := ctx.Err(); err != nil {
		return readonlyOriginalInputs{}, err
	}
	if len(command.Argv) == 0 || command.Argv[0] != inputs.Executable.Path || command.Dir != inputs.Project.Path ||
		!readonlyWithin(inputs.Project.Path, inputs.Worktree.Path) {
		return readonlyOriginalInputs{}, errors.New("readonly original command roles differ")
	}
	uid, gid, err := readonlyCallerIDs()
	if err != nil {
		return readonlyOriginalInputs{}, err
	}
	result := readonlyOriginalInputs{}
	roles := []struct {
		input ports.ReadonlyOriginal
		out   *readonlyOriginalObject
	}{{inputs.Home, &result.home}, {inputs.Worktree, &result.worktree}, {inputs.Project, &result.project}}
	for _, role := range roles {
		value, err := admitReadonlyOriginal(ctx, role.input, true, uid, gid)
		if err != nil {
			return readonlyOriginalInputs{}, err
		}
		*role.out = value
	}
	executable, err := admitReadonlyExecutable(ctx, inputs.Executable, uid, gid)
	if err != nil {
		return readonlyOriginalInputs{}, err
	}
	result.executable = executable
	return result, ctx.Err()
}

func admitReadonlyExecutable(ctx context.Context, input ports.ReadonlyOriginal, uid, gid uint32) (readonlyOriginalObject, error) {
	facts, err := readonlyFacts(input.Info)
	if err != nil {
		return readonlyOriginalObject{}, err
	}
	if readonlyWithin(input.Path, "/usr") {
		uid, gid = 0, 0
	}
	if facts.Mode&0111 == 0 {
		return readonlyOriginalObject{}, errors.New("readonly native executable lacks executable mode")
	}
	object, err := admitReadonlyOriginal(ctx, input, false, uid, gid)
	if err != nil {
		return readonlyOriginalObject{}, err
	}
	if err := readonlyNoFileCapability(object.file); err != nil {
		return readonlyOriginalObject{}, err
	}
	return object, nil
}

func readonlyNoFileCapability(file *os.File) error {
	_, err := unix.Fgetxattr(int(file.Fd()), "security.capability", nil)
	runtime.KeepAlive(file)
	if !errors.Is(err, unix.ENODATA) {
		return fmt.Errorf("readonly file capability is not absent: %w", errors.Join(err, errors.New("readonly capability refused")))
	}
	return nil
}
