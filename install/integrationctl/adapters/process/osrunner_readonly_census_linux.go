//go:build linux

package process

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"unicode/utf8"

	"golang.org/x/sys/unix"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
)

const readonlyInputEntryLimit = 4096

type readonlyCensusProof struct {
	SHA256 string
	Count  int
}

type readonlyInputCensus struct {
	rows     map[string]readonlyObjectFacts
	uid, gid uint32
	mount    uint64
}

// Census describes the original ordinary input tree. It is a bounded stable
// observation, not an atomic snapshot against a hostile concurrent writer.
func censusReadonlyInputs(ctx context.Context, inputs readonlyOriginalInputs) (readonlyCensusProof, error) {
	census := readonlyInputCensus{rows: make(map[string]readonlyObjectFacts), uid: inputs.home.facts.UID, gid: inputs.home.facts.GID}
	roots := []readonlyOriginalObject{inputs.home, inputs.worktree}
	for index, root := range roots {
		if err := root.revalidate(ctx); err != nil {
			return readonlyCensusProof{}, err
		}
		if index == 1 && readonlyWithin(root.path, inputs.home.path) {
			continue
		}
		if index == 0 && readonlyWithin(root.path, inputs.worktree.path) && root.path != inputs.worktree.path {
			continue
		}
		if err := census.root(ctx, root); err != nil {
			return readonlyCensusProof{}, err
		}
	}
	for _, role := range []readonlyOriginalObject{inputs.home, inputs.worktree, inputs.project, inputs.executable} {
		if err := role.revalidate(ctx); err != nil {
			return readonlyCensusProof{}, err
		}
	}
	return digestReadonlyCensus(ctx, census.rows)
}

func (census *readonlyInputCensus) root(ctx context.Context, root readonlyOriginalObject) (resultErr error) {
	file, err := openReadonlyDirectory(root.file, ".")
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	if err := requireReadonlyHeld(file, root.facts); err != nil {
		return err
	}
	if err := census.directory(ctx, file, root.path, root.facts, 0); err != nil {
		return err
	}
	return errors.Join(requireReadonlyMount(nil, root.path, 0, census.mount), root.revalidate(ctx))
}

func openReadonlyDirectory(parent *os.File, name string) (*os.File, error) {
	fd, err := unix.Openat(int(parent.Fd()), name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	runtime.KeepAlive(parent)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), "readonly-census-directory"), nil
}

func readonlyNamedFacts(parent *os.File, name string) (readonlyObjectFacts, error) {
	var stat unix.Stat_t
	err := unix.Fstatat(int(parent.Fd()), name, &stat, unix.AT_SYMLINK_NOFOLLOW)
	runtime.KeepAlive(parent)
	if err != nil {
		return readonlyObjectFacts{}, err
	}
	return readonlyObjectFacts{Device: stat.Dev, Inode: stat.Ino, Links: readonlyLinkCount(stat.Nlink),
		Mode: stat.Mode, UID: stat.Uid, GID: stat.Gid, Size: stat.Size,
		Mtime: syscall.Timespec{Sec: stat.Mtim.Sec, Nsec: stat.Mtim.Nsec},
		Ctime: syscall.Timespec{Sec: stat.Ctim.Sec, Nsec: stat.Ctim.Nsec}}, nil
}

func requireReadonlyHeld(file *os.File, expected readonlyObjectFacts) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	facts, err := readonlyFacts(info)
	if err != nil {
		return err
	}
	if facts != expected {
		return directoryidentity.ErrChanged
	}
	return nil
}

func requireReadonlyNamed(parent *os.File, name string, expected readonlyObjectFacts) error {
	facts, err := readonlyNamedFacts(parent, name)
	if err != nil {
		return err
	}
	if facts != expected {
		return directoryidentity.ErrChanged
	}
	return nil
}

func readonlyMountID(parent *os.File, name string, flags int) (uint64, error) {
	fd := unix.AT_FDCWD
	if parent != nil {
		fd = int(parent.Fd())
	}
	var stat unix.Statx_t
	err := unix.Statx(fd, name, flags|unix.AT_SYMLINK_NOFOLLOW, unix.STATX_MNT_ID, &stat)
	runtime.KeepAlive(parent)
	if err != nil {
		return 0, err
	}
	if stat.Mask&unix.STATX_MNT_ID == 0 || stat.Mnt_id == 0 {
		return 0, errors.New("readonly original mount identity is unavailable")
	}
	return stat.Mnt_id, nil
}

