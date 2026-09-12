//go:build darwin && arm64

package packageview

import (
	"golang.org/x/sys/unix"
	"os"
	"path"
	"strings"
	"syscall"
	"time"
)

// FileInfo for metadata-only fstatat. Sys uses the same stat representation as
// os.File.Stat; os.SameFile itself only accepts Go's private fileStat type.
type darwinInfo struct {
	name string
	st   syscall.Stat_t
}

func (i darwinInfo) Name() string       { return i.name }
func (i darwinInfo) Size() int64        { return i.st.Size }
func (i darwinInfo) ModTime() time.Time { return time.Unix(i.st.Mtimespec.Sec, i.st.Mtimespec.Nsec) }
func (i darwinInfo) IsDir() bool        { return i.Mode().IsDir() }
func (i darwinInfo) Sys() any           { s := i.st; return &s }
func (i darwinInfo) Mode() os.FileMode {
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
func darwinStat(fd int, name string) (os.FileInfo, error) {
	var u unix.Stat_t
	if e := unix.Fstatat(fd, name, &u, unix.AT_SYMLINK_NOFOLLOW); e != nil {
		return nil, e
	}
	// SF_DATALESS is public Darwin stat metadata. OFF is also required to guard
	// a transition after this observation; APFS does not imply resident content.
	if u.Flags&unix.SF_DATALESS != 0 {
		return nil, fail("filesystem_unavailable")
	}
	s := syscall.Stat_t{Dev: u.Dev, Ino: u.Ino, Mode: u.Mode, Nlink: u.Nlink, Uid: u.Uid, Gid: u.Gid, Rdev: u.Rdev,
		Size: u.Size, Flags: u.Flags, Gen: u.Gen,
		Mtimespec: syscall.Timespec{Sec: u.Mtim.Sec, Nsec: u.Mtim.Nsec},
		Ctimespec: syscall.Timespec{Sec: u.Ctim.Sec, Nsec: u.Ctim.Nsec}}
	return darwinInfo{name, s}, nil
}
func physicalName(parent, name string) string {
	if name == "." {
		return parent
	}
	if parent == "/" {
		return "/" + name
	}
	return parent + "/" + name
}
func bindingSame(a, b os.FileInfo) bool {
	if !sameIdentity(a, b) || a.Mode() != b.Mode() {
		return false
	}
	x := a.Sys().(*syscall.Stat_t)
	y := b.Sys().(*syscall.Stat_t)
	// Parent mtime/size/ctime/nlink can change from unrelated sibling activity.
	return x.Uid == y.Uid && x.Gid == y.Gid && x.Flags == y.Flags && x.Gen == y.Gen
}
func (s *source) observe(name string, info os.FileInfo) error {
	if len(name) > 4096 {
		return fail("path_limit")
	}
	if old, ok := s.bindings[name]; ok {
		equal := same(old, info)
		if s.ancestors[name] || s.anchor == nil {
			equal = bindingSame(old, info)
		}
		if !equal {
			return fail("source_changed")
		}
		return nil
	}
	// Finite metadata ownership, never one FD per entry. Includes root selection
	// and up to the existing resolver step budget of physical aliases.
	if len(s.bindings) >= 10000+8192 {
		return fail("entry_limit")
	}
	s.bindings[name] = info
	return nil
}

// directory replays physical prefixes using only single-component openat.
// Each prior binding is compared before opening/discovering the next child.
func (s *source) directory(name string) (result *os.File, err error) {
	if e := s.step(); e != nil {
		return nil, e
	}
	fd, e := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(fd), "source-directory")
	defer func() {
		if result == nil {
			f.Close()
		}
	}()
	info, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if _, e := darwinFS(fd); e != nil {
		return nil, e
	}
	if e := s.observe("/", info); e != nil {
		return nil, e
	}
	prefix := "/"
	for _, n := range strings.Split(strings.TrimPrefix(name, "/"), "/") {
		if e := s.step(); e != nil {
			return nil, e
		}
		if n == "" {
			continue
		}
		info, e := darwinStat(int(f.Fd()), n)
		next := physicalName(prefix, n)
		if e != nil {
			if _, seen := s.bindings[next]; seen {
				return nil, acquisitionError(e, "source_changed")
			}
			return nil, e
		}
		if e := s.observe(next, info); e != nil {
			return nil, e
		}
		if !info.IsDir() {
			return nil, fail("source_changed")
		}
		p := &pinned{file: f, info: info, name: n, parent: prefix, source: s}
		child, e := p.openDirectory()
		if e != nil {
			return nil, e
		}
		if e := f.Close(); e != nil {
			child.Close()
			return nil, fail("close_failed")
		}
		f = child
		prefix = next
	}
	return f, nil
}
func (s *source) resolve(rel string, nofollow, selecting bool) (*pinned, error) {
	if rel == "" || len(rel) > 4096 || strings.IndexByte(rel, 0) >= 0 || (!selecting && strings.HasPrefix(rel, "/")) {
		return nil, syscall.EXDEV
	}
	base := s.physical
	if selecting {
		base = "/"
	}
	parent := base
	todo := strings.Split(strings.TrimPrefix(rel, "/"), "/")
	links, steps := 0, 0
	var owned *os.File
	defer func() {
		if owned != nil {
			owned.Close()
		}
	}()
	for len(todo) > 0 {
		if e := s.step(); e != nil {
			return nil, e
		}
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
			if !selecting && parent == base {
				return nil, syscall.EXDEV
			}
			parent = path.Dir(parent)
			if len(todo) > 0 {
				continue
			}
			n = "."
		}
		dir, e := s.directory(parent)
		if e != nil {
			return nil, e
		}
		owned = dir
		info, e := darwinStat(int(dir.Fd()), n)
		physical := physicalName(parent, n)
		if e != nil {
			dir.Close()
			if _, seen := s.bindings[physical]; seen {
				return nil, acquisitionError(e, "source_changed")
			}
			return nil, e
		}
		if e = s.observe(physical, info); e != nil {
			dir.Close()
			return nil, e
		}
		if !selecting && info.Sys().(*syscall.Stat_t).Dev != s.dev {
			dir.Close()
			return nil, syscall.EXDEV
		}
		p := &pinned{file: dir, info: info, name: n, parent: parent, source: s}
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
				return nil, fail("close_failed")
			}
			if target == "" || (!selecting && strings.HasPrefix(target, "/")) {
				return nil, syscall.EXDEV
			}
			if strings.HasPrefix(target, "/") {
				parent = "/"
			}
			if len(todo)+strings.Count(target, "/")+1 > 8192 {
				return nil, syscall.ELOOP
			}
			todo = append(strings.Split(target, "/"), todo...)
			continue
		}
		if len(todo) == 0 {
			owned = nil
			return p, nil
		}
		if !info.IsDir() {
			dir.Close()
			return nil, syscall.ENOTDIR
		}
		// Approve and bind before advancing even if the next component is '..'.
		child, e := p.openDirectory()
		ce := dir.Close()
		if e != nil {
			return nil, e
		}
		childErr := child.Close()
		if ce != nil || childErr != nil {
			return nil, fail("close_failed")
		}
		parent = physical
	}
	return nil, syscall.ENOENT
}
func (s *source) verifyBinding(name string, old os.FileInfo) error {
	parent, n := path.Dir(name), path.Base(name)
	if name == "/" {
		parent = "/"
		n = "."
	}
	dir, e := s.directory(parent)
	if e != nil {
		return acquisitionError(e, "source_changed")
	}
	defer dir.Close()
	info, e := darwinStat(int(dir.Fd()), n)
	if e != nil {
		return acquisitionError(e, "source_changed")
	}
	equal := same(old, info)
	if s.ancestors[name] {
		equal = bindingSame(old, info)
	}
	if !equal {
		return fail("source_changed")
	}
	if text, ok := s.links[name]; ok {
		p := &pinned{file: dir, info: info, name: n, parent: parent, source: s}
		got, e := p.link(4096)
		if e != nil {
			return acquisitionError(e, "source_changed")
		}
		if got != text {
			return fail("source_changed")
		}
	}
	return nil
}
func (s *source) verifyAncestors() error {
	for name := range s.ancestors {
		if e := s.verifyBinding(name, s.bindings[name]); e != nil {
			return e
		}
	}
	if s.anchor != nil {
		old := s.bindings[s.physical]
		if e := s.verifyBinding(s.physical, old); e != nil {
			return e
		}
	}
	return nil
}
func (s *source) verifyBindings() error {
	for name, info := range s.bindings {
		s.work = 0
		if e := s.verifyBinding(name, info); e != nil {
			return e
		}
	}
	return nil
}

// Include replay work in the resolver ceiling, not only pending link expansion.
func (s *source) step() error {
	if e := contextError(s.ctx); e != nil {
		return e
	}
	s.work++
	if s.work > 8192 {
		return fail("path_limit")
	}
	return nil
}
