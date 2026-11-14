//go:build darwin && arm64

package packageview

import (
	"errors"
	"os"
	"runtime"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// Darwin accepts local APFS, including ordinary writable volumes, under a
// quiescent-source contract: callers must stop writers for the entire lease.
// Held-parent traversal constrains name resolution; post-open checks reject
// observed replacements before data reads. Without an O_PATH upgrade seam this
// is not protection against hostile concurrent type swaps at open, privileged
// remounts, or coordinated writers. No snapshot or atomic revision is promised.
type source struct {
	anchor     *os.File
	legacyInfo os.FileInfo
	dev        int32
	fsid       unix.Fsid
}
type pinned struct {
	file   *os.File // owned parent directory, NOT a data handle to the entry
	info   os.FileInfo
	name   string // one component in the held parent
	source *source
}

// Local APFS is mandatory for every opened directory and data descriptor.
func darwinFS(fd int) (unix.Statfs_t, error) {
	var fs unix.Statfs_t
	if e := unix.Fstatfs(fd, &fs); e != nil {
		return fs, e
	}
	if unix.ByteSliceToString(fs.Fstypename[:]) != "apfs" || fs.Flags&unix.MNT_LOCAL == 0 {
		return fs, fail("filesystem_unavailable")
	}
	return fs, nil
}
func openSource(name string, generated GeneratedStaging) (_ *source, err error) {
	if n := strings.TrimRight(name, "/"); n != "" {
		name = n
	}
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
	// O_DIRECTORY constrains type in the kernel before open; O_NOFOLLOW applies
	// to the final component even if the caller supplied a trailing slash.
	fd, e := unix.Open(name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if e != nil {
		return nil, fail("root_unreadable")
	}
	s := &source{anchor: os.NewFile(uintptr(fd), "source-root")}
	defer func() {
		if err != nil {
			_ = s.close()
		}
	}()
	opened, e := s.anchor.Stat()
	if e != nil || !os.SameFile(before, opened) {
		return nil, fail("source_changed")
	}
	// A supplied proof must match the exact object just opened by path, or the
	// call fails closed: a stale/mismatched proof is never silently downgraded
	// to the ordinary strict profile, since that would mask a swapped root.
	if generated.present() {
		if !generated.matches(opened) {
			return nil, fail("generated_staging_mismatch")
		}
	}
	fs, e := darwinFS(fd)
	if e != nil {
		return nil, e
	}
	s.fsid = fs.Fsid
	st, ok := opened.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fail("platform_unavailable")
	}
	s.dev = st.Dev
	after, e := os.Lstat(name)
	if e != nil || !after.IsDir() || !os.SameFile(opened, after) {
		return nil, fail("source_changed")
	}
	return s, nil
}
func (s *source) close() error {
	if s.anchor == nil {
		return nil
	}
	e := s.anchor.Close()
	s.anchor = nil
	return e
}
func (s *source) pin(rel string, nofollow bool) (result *pinned, err error) {
	if rel == "" || strings.HasPrefix(rel, "/") || strings.IndexByte(rel, 0) >= 0 || len(rel) > 4096 {
		return nil, syscall.EXDEV
	}
	fd, e := unix.Openat(int(s.anchor.Fd()), ".", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if e != nil {
		return nil, e
	}
	parents := []*os.File{os.NewFile(uintptr(fd), "source-parent")}
	defer func() {
		var closeErr error
		for _, f := range parents {
			closeErr = errors.Join(closeErr, f.Close())
		}
		if err == nil && closeErr != nil {
			if result != nil {
				_ = result.file.Close()
				result = nil
			}
			err = fail("close_failed")
		}
	}()
	todo := strings.Split(rel, "/")
	links, steps := 0, 0
	for len(todo) > 0 {
		steps++
		if steps > 8192 {
			return nil, syscall.ELOOP
		}
		n := todo[0]
		todo = todo[1:]
		if n == "" || n == "." {
			if len(todo) > 0 {
				continue
			}
			n = "."
		}
		if n == ".." {
			if len(parents) == 1 {
				return nil, syscall.EXDEV
			}
			if e := parents[len(parents)-1].Close(); e != nil {
				return nil, e
			}
			parents = parents[:len(parents)-1]
			if len(todo) > 0 {
				continue
			}
			n = "."
		}
		dir := parents[len(parents)-1]
		fs, e := darwinFS(int(dir.Fd()))
		if e != nil || fs.Fsid != s.fsid {
			return nil, syscall.EXDEV
		}
		info, e := darwinLstat(dir, n)
		if e != nil {
			return nil, e
		}
		st := info.Sys().(*syscall.Stat_t)
		if st.Dev != s.dev {
			return nil, syscall.EXDEV
		}
		p := &pinned{file: dir, info: info, name: n, source: s}
		if info.Mode()&os.ModeSymlink != 0 && (!nofollow || len(todo) > 0) {
			links++
			if links > 40 {
				return nil, syscall.ELOOP
			}
			target, e := p.link(4096)
			if e != nil {
				return nil, e
			}
			if target == "" || strings.HasPrefix(target, "/") {
				return nil, syscall.EXDEV
			}
			todo = append(strings.Split(target, "/"), todo...)
			continue
		}
		if len(todo) == 0 {
			// Transfer only this parent to the pin; the traversal owns the rest.
			parents = parents[:len(parents)-1]
			return p, nil
		}
		if !info.IsDir() {
			return nil, syscall.ENOTDIR
		}
		if n != "." {
			next, e := p.reopen(true)
			if e != nil {
				return nil, e
			}
			parents = append(parents, next)
		}
	}
	return nil, syscall.ENOENT
}
func (p *pinned) link(limit int64) (string, error) {
	defer runtime.KeepAlive(p)
	if p.info.Mode()&os.ModeSymlink == 0 {
		return "", syscall.EINVAL
	}
	if limit < 0 {
		return "", fail("byte_limit")
	}
	b := make([]byte, int(min(limit, 4096)+1))
	n, e := unix.Readlinkat(int(p.file.Fd()), p.name, b)
	if e != nil {
		return "", e
	}
	if n == len(b) || int64(n) > limit {
		return "", fail("byte_limit")
	}
	after, e := darwinLstat(p.file, p.name)
	if e != nil || !same(p.info, after) {
		return "", fail("source_changed")
	}
	return string(b[:n]), nil
}
func (p *pinned) reopen(directory bool) (*os.File, error) {
	if (directory && !p.info.IsDir()) || (!directory && (!p.info.Mode().IsRegular() || multipleLinks(p.info))) {
		return nil, fail("wrong_kind")
	}
	fs, e := darwinFS(int(p.file.Fd()))
	if e != nil || fs.Fsid != p.source.fsid {
		return nil, fail("filesystem_unavailable")
	}
	flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
	if directory {
		flags |= unix.O_DIRECTORY
	}
	// The caller keeps the source quiescent; verify the opened object before reads.
	fd, e := unix.Openat(int(p.file.Fd()), p.name, flags, 0)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(fd), "source-data")
	openedFS, e := darwinFS(fd)
	if e != nil || openedFS.Fsid != p.source.fsid {
		f.Close()
		return nil, fail("filesystem_unavailable")
	}
	info, e := f.Stat()
	if e != nil || (directory && !info.IsDir()) || (!directory && (!info.Mode().IsRegular() || multipleLinks(info))) || !same(p.info, info) {
		f.Close()
		return nil, fail("source_changed")
	}
	if !directory {
		if p.source.legacyInfo != nil && sameIdentity(info, p.source.legacyInfo) {
			f.Close()
			return nil, fail("source_changed")
		}
		legacy, err := p.source.pin("plugin/plugin.yaml", false)
		if err == nil {
			aliased := sameIdentity(info, legacy.info)
			ce := legacy.file.Close()
			if aliased || ce != nil {
				f.Close()
				return nil, fail("source_changed")
			}
		} else if stateOf(err) != Absent && stateOf(err) != WrongKind {
			f.Close()
			return nil, fail("source_changed")
		}
	}
	return f, nil
}
func stateOf(e error) State {
	if errors.Is(e, syscall.ENOENT) {
		return Absent
	}
	if errors.Is(e, syscall.ENOTDIR) {
		return WrongKind
	}
	if errors.Is(e, syscall.EXDEV) || errors.Is(e, syscall.ELOOP) {
		return Blocked
	}
	return Unreadable
}
func same(a, b os.FileInfo) bool {
	if a == nil || b == nil || !sameIdentity(a, b) || a.Mode() != b.Mode() || a.Size() != b.Size() || !a.ModTime().Equal(b.ModTime()) {
		return false
	}
	x, ok := a.Sys().(*syscall.Stat_t)
	y, ok2 := b.Sys().(*syscall.Stat_t)
	return ok && ok2 && x.Ctimespec == y.Ctimespec && x.Nlink == y.Nlink && x.Gen == y.Gen
}
func multipleLinks(i os.FileInfo) bool {
	s, ok := i.Sys().(*syscall.Stat_t)
	return !ok || s.Nlink != 1
}