func requireReadonlyMount(parent *os.File, name string, flags int, expected uint64) error {
	actual, err := readonlyMountID(parent, name, flags)
	if err != nil {
		return err
	}
	if actual != expected {
		return errors.New("readonly input contains an unadmitted mount")
	}
	return nil
}

func (census *readonlyInputCensus) record(path string, facts readonlyObjectFacts) error {
	if !readonlyAbsolute(path) || len(census.rows) >= readonlyInputEntryLimit {
		return errors.New("readonly input census bound exceeded")
	}
	if _, exists := census.rows[path]; exists {
		return errors.New("readonly input census contains overlapping paths")
	}
	if err := admitReadonlyFacts(facts, facts.Mode&unix.S_IFMT == unix.S_IFDIR, census.uid, census.gid); err != nil {
		return err
	}
	census.rows[path] = facts
	return nil
}

func (census *readonlyInputCensus) directory(ctx context.Context, file *os.File, path string, before readonlyObjectFacts, depth int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if depth > 32 {
		return errors.New("readonly input directory depth exceeded")
	}
	if depth == 0 {
		mount, err := readonlyMountID(file, "", unix.AT_EMPTY_PATH)
		if err != nil {
			return err
		}
		census.mount = mount
		if err := requireReadonlyMount(nil, path, 0, mount); err != nil {
			return err
		}
	}
	if err := census.record(path, before); err != nil {
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries, err := file.ReadDir(1)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if len(entries) != 1 || len(census.rows) >= readonlyInputEntryLimit {
			return errors.New("readonly input directory count exceeded")
		}
		if err := census.entry(ctx, file, path, entries[0].Name(), depth); err != nil {
			return err
		}
	}
	return errors.Join(requireReadonlyHeld(file, before), requireReadonlyMount(file, "", unix.AT_EMPTY_PATH, census.mount), ctx.Err())
}

func (census *readonlyInputCensus) entry(ctx context.Context, parent *os.File, path, name string, depth int) error {
	if name == "" || name == "." || name == ".." || !utf8.ValidString(name) || strings.ContainsAny(name, "/\x00\r\n") {
		return errors.New("readonly input member name differs")
	}
	if err := requireReadonlyMount(parent, name, 0, census.mount); err != nil {
		return err
	}
	before, err := readonlyNamedFacts(parent, name)
	if err != nil {
		return err
	}
	path = filepath.Join(path, name)
	if before.Mode&unix.S_IFMT == unix.S_IFDIR {
		return census.childDirectory(ctx, parent, name, path, before, depth+1)
	}
	if err := census.record(path, before); err != nil {
		return err
	}
	return errors.Join(requireReadonlyNamed(parent, name, before), requireReadonlyMount(parent, name, 0, census.mount))
}

func (census *readonlyInputCensus) childDirectory(ctx context.Context, parent *os.File, name, path string, before readonlyObjectFacts, depth int) (resultErr error) {
	if err := admitReadonlyFacts(before, true, census.uid, census.gid); err != nil {
		return err
	}
	file, err := openReadonlyDirectory(parent, name)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	if err := requireReadonlyHeld(file, before); err != nil {
		return err
	}
	if err := census.directory(ctx, file, path, before, depth); err != nil {
		return err
	}
	return errors.Join(requireReadonlyNamed(parent, name, before), requireReadonlyMount(parent, name, 0, census.mount))
}

func digestReadonlyCensus(ctx context.Context, rows map[string]readonlyObjectFacts) (readonlyCensusProof, error) {
	if len(rows) == 0 || len(rows) > readonlyInputEntryLimit {
		return readonlyCensusProof{}, errors.New("readonly census digest count differs")
	}
	keys := make([]string, 0, len(rows))
	for path := range rows {
		keys = append(keys, path)
	}
	sort.Strings(keys)
	hash := sha256.New()
	for _, path := range keys {
		if err := ctx.Err(); err != nil {
			return readonlyCensusProof{}, err
		}
		facts := rows[path]
		_, _ = fmt.Fprintf(hash, "%s\x00%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%d\n", path,
			facts.Device, facts.Inode, facts.Mode, facts.Links, facts.UID, facts.GID, facts.Size,
			facts.Mtime.Sec, facts.Mtime.Nsec, facts.Ctime.Sec, facts.Ctime.Nsec)
	}
	return readonlyCensusProof{SHA256: hex.EncodeToString(hash.Sum(nil)), Count: len(rows)}, nil
}
