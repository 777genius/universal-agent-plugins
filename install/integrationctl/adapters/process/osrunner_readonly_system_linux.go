//go:build linux

package process

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/sys/unix"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const readonlyPrimitivePath = "/usr/bin/bwrap"
const readonlyBootstrapPath = "/.agentplugins-readonly-bootstrap"

type readonlyRuntimeRoot struct {
	visible string
	object  readonlyOriginalObject
	alias   *readonlyObjectFacts
	target  string
}

type readonlySystemInputs struct {
	primitive    readonlyOriginalObject
	runtime      [4]readonlyRuntimeRoot
	self         *os.File
	selfFacts    readonlyObjectFacts
	selfSHA      string
	primitiveSHA string
	owned        []*os.File
	closeErr     error
}

func readonlyRuntimePaths() [4]string { return [4]string{"/usr", "/bin", "/lib", "/lib64"} }

func openReadonlySystem(ctx context.Context) (result *readonlySystemInputs, resultErr error) {
	result = &readonlySystemInputs{}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.close())
			result = nil
		}
	}()
	primitive, err := result.open(ctx, readonlyPrimitivePath, false)
	if err != nil {
		return result, err
	}
	if primitive.facts.Mode&0111 == 0 {
		return result, errors.New("readonly primitive is not executable")
	}
	if err := readonlyNoFileCapability(primitive.file); err != nil {
		return result, err
	}
	result.primitive = primitive
	result.primitiveSHA, err = hashReadonlyHeld(ctx, primitive.file, primitive.facts)
	if err != nil {
		return result, err
	}
	for i, path := range readonlyRuntimePaths() {
		root, err := result.runtimeRoot(ctx, path)
		if err != nil {
			return result, err
		}
		result.runtime[i] = root
	}
	if err := result.openSelf(ctx); err != nil {
		return result, err
	}
	return result, ctx.Err()
}

func (system *readonlySystemInputs) open(ctx context.Context, path string, directory bool) (readonlyOriginalObject, error) {
	if err := ctx.Err(); err != nil {
		return readonlyOriginalObject{}, err
	}
	flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW | unix.O_NONBLOCK
	if directory {
		flags |= unix.O_DIRECTORY
	}
	fd, err := unix.Open(path, flags, 0)
	if err != nil {
		return readonlyOriginalObject{}, err
	}
	file := os.NewFile(uintptr(fd), "readonly-system-input")
	system.owned = append(system.owned, file)
	info, err := file.Stat()
	if err != nil {
		return readonlyOriginalObject{}, err
	}
	return admitReadonlyOriginal(ctx, ports.ReadonlyOriginal{Path: path, File: file, Info: info}, directory, 0, 0)
}

func (system *readonlySystemInputs) runtimeRoot(ctx context.Context, visible string) (readonlyRuntimeRoot, error) {
	if err := ctx.Err(); err != nil {
		return readonlyRuntimeRoot{}, err
	}
	before, err := os.Lstat(visible)
	if err != nil {
		return readonlyRuntimeRoot{}, err
	}
	canonical, err := filepath.EvalSymlinks(visible)
	if err != nil || !readonlyWithin(canonical, "/usr") {
		return readonlyRuntimeRoot{}, errors.Join(err, errors.New("readonly runtime root is outside /usr"))
	}
	object, err := system.open(ctx, canonical, true)
	if err != nil {
		return readonlyRuntimeRoot{}, err
	}
	root := readonlyRuntimeRoot{visible: visible, object: object}
	if canonical != visible {
		facts, err := readonlyFacts(before)
		if err != nil || facts.Mode&unix.S_IFMT != unix.S_IFLNK || facts.UID != 0 || facts.GID != 0 || facts.Links != 1 {
			return readonlyRuntimeRoot{}, errors.Join(err, errors.New("readonly runtime alias authority differs"))
		}
		target, err := readReadonlyLink(visible)
		if err != nil || filepath.Clean(filepath.Join(filepath.Dir(visible), target)) != canonical {
			return readonlyRuntimeRoot{}, errors.Join(err, errors.New("readonly runtime alias target differs"))
		}
		root.alias, root.target = &facts, target
	} else if before.Mode()&os.ModeSymlink != 0 {
		return readonlyRuntimeRoot{}, errors.New("readonly runtime root alias is not canonical")
	}
	return root, root.revalidate(ctx)
}

