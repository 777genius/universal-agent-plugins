//go:build linux && (amd64 || arm64)

package directoryidentity

import (
	"errors"
	"os"
	"runtime"
	"strconv"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Linux v6.10 include/uapi/linux/fs.h: fsuuid2 is length + 16 bytes.
// This request is qualified for the amd64/arm64 _IOR ABI only.
const linuxGetFSUUID = 0x80111500

func platformScheme() string { return "linux-fsuuid-inode-v1" }

func openDirectory(parent *os.File, name string) (*os.File, error) {
	flags := unix.O_PATH | unix.O_DIRECTORY | unix.O_NOFOLLOW | unix.O_CLOEXEC
	var fd int
	var err error
	if parent == nil {
		fd, err = unix.Open(name, flags, 0)
	} else {
		fd, err = unix.Openat(int(parent.Fd()), name, flags, 0)
		runtime.KeepAlive(parent)
	}
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), "directory-identity-pin"), nil
}

func directoryFacts(pin *os.File) (string, string, error) {
	defer runtime.KeepAlive(pin)
	var before unix.Stat_t
	if err := unix.Fstat(int(pin.Fd()), &before); err != nil {
		return "", "", err
	}
	if before.Mode&unix.S_IFMT != unix.S_IFDIR {
		return "", "", unix.ENOTDIR
	}
	// O_PATH cannot ioctl. Upgrade only '.' relative to this live pin, never
	// procfs or the recorded pathname, and compare the separate fstat inode.
	fd, err := unix.Openat(int(pin.Fd()), ".", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return "", "", err
	}
	volume, inode, err := linuxReadableFacts(fd, before)
	return volume, inode, errors.Join(err, unix.Close(fd))
}

func linuxReadableFacts(fd int, before unix.Stat_t) (string, string, error) {
	var opened unix.Stat_t
	if err := unix.Fstat(fd, &opened); err != nil {
		return "", "", err
	}
	if opened.Dev != before.Dev || opened.Ino != before.Ino || opened.Mode&unix.S_IFMT != unix.S_IFDIR {
		return "", "", ErrChanged
	}
	// Pointer audit: one fixed 17-byte object; synchronous ioctl writes exactly
	// fsuuid2, no retained pointer. KeepAlive covers the response until return.
	var raw [17]byte
	// #nosec G103 -- FS_IOC_GETFSUUID 0x80111500 synchronously copies 17 bytes to owned raw on the live readable fd; no pointer retention; KeepAlive below.
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), linuxGetFSUUID, uintptr(unsafe.Pointer(&raw)))
	runtime.KeepAlive(raw)
	var err error
	if errno != 0 {
		err = errno
	}
	if err != nil {
		return "", "", linuxUUIDError(err)
	}
	volume, err := decodeLinuxUUID(raw[:])
	if err != nil {
		return "", "", err
	}
	if err := linuxCaseSensitive(fd); err != nil {
		return "", "", err
	}
	return volume, strconv.FormatUint(opened.Ino, 10), nil
}

func linuxUUIDError(err error) error {
	if errors.Is(err, unix.ENOTTY) {
		return errors.Join(ErrUnsupported, err)
	}
	return err
}

func verifyCanonicalName(_ *os.File, _ string) error { return nil }

// Linux has no held-fd canonical-name query without the forbidden procfs route.
// Refuse case-folding directory tuples rather than guess their canonical spelling.
// FS_CASEFOLD_FL is in the pinned v6.10 UAPI; GETFLAGS is a read-only query.
func linuxCaseSensitive(fd int) error {
	flags, err := unix.IoctlGetInt(fd, unix.FS_IOC_GETFLAGS)
	if err != nil {
		return err
	}
	return checkLinuxDirectoryFlags(flags)
}

func checkLinuxDirectoryFlags(flags int) error {
	const casefold = 0x40000000
	if flags&casefold != 0 {
		return errors.Join(ErrUnsupported, errors.New("case-folding directory tuple unqualified"))
	}
	return nil
}
