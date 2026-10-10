//go:build darwin && arm64

package nativeconfig

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

type plainImage struct {
	stat       unix.Stat_t
	fs         unix.Fsid
	body       []byte
	provenance []byte
	present    bool
	exists     bool
}

func (image plainImage) snapshot() FileSnapshot {
	mode := os.FileMode(0600)
	if image.exists {
		mode = os.FileMode(image.stat.Mode & 0777)
	}
	return FileSnapshot{Body: bytes.Clone(image.body), Mode: mode, Exists: image.exists}
}

func plainSameStat(a, b unix.Stat_t, epoch bool) bool {
	return a.Dev == b.Dev && a.Ino == b.Ino && a.Mode == b.Mode && a.Uid == b.Uid &&
		a.Gid == b.Gid && a.Flags == b.Flags && a.Nlink == b.Nlink &&
		(!epoch || (a.Size == b.Size && a.Mtim == b.Mtim && a.Ctim == b.Ctim))
}

func plainSame(a, b plainImage, epoch bool) bool {
	return a.exists == b.exists && (!a.exists || (plainSameStat(a.stat, b.stat, epoch) &&
		a.fs == b.fs && plainMetadataEqual(a, b) && bytes.Equal(a.body, b.body)))
}

// APFS counts directory children in Nlink. Only a bracketed own namespace
// operation may adjust that count and rebind epochs; file comparisons stay strict.
func plainSameParent(a, b plainImage, epoch bool, childDelta int) bool {
	if epoch {
		return childDelta == 0 && plainSame(a, b, true)
	}
	if a.stat.Mode&unix.S_IFMT != unix.S_IFDIR || childDelta < -1 || childDelta > 1 {
		return false
	}
	links := int64(a.stat.Nlink) + int64(childDelta)
	if links < 0 || links > int64(^uint16(0)) || int64(b.stat.Nlink) != links {
		return false
	}
	a.stat.Nlink = b.stat.Nlink
	return plainSame(a, b, false)
}

func plainMetadataEqual(a, b plainImage) bool {
	return a.present == b.present && bytes.Equal(a.provenance, b.provenance)
}

func plainAdmission(st unix.Stat_t, directory bool) error {
	kind, access := uint16(unix.S_IFREG), uint16(0200)
	if directory {
		kind, access = unix.S_IFDIR, 0300
	}
	if st.Mode&unix.S_IFMT != kind || st.Mode&07000 != 0 || st.Mode&access != access ||
		int64(st.Uid) != int64(os.Geteuid()) || st.Flags != 0 || (!directory && st.Nlink != 1) {
		return errors.New("plain exact file identity/security is not admitted")
	}
	return nil
}

// Each image reads security twice and data from this same descriptor, inside
// Fstat brackets. Only documented accessible channels on native APFS qualify.
func plainCapture(file *os.File, directory bool) (plainImage, error) {
	defer runtime.KeepAlive(file)
	image := plainImage{exists: true}
	fd := int(file.Fd())
	if err := unix.Fstat(fd, &image.stat); err != nil {
		return image, err
	}
	if err := plainAdmission(image.stat, directory); err != nil {
		return image, err
	}
	fs, err := plainFilesystem(fd)
	if err != nil {
		return image, err
	}
	image.fs = fs
	value, present, err := plainSecurity(file)
	if err != nil {
		return image, err
	}
	image.provenance, image.present = value, present
	if !directory {
		if _, err = file.Seek(0, io.SeekStart); err != nil {
			return image, err
		}
		if image.body, err = io.ReadAll(file); err != nil {
			return image, err
		}
	}
	value, present, err = plainSecurity(file)
	if err != nil {
		return image, err
	}
	fs, err = plainFilesystem(fd)
	if err != nil {
		return image, err
	}
	var after unix.Stat_t
	if err = unix.Fstat(fd, &after); err != nil {
		return image, err
	}
	if image.fs != fs || !plainSameStat(image.stat, after, true) || image.present != present || !bytes.Equal(image.provenance, value) ||
		(!directory && int64(len(image.body)) != image.stat.Size) {
		return image, ErrConcurrentChange
	}
	return image, nil
}

