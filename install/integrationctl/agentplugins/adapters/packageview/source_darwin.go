//go:build darwin && arm64

package packageview

import (
	"errors"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// Darwin has no O_PATH -> data-descriptor upgrade in the Go/xsys seam.
// This profile therefore requires a READ-ONLY LOCAL APFS volume, UNLESS the
// caller supplied a matching GeneratedStaging proof (see openSource), in
// which case only this exact, caller-proven directory is exempt from the
// read-only requirement; it must still be local APFS. The held directory plus
// immutable directory entry pins an object before any data open. A privileged
// remount, hostile mount namespace or underlying block-device writer is
// outside the profile. Arbitrary writable APFS remains deliberately
// unavailable. In particular O_EVTONLY and /dev/fd are NOT used to upgrade
// file access.
type source struct {
	root       *os.Root
	anchor     *os.File
	legacyInfo os.FileInfo
	dev        int32
	fsid       unix.Fsid
	// trustedWritable is set only when openSource verified a GeneratedStaging
	// proof against this exact opened directory. It never widens to a
	// separately opened or path-resolved object.
	trustedWritable bool
}
type pinned struct {
	file   *os.File // owned parent directory, NOT a data handle to the entry
	info   os.FileInfo
	name   string // one component in the immutable parent
	source *source
}

// darwinFS enforces the local-APFS profile. trustedWritable, which only
// openSource can set (and only after a live-handle identity match), is the
// sole thing that exempts a source from the MNT_RDONLY requirement; MNT_LOCAL
// and the apfs filesystem type are always required, trusted or not.
func darwinFS(fd int, trustedWritable bool) (unix.Statfs_t, error) {
	var fs unix.Statfs_t
	if e := unix.Fstatfs(fd, &fs); e != nil {
		return fs, e
	}
	if unix.ByteSliceToString(fs.Fstypename[:]) != "apfs" || fs.Flags&unix.MNT_LOCAL == 0 {
		return fs, fail("filesystem_unavailable")
	}
	if !trustedWritable && fs.Flags&unix.MNT_RDONLY == 0 {
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
		s.trustedWritable = true
	}
	fs, e := darwinFS(fd, s.trustedWritable)
	if e != nil {
		return nil, e
	}
	s.fsid = fs.Fsid
	st, ok := opened.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fail("platform_unavailable")
	}
	s.dev = st.Dev
	// Go has no Root-from-descriptor constructor. Only this synthesized owned
	// DIRECTORY descriptor is passed through trusted devfs, never source text.
	var dfs unix.Statfs_t
	if unix.Statfs("/dev/fd", &dfs) != nil || unix.ByteSliceToString(dfs.Fstypename[:]) != "devfs" {
		return nil, fail("platform_unavailable")
	}
	s.root, e = os.OpenRoot("/dev/fd/" + strconv.Itoa(fd))
	if e != nil {
		return nil, fail("platform_unavailable")
	}
	f, e := s.root.OpenFile(".", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if e != nil {
		return nil, fail("platform_unavailable")
	}
	ri, se := f.Stat()
	ce := f.Close()
	if se != nil || ce != nil || !os.SameFile(opened, ri) {
		return nil, fail("source_changed")
	}
	after, e := os.Lstat(name)
	if e != nil || !after.IsDir() || !os.SameFile(opened, after) {
		return nil, fail("source_changed")
	}
	return s, nil
}
func (s *source) close() error {
	var a, b error
	if s.root != nil {
		a = s.root.Close()
		s.root = nil
	}
	if s.anchor != nil {
		b = s.anchor.Close()
		s.anchor = nil
	}
	return errors.Join(a, b)
}
func (s *source) pin(rel string, nofollow bool) (*pinned, error) {
	if rel == "" || strings.HasPrefix(rel, "/") || strings.IndexByte(rel, 0) >= 0 || len(rel) > 4096 {
		return nil, syscall.EXDEV
	}
	todo := strings.Split(rel, "/")
	var done []string
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
			if len(done) == 0 {
				return nil, syscall.EXDEV
			}
			done = done[:len(done)-1]
			if len(todo) > 0 {
				continue
			}
			n = "."
		}
		parent := "."
		if len(done) > 0 {
			parent = strings.Join(done, "/")
		}
		// All parent components were checked as physical directories, on the same
		// immutable filesystem. No source symlink reaches this OpenFile call.
		dir, e := s.root.OpenFile(parent, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if e != nil {
			return nil, e
		}
		fs, e := darwinFS(int(dir.Fd()), s.trustedWritable)
		if e != nil || fs.Fsid != s.fsid {
			dir.Close()
			return nil, syscall.EXDEV
		}
		path := parent + "/" + n
		info, e := s.root.Lstat(path)
		if e != nil {
			dir.Close()
			return nil, e
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok || st.Dev != s.dev {
			dir.Close()
			return nil, syscall.EXDEV
		}
		p := &pinned{file: dir, info: info, name: n, source: s}
		if info.Mode()&os.ModeSymlink != 0 && (!nofollow || len(todo) > 0) {
			links++
			if links > 40 {
				dir.Close()
				return nil, syscall.ELOOP
			}
			target, e := p.link(4096)
			ce := dir.Close()
			if e != nil {
				return nil, e
			}
			if ce != nil {
				return nil, ce
			}
			if target == "" || strings.HasPrefix(target, "/") {
				return nil, syscall.EXDEV
			}
			todo = append(strings.Split(target, "/"), todo...)
			continue
		}
		if len(todo) == 0 {
			return p, nil
		}
		if !info.IsDir() {
			dir.Close()
			return nil, syscall.ENOTDIR
		}
		if e := dir.Close(); e != nil {
			return nil, e
		}
		if n != "." {
			done = append(done, n)
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
	return string(b[:n]), nil
}
func (p *pinned) reopen(directory bool) (*os.File, error) {
	if (directory && !p.info.IsDir()) || (!directory && !p.info.Mode().IsRegular()) {
		return nil, fail("wrong_kind")
	}
	fs, e := darwinFS(int(p.file.Fd()), p.source.trustedWritable)
	if e != nil || fs.Fsid != p.source.fsid {
		return nil, fail("filesystem_unavailable")
	}
	flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
	if directory {
		flags |= unix.O_DIRECTORY
	}
	// Safe only because the checked immutable entry cannot change type/object.
	fd, e := unix.Openat(int(p.file.Fd()), p.name, flags, 0)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(fd), "source-data")
	openedFS, e := darwinFS(fd, p.source.trustedWritable)
	if e != nil || openedFS.Fsid != p.source.fsid {
		f.Close()
		return nil, fail("filesystem_unavailable")
	}
	info, e := f.Stat()
	if e != nil || !same(p.info, info) {
		f.Close()
		return nil, fail("source_changed")
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
	if a == nil || b == nil || !os.SameFile(a, b) || a.Mode() != b.Mode() || a.Size() != b.Size() || !a.ModTime().Equal(b.ModTime()) {
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
