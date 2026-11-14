//go:build linux

package process

import (
	"context"
	"errors"
	"io"
	"math"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

type readonlyKernelInputs struct {
	namespaces               [7]uint64
	overflowUID, overflowGID uint32
	groups                   []uint32
	devices                  [6]readonlyPolicyDevice
	files                    [6]*os.File
	root                     readonlyOriginalObject
}

func readonlyNamespaceNames() [7]string {
	return [7]string{"user", "pid", "net", "ipc", "uts", "mnt", "time"}
}
func readonlyDeviceNames() [6]string {
	return [6]string{"null", "zero", "full", "random", "urandom", "tty"}
}

func captureReadonlyGroups() ([]uint32, error) {
	groups, err := os.Getgroups()
	if err != nil || len(groups) > 64 {
		return nil, errors.Join(err, errors.New("readonly original supplementary groups differ"))
	}
	result := make([]uint32, len(groups))
	for i, group := range groups {
		if group < 0 || uint64(group) > math.MaxUint32 {
			return nil, errors.New("readonly original group identity is outside its bound")
		}
		result[i] = uint32(group)
	}
	return result, nil
}

func captureReadonlyNamespaces(ctx context.Context) ([7]uint64, error) {
	var result [7]uint64
	for i, name := range readonlyNamespaceNames() {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		info, err := os.Stat("/proc/self/ns/" + name)
		if err != nil {
			return result, err
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || stat == nil || stat.Ino == 0 {
			return result, errors.New("readonly original namespace identity differs")
		}
		result[i] = stat.Ino
	}
	return result, ctx.Err()
}

func readonlyProcLimit(path string) int {
	switch path {
	case "/proc/self/status":
		return 65536
	case "/proc/self/mountinfo":
		return 262144
	case "/proc/self/uid_map", "/proc/self/gid_map":
		return 128
	case "/proc/self/setgroups":
		return 16
	case "/proc/sys/kernel/overflowuid", "/proc/sys/kernel/overflowgid":
		return 32
	default:
		return 0
	}
}

func readReadonlyProc(ctx context.Context, path string) (body []byte, resultErr error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	limit := readonlyProcLimit(path)
	if limit == 0 {
		return nil, errors.New("readonly proc read is outside its fixed fields")
	}
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), "readonly-proc")
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	var filesystem unix.Statfs_t
	if err := unix.Fstatfs(fd, &filesystem); err != nil || filesystem.Type != unix.PROC_SUPER_MAGIC {
		return nil, errors.Join(err, errors.New("readonly proc source is not the kernel filesystem"))
	}
	body, err = io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil || len(body) == 0 || len(body) > limit {
		return nil, errors.Join(err, errors.New("readonly proc field exceeds its bound"))
	}
	return body, ctx.Err()
}

func readonlyOverflow(ctx context.Context, path string) (uint32, error) {
	body, err := readReadonlyProc(ctx, path)
	if err != nil {
		return 0, err
	}
	value, err := strconv.ParseUint(strings.TrimSuffix(string(body), "\n"), 10, 32)
	if err != nil || value == 0 || string(body) != strconv.FormatUint(value, 10)+"\n" {
		return 0, errors.Join(err, errors.New("readonly overflow identity is not canonical"))
	}
	return uint32(value), nil
}

func captureReadonlyKernel(ctx context.Context, system *readonlySystemInputs) (result readonlyKernelInputs, resultErr error) {
	var err error
	result.namespaces, err = captureReadonlyNamespaces(ctx)
	if err != nil {
		return result, err
	}
	result.overflowUID, err = readonlyOverflow(ctx, "/proc/sys/kernel/overflowuid")
	if err != nil {
		return result, err
	}
	result.overflowGID, err = readonlyOverflow(ctx, "/proc/sys/kernel/overflowgid")
	if err != nil {
		return result, err
	}
	result.groups, err = captureReadonlyGroups()
	if err != nil {
		return result, err
	}
	root, err := system.kernelFile(nil, "/dev", true)
	if err != nil {
		return result, err
	}
	info, err := root.Stat()
	if err != nil {
		return result, err
	}
	facts, err := readonlyFacts(info)
	if err != nil {
		return result, err
	}
	if err := admitReadonlyFacts(facts, true, 0, 0); err != nil {
		return result, err
	}
	result.root = readonlyOriginalObject{path: "/dev", file: root, facts: facts, directory: true}
	for i, name := range readonlyDeviceNames() {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if err := result.captureDevice(system, i, name); err != nil {
			return result, err
		}
	}
	return result, result.revalidate(ctx)
}

func (system *readonlySystemInputs) kernelFile(parent *os.File, path string, directory bool) (*os.File, error) {
	flags := unix.O_PATH | unix.O_NOFOLLOW | unix.O_CLOEXEC
	if directory {
		flags |= unix.O_DIRECTORY
	}
	parentFD := unix.AT_FDCWD
	if parent != nil {
		parentFD = int(parent.Fd())
	}
	fd, err := unix.Openat(parentFD, path, flags, 0)
	runtime.KeepAlive(parent)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), "readonly-kernel-input")
	system.owned = append(system.owned, file)
	return file, nil
}

func (kernel *readonlyKernelInputs) captureDevice(system *readonlySystemInputs, index int, name string) error {
	if err := kernel.root.joinMetadata(); err != nil {
		return err
	}
	file, err := system.kernelFile(kernel.root.file, name, false)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	facts, err := readonlyFacts(info)
	if err != nil {
		return err
	}
	kernel.files[index] = file
	kernel.devices[index] = readonlyPolicyDevice{Facts: facts, Rdev: info.Sys().(*syscall.Stat_t).Rdev}
	return requireReadonlyNamed(kernel.root.file, name, facts)
}

func (kernel readonlyKernelInputs) revalidate(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := kernel.root.joinMetadata(); err != nil {
		return err
	}
	namespaces, err := captureReadonlyNamespaces(ctx)
	if err != nil || namespaces != kernel.namespaces {
		return errors.Join(err, errors.New("readonly parent namespaces changed"))
	}
	groups, err := captureReadonlyGroups()
	if err != nil || !reflect.DeepEqual(groups, kernel.groups) {
		return errors.Join(err, errors.New("readonly parent groups changed"))
	}
	for i, name := range readonlyDeviceNames() {
		if err := errors.Join(requireReadonlyHeld(kernel.files[i], kernel.devices[i].Facts),
			requireReadonlyNamed(kernel.root.file, name, kernel.devices[i].Facts)); err != nil {
			return err
		}
	}
	return ctx.Err()
}
