//go:build darwin && arm64

package packageview

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const ReadProfile = "packageview-local-darwin-quiescent-apfs-v2"

// Fstatat observes exactly one entry in a held parent without opening its data.
// Sys uses syscall.Stat_t so identity/epoch comparisons also accept File.Stat.
type darwinInfo struct {
	name string
	st   syscall.Stat_t
}

func darwinLstat(parent *os.File, name string) (os.FileInfo, error) {
	defer runtime.KeepAlive(parent)
	var st unix.Stat_t
	if e := unix.Fstatat(int(parent.Fd()), name, &st, unix.AT_SYMLINK_NOFOLLOW); e != nil {
		return nil, e
	}
	return &darwinInfo{name: name, st: syscall.Stat_t{
		Dev: st.Dev, Ino: st.Ino, Mode: st.Mode, Nlink: st.Nlink, Size: st.Size, Gen: st.Gen,
		Mtimespec: syscall.Timespec{Sec: st.Mtim.Sec, Nsec: st.Mtim.Nsec},
		Ctimespec: syscall.Timespec{Sec: st.Ctim.Sec, Nsec: st.Ctim.Nsec},
	}}, nil
}
func (i *darwinInfo) Name() string       { return i.name }
func (i *darwinInfo) Size() int64        { return i.st.Size }
func (i *darwinInfo) ModTime() time.Time { return time.Unix(i.st.Mtimespec.Sec, i.st.Mtimespec.Nsec) }
func (i *darwinInfo) IsDir() bool        { return i.Mode().IsDir() }
func (i *darwinInfo) Sys() any           { return &i.st }
func (i *darwinInfo) Mode() os.FileMode {
	m := os.FileMode(i.st.Mode & 0777)
	switch i.st.Mode & unix.S_IFMT {
	case unix.S_IFDIR:
		m |= os.ModeDir
	case unix.S_IFLNK:
		m |= os.ModeSymlink
	case unix.S_IFIFO:
		m |= os.ModeNamedPipe
	case unix.S_IFSOCK:
		m |= os.ModeSocket
	case unix.S_IFCHR:
		m |= os.ModeDevice | os.ModeCharDevice
	case unix.S_IFBLK:
		m |= os.ModeDevice
	case unix.S_IFREG:
	default:
		m |= os.ModeIrregular
	}
	if i.st.Mode&unix.S_ISUID != 0 {
		m |= os.ModeSetuid
	}
	if i.st.Mode&unix.S_ISGID != 0 {
		m |= os.ModeSetgid
	}
	if i.st.Mode&unix.S_ISVTX != 0 {
		m |= os.ModeSticky
	}
	return m
}
func sameIdentity(a, b os.FileInfo) bool {
	if a == nil || b == nil {
		return false
	}
	x, ok := a.Sys().(*syscall.Stat_t)
	y, ok2 := b.Sys().(*syscall.Stat_t)
	return ok && ok2 && x.Dev == y.Dev && x.Ino == y.Ino
}

func scratchIdentityCheck(source *source, tmp string) error {
	// APFS can preserve spelling while comparing names case-insensitively.
	// Walk scratch ancestors by identity so alternate case and Unicode spellings
	// cannot place owned scratch inside the held source root.

	rootInfo, e := source.anchor.Stat()
	if e != nil {
		return fail("root_unreadable")
	}
	for ancestor := tmp; ; ancestor = filepath.Dir(ancestor) {
		info, e := os.Stat(ancestor)
		if e != nil {
			return fail("scratch_unavailable")
		}
		if os.SameFile(rootInfo, info) {
			return fail("scratch_overlaps_source")
		}
		if filepath.Dir(ancestor) == ancestor {
			break
		}
	}
	return nil
}