func plainFilesystem(fd int) (unix.Fsid, error) {
	var fs unix.Statfs_t
	if err := unix.Fstatfs(fd, &fs); err != nil {
		return fs.Fsid, err
	}
	if unix.ByteSliceToString(fs.Fstypename[:]) != "apfs" || fs.Flags&unix.MNT_LOCAL == 0 || fs.Flags&unix.MNT_RDONLY != 0 {
		return fs.Fsid, errors.New("plain exact requires writable native local APFS")
	}
	return fs.Fsid, nil
}

func plainSecurity(file *os.File) ([]byte, bool, error) {
	if err := plainNullACL(file); err != nil {
		return nil, false, err
	}
	return plainProvenance(file)
}

func plainNullACL(file *os.File) error {
	defer runtime.KeepAlive(file)
	// 24-byte attrlist, no returned-attrs/invalid-field packing substitution.
	attrs := unix.Attrlist{Bitmapcount: 5, Commonattr: 0x00400000 | 0x00800000 | 0x01000000}
	var packet [44]byte
	// #nosec G103 -- synchronous libSystem call uses owned 24-byte request and 44-byte packet, explicit capacity, no pointer retention; uintptrescapes and KeepAlive protect allocations.
	_, _, errno := plainSystemCall6(plainGetattrAddress, file.Fd(), uintptr(unsafe.Pointer(&attrs)), uintptr(unsafe.Pointer(&packet[0])), uintptr(len(packet)), 4, 0)
	runtime.KeepAlive(attrs)
	runtime.KeepAlive(packet)
	if errno != 0 {
		return errno
	}
	// The exact encoding 40 also excludes every negative signed offset.
	if binary.LittleEndian.Uint32(packet[0:4]) != 44 || binary.LittleEndian.Uint32(packet[4:8]) != 40 ||
		binary.LittleEndian.Uint32(packet[8:12]) != 0 || !bytes.Equal(packet[12:], make([]byte, 32)) {
		return errors.New("plain exact requires supported null ACL and zero security GUIDs")
	}
	return nil
}

func plainProvenance(file *os.File) ([]byte, bool, error) {
	defer runtime.KeepAlive(file)
	var names [21]byte
	// #nosec G103 -- synchronous flistxattr fills owned fixed 21-byte names, no retention; escape and KeepAlive below.
	n, _, errno := plainSystemCall6(plainListAddress, file.Fd(), uintptr(unsafe.Pointer(&names[0])), uintptr(len(names)), 0x20, 0, 0)
	runtime.KeepAlive(names)
	if errno != 0 {
		return nil, false, errno
	}
	if n == 0 {
		return nil, false, nil
	}
	if n != uintptr(len(names)) || string(names[:]) != "com.apple.provenance\x00" {
		return nil, false, errors.New("plain exact refuses other or malformed xattrs")
	}
	var value [256]byte
	name := []byte("com.apple.provenance\x00")
	// #nosec G103 -- synchronous fgetxattr reads only owned terminated name and fixed 256-byte value, position zero, no retention; escape and KeepAlive below.
	n, _, errno = plainSystemCall6(plainGetAddress, file.Fd(), uintptr(unsafe.Pointer(&name[0])), uintptr(unsafe.Pointer(&value[0])), uintptr(len(value)), 0, 0x20)
	runtime.KeepAlive(name)
	runtime.KeepAlive(value)
	if errno != 0 {
		return nil, false, errno
	}
	if n > uintptr(len(value)) {
		return nil, false, errors.New("plain provenance exceeds bound")
	}
	return bytes.Clone(value[:int(n)]), true, nil
}

// Use the existing pinned x/sys libSystem ABI, with no P1 imports or authority.
//
//go:linkname plainSystemCall6 syscall.syscall6
//go:uintptrescapes
func plainSystemCall6(fn, a1, a2, a3, a4, a5, a6 uintptr) (r1, r2 uintptr, err unix.Errno)

var plainGetattrAddress, plainListAddress, plainGetAddress uintptr

//go:cgo_import_dynamic plain_libc_fgetattrlist fgetattrlist "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic plain_libc_flistxattr flistxattr "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic plain_libc_fgetxattr fgetxattr "/usr/lib/libSystem.B.dylib"