func readReadonlyLink(path string) (string, error) {
	buffer := make([]byte, 4097)
	n, err := unix.Readlink(path, buffer)
	if err != nil {
		return "", err
	}
	if n <= 0 || n > 4096 || !utf8.Valid(buffer[:n]) || strings.ContainsAny(string(buffer[:n]), "\x00\r\n") {
		return "", errors.New("readonly runtime alias target exceeds its bound")
	}
	return string(buffer[:n]), nil
}

func (root readonlyRuntimeRoot) revalidate(ctx context.Context) error {
	if err := root.object.revalidate(ctx); err != nil {
		return err
	}
	if root.alias == nil {
		return nil
	}
	info, err := os.Lstat(root.visible)
	if err != nil {
		return err
	}
	facts, err := readonlyFacts(info)
	if err != nil || facts != *root.alias {
		return errors.Join(err, errors.New("readonly runtime alias changed"))
	}
	target, err := readReadonlyLink(root.visible)
	if err != nil || target != root.target {
		return errors.Join(err, errors.New("readonly runtime alias changed"))
	}
	return ctx.Err()
}

func (system *readonlySystemInputs) openSelf(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Capture the running inode through the kernel link. Unlinked or unsafe
	// metadata still refuses; this never adopts a caller-selected executable.
	file, err := os.Open("/proc/self/exe")
	if err != nil {
		return err
	}
	system.owned = append(system.owned, file)
	info, err := file.Stat()
	if err != nil {
		return err
	}
	facts, err := readonlyFacts(info)
	if err != nil {
		return err
	}
	uid, gid, err := readonlyCallerIDs()
	if err != nil {
		return err
	}
	if facts.UID == 0 && facts.GID == 0 {
		uid, gid = 0, 0
	}
	if err := admitReadonlyFacts(facts, false, uid, gid); err != nil {
		return err
	}
	if facts.Mode&0111 == 0 {
		return errors.New("readonly bootstrap is not executable")
	}
	if err := readonlyNoFileCapability(file); err != nil {
		return err
	}
	digest, err := hashReadonlyHeld(ctx, file, facts)
	if err != nil {
		return err
	}
	system.self, system.selfFacts, system.selfSHA = file, facts, digest
	return ctx.Err()
}

func (system *readonlySystemInputs) close() error {
	if system == nil {
		return nil
	}
	for _, file := range system.owned {
		system.closeErr = errors.Join(system.closeErr, file.Close())
	}
	system.owned = nil
	return system.closeErr
}

func hashReadonlyHeld(ctx context.Context, file *os.File, before readonlyObjectFacts) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if before.Mode&unix.S_IFMT != unix.S_IFREG || before.Links != 1 || before.Size <= 0 || before.Size > 512<<20 {
		return "", errors.New("readonly executable byte bound differs")
	}
	if err := requireReadonlyHeld(file, before); err != nil {
		return "", err
	}
	hash := sha256.New()
	buffer := make([]byte, 32<<10)
	for offset := int64(0); offset < before.Size; {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		count := min(int64(len(buffer)), before.Size-offset)
		n, err := file.ReadAt(buffer[:count], offset)
		if int64(n) != count || err != nil && !errors.Is(err, io.EOF) {
			return "", errors.Join(err, errors.New("readonly executable bytes changed"))
		}
		_, _ = hash.Write(buffer[:n])
		offset += int64(n)
	}
	n, err := file.ReadAt(buffer[:1], before.Size)
	if n != 0 || !errors.Is(err, io.EOF) {
		return "", errors.Join(err, errors.New("readonly executable grew during read"))
	}
	if err := errors.Join(requireReadonlyHeld(file, before), ctx.Err()); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
