//go:build darwin && arm64

package directoryidentity

import (
	"bytes"
	"errors"
	"os"
	"runtime"
	"strconv"
	"unsafe"

	"golang.org/x/sys/unix"
)

func platformScheme() string { return "darwin-voluuid-inode-v1" }

func openDirectory(parent *os.File, name string) (*os.File, error) {
	flags := unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
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

func directoryFacts(f *os.File) (string, string, error) {
	defer runtime.KeepAlive(f)
	var stat unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &stat); err != nil {
		return "", "", err
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFDIR {
		return "", "", unix.ENOTDIR
	}
	var fs unix.Statfs_t
	if err := unix.Fstatfs(int(f.Fd()), &fs); err != nil {
		return "", "", err
	}
	if unix.ByteSliceToString(fs.Fstypename[:]) != "apfs" || fs.Flags&unix.MNT_LOCAL == 0 {
		return "", "", ErrUnsupported
	}
	attrs := unix.Attrlist{Bitmapcount: 5, Commonattr: darwinReturnedAttrs, Volattr: darwinVolumeInfo | darwinVolumeUUID}
	var b [40]byte
	// XNU fgetattrlist operates on this directory fd. The volume query uses the
	// volume-root vnode, so its result must never replace the separate fstat inode.
	// Pointer audit: the target Attrlist is 24 bytes and the output exactly
	// 40 bytes. The synchronous libc call retains neither pointer; escapes
	// plus KeepAlive protect both allocations. Packed lengths are decoded below.
	// #nosec G103 -- fgetattrlist bitmapcount 5/VOL_INFO|UUID synchronously uses owned 24-byte attrs and 40-byte b (explicit capacity); no pointer retention; both escape and KeepAlive below.
	_, _, errno := libSystemCall6(fgetattrlistAddress, f.Fd(), uintptr(unsafe.Pointer(&attrs)), uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), 0, 0)
	runtime.KeepAlive(attrs)
	runtime.KeepAlive(b)
	if errno != 0 {
		return "", "", errno
	}
	volume, err := decodeDarwinUUID(b[:])
	return volume, strconv.FormatUint(stat.Ino, 10), err
}

// The pinned x/sys libSystem convention supplies the missing held-fd wrapper;
// no deprecated Darwin raw-syscall or pathname fallback is used. uintptr escapes
// keep the fixed request/response storage live across the synchronous libc call.
//
//go:linkname libSystemCall6 syscall.syscall6
//go:uintptrescapes
func libSystemCall6(fn, a1, a2, a3, a4, a5, a6 uintptr) (r1, r2 uintptr, err unix.Errno)

// Darwin arm64 fcntl is variadic: syscall.syscall copies argument 3 to the
// variadic stack slot, while syscall6 copies argument 4. Match pinned x/sys's
// fcntl convention and keep the fixed-argument fgetattrlist seam separate.
//
//go:linkname libSystemCall3 syscall.syscall
//go:uintptrescapes
func libSystemCall3(fn, a1, a2, a3 uintptr) (r1, r2 uintptr, err unix.Errno)

var fgetattrlistAddress uintptr

//go:cgo_import_dynamic p1_libc_fgetattrlist fgetattrlist "/usr/lib/libSystem.B.dylib"

// F_GETPATH on the held fd supplies the native spelling, including APFS case /
// Unicode behavior. Symlinks were already resolved once by canonicalize; this
// refuses unresolved aliases without discovering a home or using a pathname fd.
func verifyCanonicalName(f *os.File, expected string) error {
	defer runtime.KeepAlive(f)
	var b [1024]byte // Darwin MAXPATHLEN for F_GETPATH.
	// #nosec G103 -- fcntl F_GETPATH synchronously fills owned MAXPATHLEN (1024-byte) b through the three-argument variadic ABI; no retention; escape and KeepAlive below.
	_, _, errno := libSystemCall3(fcntlAddress, f.Fd(), unix.F_GETPATH, uintptr(unsafe.Pointer(&b[0])))
	runtime.KeepAlive(b)
	if errno != 0 {
		return errno
	}
	n := bytes.IndexByte(b[:], 0)
	if n < 0 {
		return unix.ENAMETOOLONG
	}
	if string(b[:n]) != expected {
		return errors.New("unresolved directory path alias")
	}
	return nil
}

var fcntlAddress uintptr

//go:cgo_import_dynamic p1_libc_fcntl fcntl "/usr/lib/libSystem.B.dylib"
