//go:build darwin && arm64

package packageview

import (
	"context"
	"errors"
	"golang.org/x/sys/unix"
	"os"
	"runtime"
	"strings"
	"syscall"
)

// Writable APFS requires quiescence from Open through Close, including ancestor
// bindings. A parent/name pin is NOT an inode capability. Hostile replacement
// can cause a forbidden open before fstat detects it. See ADR 0006.
type source struct {
	work  int
	ctx   context.Context
	hooks *captureHooks

	anchor     *os.File
	legacyInfo os.FileInfo
	dev        int32
	fsid       unix.Fsid
	physical   string
	bindings   map[string]os.FileInfo
	links      map[string]string
	ancestors  map[string]bool
}
type pinned struct {
	file   *os.File // owns only the final physical parent
	info   os.FileInfo
	name   string
	parent string
	source *source
}

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
// This profile already permits ordinary quiescent writable local APFS (see
// ADR 0006), so a generated-staging proof adds nothing here and is
// intentionally ignored, matching source_linux.go's identical reasoning.
func openSource(name string, _ GeneratedStaging) (*source, error) {
	return openSourceContext(context.Background(), name)
}
func openSourceContext(ctx context.Context, name string) (result *source, err error) {
	s := &source{ctx: ctx, bindings: map[string]os.FileInfo{}, links: map[string]string{}, ancestors: map[string]bool{}}
	defer func() {
		if result == nil {
			_ = s.close()
		}
	}()
	name = strings.TrimRight(name, "/")
	if name == "" {
		name = "/"
	}
	if !strings.HasPrefix(name, "/") {
		cwd, e := os.Getwd()
		if e != nil {
			return nil, fail("root_unreadable")
		}
		name = cwd + "/" + name
	}
	// No filepath.Clean: a/.. must visit a before ascending.
	p, e := s.resolve(name, true, true)
	if e != nil {
		if fatal := fatalAcquisition(e); fatal != nil {
			return nil, fatal
		}
		if errors.Is(e, syscall.ENOENT) {
			return nil, fail("root_absent")
		}
		if errors.Is(e, syscall.ENOTDIR) {
			return nil, fail("root_wrong_kind")
		}
		return nil, fail("root_unreadable")
	}
	defer p.file.Close()
	if !p.info.IsDir() {
		return nil, fail("root_wrong_kind")
	}
	s.physical = physicalName(p.parent, p.name)
	s.anchor, e = p.openDirectory()
	if e != nil {
		return nil, e
	}
	fs, e := darwinFS(int(s.anchor.Fd()))
	if e != nil {
		return nil, e
	}
	s.dev = p.info.Sys().(*syscall.Stat_t).Dev
	s.fsid = fs.Fsid
	// Only root-selection records tolerate unrelated sibling directory activity.
	for path := range s.bindings {
		s.ancestors[path] = path != s.physical
	}
	if e := s.verifyBindings(); e != nil {
		return nil, e
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
func (s *source) pin(rel string, nofollow bool) (*pinned, error) {
	s.work = 0
	if e := s.verifyAncestors(); e != nil {
		return nil, e
	}
	return s.resolve(rel, nofollow, false)
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
	target := string(b[:n])
	path := physicalName(p.parent, p.name)
	if prior, ok := p.source.links[path]; ok && prior != target {
		return "", fail("source_changed")
	}
	p.source.links[path] = target
	return target, nil
}
func (p *pinned) openDirectory() (*os.File, error) {
	return p.open(unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW | unix.O_CLOEXEC)
}
func (p *pinned) open(flags int) (*os.File, error) {
	if p.source.hooks != nil && p.source.hooks.nativeOpen != nil {
		p.source.hooks.nativeOpen(p.name, flags)
	}
	fd, e := unix.Openat(int(p.file.Fd()), p.name, flags, 0)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(fd), "source-data")
	info, e := f.Stat()
	equal := e == nil && same(p.info, info)
	if e == nil && p.info.IsDir() && (p.source.anchor == nil || p.source.ancestors[physicalName(p.parent, p.name)]) {
		equal = bindingSame(p.info, info)
	}
	if !equal {
		f.Close()
		return nil, fail("source_changed")
	}
	fs, e := darwinFS(fd)
	if e != nil {
		f.Close()
		return nil, e
	}
	if p.source.anchor != nil && (p.parent == p.source.physical || strings.HasPrefix(p.parent, p.source.physical+"/")) && fs.Fsid != p.source.fsid {
		f.Close()
		return nil, fail("filesystem_unavailable")
	}
	return f, nil
}
func (p *pinned) reopen(directory bool) (*os.File, error) {
	p.source.work = 0
	if (directory && !p.info.IsDir()) || (!directory && !p.info.Mode().IsRegular()) {
		return nil, fail("wrong_kind")
	}
	if e := p.source.verifyAncestors(); e != nil {
		return nil, e
	}
	// Replay the physical parent directory by directory and compare the held FD.
	dir, e := p.source.directory(p.parent)
	if e != nil {
		return nil, e
	}
	current, e := dir.Stat()
	held, he := p.file.Stat()
	ce := dir.Close()
	if e != nil || he != nil || ce != nil || !same(current, held) {
		return nil, fail("source_changed")
	}
	now, e := darwinStat(int(p.file.Fd()), p.name)
	if e != nil || !same(p.info, now) {
		return nil, fail("source_changed")
	}
	flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_NOCTTY | unix.O_CLOEXEC
	if directory {
		flags |= unix.O_DIRECTORY
	}
	// Quiescence supplies name stability, NOT the flags or post-open comparison.
	return p.open(flags)
}
func sameIdentity(a, b os.FileInfo) bool {
	if a == nil || b == nil {
		return false
	}
	x, ok := a.Sys().(*syscall.Stat_t)
	y, ok2 := b.Sys().(*syscall.Stat_t)
	return ok && ok2 && x.Dev == y.Dev && x.Ino == y.Ino
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
	return ok && ok2 && x.Ctimespec == y.Ctimespec && x.Nlink == y.Nlink && x.Gen == y.Gen && x.Flags == y.Flags && x.Uid == y.Uid && x.Gid == y.Gid
}
func multipleLinks(i os.FileInfo) bool {
	s, ok := i.Sys().(*syscall.Stat_t)
	return !ok || s.Nlink != 1
}

func (s *source) sourceHooks(h *captureHooks) { s.hooks = h }

func (s *source) phaseContext(ctx context.Context) { s.ctx = ctx }
