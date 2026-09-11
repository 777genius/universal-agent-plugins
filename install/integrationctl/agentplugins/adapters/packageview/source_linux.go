//go:build linux

package packageview

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

type source struct {
	root       *os.Root
	anchor     *os.File
	legacyInfo os.FileInfo
}
type pinned struct {
	file *os.File
	info os.FileInfo
}

// Linux's filesystem allowlist already permits ordinary writable local
// filesystems (see the switch below), so a generated-staging proof adds
// nothing here and is intentionally ignored.
func openSource(name string, _ GeneratedStaging) (_ *source, err error) {
	// A trailing slash must not hide a final symlink from Lstat.
	if trimmed := strings.TrimRight(name, "/"); trimmed != "" {
		name = trimmed
	}
	// Do not Clean the caller's path: a/../b must retain filesystem traversal order.
	before, e := os.Lstat(name)
	if e != nil {
		if os.IsNotExist(e) {
			return nil, fail("root_absent")
		}
		return nil, fail("root_unreadable")
	}
	if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return nil, fail("root_wrong_kind")
	}
	r, e := os.OpenRoot(name)
	if e != nil {
		return nil, fail("root_unreadable")
	}
	s := &source{root: r}
	defer func() {
		if err != nil {
			_ = s.close()
		}
	}()
	s.anchor, e = r.OpenFile(".", unix.O_PATH|unix.O_DIRECTORY, 0)
	if e != nil {
		return nil, fail("platform_unavailable")
	}
	opened, e := s.anchor.Stat()
	if e != nil || !os.SameFile(before, opened) {
		return nil, fail("source_changed")
	}
	after, e := os.Lstat(name)
	if e != nil || !after.IsDir() || !os.SameFile(opened, after) {
		return nil, fail("source_changed")
	}
	var fs unix.Statfs_t
	if unix.Fstatfs(int(s.anchor.Fd()), &fs) != nil {
		return nil, fail("platform_unavailable")
	}
	// Restrict data reads to ordinary local filesystem implementations allowed by
	// this profile. Evidence must identify the filesystem actually tested. In particular O_NONBLOCK cannot make proc/FUSE regular files
	// bounded or side-effect-free. Bind mounts below the root are rejected below.
	switch uint64(fs.Type) {
	case unix.EXT4_SUPER_MAGIC, unix.TMPFS_MAGIC, unix.OVERLAYFS_SUPER_MAGIC, unix.XFS_SUPER_MAGIC, unix.BTRFS_SUPER_MAGIC:
	default:
		return nil, fail("filesystem_unavailable")
	}
	p, e := s.pin(".", false)
	if e != nil {
		return nil, fail("platform_unavailable")
	}
	if e = p.file.Close(); e != nil {
		return nil, fail("close_failed")
	}
	return s, nil
}
func (s *source) close() error {
	var a, b error
	if s.anchor != nil {
		a = s.anchor.Close()
		s.anchor = nil
	}
	if s.root != nil {
		b = s.root.Close()
		s.root = nil
	}
	return errors.Join(a, b)
}
func (s *source) pin(rel string, nofollow bool) (*pinned, error) {
	flags := uint64(unix.O_PATH | unix.O_CLOEXEC)
	if nofollow {
		flags |= unix.O_NOFOLLOW
	}
	fd, e := unix.Openat2(int(s.anchor.Fd()), rel, &unix.OpenHow{Flags: flags, Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_MAGICLINKS | unix.RESOLVE_NO_XDEV})
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(fd), "captured-source-handle")
	info, e := f.Stat()
	if e != nil {
		_ = f.Close()
		return nil, e
	}
	return &pinned{f, info}, nil
}
func stateOf(err error) State {
	if errors.Is(err, syscall.ENOENT) {
		return Absent
	}
	if errors.Is(err, syscall.ENOTDIR) {
		return WrongKind
	}
	if errors.Is(err, syscall.EXDEV) || errors.Is(err, syscall.ELOOP) || errors.Is(err, syscall.ENOSYS) || errors.Is(err, syscall.EAGAIN) {
		return Blocked
	}
	return Unreadable
}
func same(a, b os.FileInfo) bool {
	if a == nil || b == nil || !os.SameFile(a, b) || a.Mode() != b.Mode() || a.Size() != b.Size() || !a.ModTime().Equal(b.ModTime()) {
		return false
	}
	x, ok := a.Sys().(*syscall.Stat_t)
	y, ok2 := b.Sys().(*syscall.Stat_t)
	return ok && ok2 && x.Ctim == y.Ctim && x.Nlink == y.Nlink
}
func multipleLinks(info os.FileInfo) bool {
	s, ok := info.Sys().(*syscall.Stat_t)
	return !ok || s.Nlink != 1
}
func (p *pinned) link(limit int64) (string, error) {
	// Linux PATH_MAX bounds symlink bytes. Allocate at most limit+1 beforehand.
	size := int(min(limit+1, 4097))
	b := make([]byte, size)
	n, e := unix.Readlinkat(int(p.file.Fd()), "", b)
	if e != nil {
		return "", e
	}
	if n == len(b) || int64(n) > limit {
		return "", fail("byte_limit")
	}
	return string(b[:n]), nil
}
func (p *pinned) reopen(directory bool) (*os.File, error) {
	if directory {
		if !p.info.IsDir() {
			return nil, fail("wrong_kind")
		}
	} else if !p.info.Mode().IsRegular() {
		return nil, fail("wrong_kind")
	}
	// The path is synthesized solely from an owned live O_PATH descriptor, never
	// source text. Procfs reopening selects that inode, even after source rename.
	// Check procfs itself, then use its pinned directory to avoid path replacement.
	proc, e := os.OpenRoot("/proc/self/fd")
	if e != nil {
		return nil, fail("platform_unavailable")
	}
	defer proc.Close()
	pf, e := proc.Open(".")
	if e != nil {
		return nil, fail("platform_unavailable")
	}
	var fs unix.Statfs_t
	e = unix.Fstatfs(int(pf.Fd()), &fs)
	if e != nil || uint64(fs.Type) != unix.PROC_SUPER_MAGIC {
		_ = pf.Close()
		return nil, fail("platform_unavailable")
	}
	flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NONBLOCK | unix.O_NOATIME
	if directory {
		flags |= unix.O_DIRECTORY
	}
	fd, e := unix.Openat(int(pf.Fd()), strconv.Itoa(int(p.file.Fd())), flags, 0)
	ce := pf.Close()
	if e != nil {
		return nil, e
	}
	if ce != nil {
		_ = unix.Close(fd)
		return nil, fail("close_failed")
	}
	f := os.NewFile(uintptr(fd), "captured-source-data")
	info, e := f.Stat()
	if e != nil || !same(p.info, info) {
		_ = f.Close()
		return nil, fail("source_changed")
	}
	return f, nil
}
