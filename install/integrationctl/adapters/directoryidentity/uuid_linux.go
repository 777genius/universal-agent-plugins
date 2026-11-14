//go:build linux && (amd64 || arm64)

package directoryidentity

import (
	"encoding/binary"
	"errors"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Linux v6.8 ext4 UAPI: _IOR('f', 44, struct fsuuid), whose fixed header is
// two u32 fields followed by the caller-sized UUID buffer. This read-only
// request returns the same on-disk UUID as the newer generic request.
const linuxExt4GetFSUUID = 0x8008662c

func linuxFilesystemUUID(fd int) (string, error) {
	var raw [17]byte
	// #nosec G103 -- synchronous read-only ioctl into owned 17-byte storage; no pointer retention; KeepAlive below.
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), linuxGetFSUUID, uintptr(unsafe.Pointer(&raw)))
	runtime.KeepAlive(raw)
	if errno == unix.ENOTTY {
		return linuxExt4UUID(fd)
	}
	if errno != 0 {
		return "", linuxUUIDError(errno)
	}
	return decodeLinuxUUID(raw[:])
}

func linuxExt4UUID(fd int) (string, error) {
	var fs unix.Statfs_t
	if err := unix.Fstatfs(fd, &fs); err != nil {
		return "", err
	}
	// Never send a filesystem-specific request to another filesystem driver.
	if fs.Type != unix.EXT4_SUPER_MAGIC {
		return "", linuxUUIDError(unix.ENOTTY)
	}
	var raw [24]byte
	binary.NativeEndian.PutUint32(raw[:4], 16)
	// #nosec G103 -- synchronous read-only ext4 ioctl into owned 8-byte header plus 16 UUID bytes; no pointer retention; KeepAlive below.
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), linuxExt4GetFSUUID, uintptr(unsafe.Pointer(&raw)))
	runtime.KeepAlive(raw)
	if errno != 0 {
		return "", linuxUUIDError(errno)
	}
	return decodeLinuxExt4UUID(raw[:])
}

func decodeLinuxExt4UUID(raw []byte) (string, error) {
	if len(raw) != 24 || binary.NativeEndian.Uint32(raw[:4]) != 16 || binary.NativeEndian.Uint32(raw[4:8]) != 0 {
		return "", errors.New("malformed ext4 UUID response")
	}
	var generic [17]byte
	generic[0] = 16
	copy(generic[1:], raw[8:])
	return decodeLinuxUUID(generic[:])
}
