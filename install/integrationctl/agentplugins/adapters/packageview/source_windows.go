//go:build windows && amd64

package packageview

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"golang.org/x/sys/windows"
)

// This profile accepts local fixed-drive NTFS only. Every source component is
// opened with NtCreateFile relative to a held directory, with no reparse follow
// and no data access. ReOpenFile first acquires protected pins by HANDLE:
// regular/link pins use DELETE for share accounting without FILE_READ_DATA;
// proven directories use list access and deny delete sharing to retain ancestry.
// ReOpenFile then upgrades verified regular objects by HANDLE. DELETE permission is
// a capability prerequisite only; no source deletion or other write is issued.
// Device-map changes, hostile filesystem filters and privileged volume changes
// are outside this local-kernel profile. No UNC, ADS or DOS device path fallback.
type source struct {
	anchor     *os.File
	legacyInfo os.FileInfo
	volume     uint32
	records    map[winSnapshot]*winObservation
}
type pinned struct {
	file *os.File
	info os.FileInfo
}

// Keep genuine os.FileInfo values: common legacy checks use os.SameFile, which
// rejects custom FileInfo wrappers. Go's Windows Sys omits nlink and ChangeTime.
// A bounded, lease-owned side table supplies those facts to the existing same
// seam, including comparisons against fresh File.Stat from common read code.
// Entries and their metadata-only handles are removed on every source.close.
var winInfos = struct {
	sync.RWMutex
	m map[os.FileInfo]*winObservation
}{m: make(map[os.FileInfo]*winObservation)}

type winBasic struct {
	Creation, Access, Write, Change int64
	Attributes                      uint32
	_                               uint32
}
type winSnapshot struct {
	Volume, High, Low, Links, Attributes uint32
	Size                                 int64
	Creation, Write, Change              int64
}
type winObservation struct {
	file *os.File
	info os.FileInfo
	meta winSnapshot
}

const winShare = windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE

var winReOpen = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReOpenFile")

func winMeta(f *os.File) (winSnapshot, error) {
	defer runtime.KeepAlive(f)
	h := windows.Handle(f.Fd())
	var i windows.ByHandleFileInformation
	if e := windows.GetFileInformationByHandle(h, &i); e != nil {
		return winSnapshot{}, e
	}
	var b winBasic
	if e := windows.GetFileInformationByHandleEx(h, windows.FileBasicInfo, (*byte)(unsafe.Pointer(&b)), uint32(unsafe.Sizeof(b))); e != nil {
		return winSnapshot{}, e
	}
	return winSnapshot{i.VolumeSerialNumber, i.FileIndexHigh, i.FileIndexLow, i.NumberOfLinks, i.FileAttributes, int64(i.FileSizeHigh)<<32 | int64(i.FileSizeLow), b.Creation, b.Write, b.Change}, nil
}
func winOpen(parent windows.Handle, name string, directory bool) (*os.File, error) {
	u, e := windows.NewNTUnicodeString(name)
	if e != nil {
		return nil, e
	}
	oa := windows.OBJECT_ATTRIBUTES{RootDirectory: parent, ObjectName: u, Attributes: windows.OBJ_CASE_INSENSITIVE}
	oa.Length = uint32(unsafe.Sizeof(oa))
	options := uint32(windows.FILE_OPEN_REPARSE_POINT | windows.FILE_OPEN_NO_RECALL | windows.FILE_SYNCHRONOUS_IO_NONALERT | windows.FILE_OPEN_FOR_BACKUP_INTENT)
	// This first type probe has no data access. Attribute-only handles do not
	// establish the share protection needed below; remember upgrades by HANDLE
	// to a DELETE-access metadata pin before trusting the object for data access.
	if directory {
		options |= windows.FILE_DIRECTORY_FILE
	}
	var h windows.Handle
	e = windows.NtCreateFile(&h, windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE, &oa, &windows.IO_STATUS_BLOCK{}, nil, 0, winShare, windows.FILE_OPEN, options, 0, 0)
	if e != nil {
		return nil, e
	}
	return os.NewFile(uintptr(h), "source-metadata"), nil
}

// winReopen never resolves a source pathname. OPEN_REPARSE_POINT is retained
// even for the first metadata upgrade, so a concurrent reparse conversion cannot
// redirect acquisition into another namespace.
func winReopen(f *os.File, access, share uint32) (*os.File, error) {
	defer runtime.KeepAlive(f)
	flags := uint32(windows.FILE_FLAG_OPEN_REPARSE_POINT | windows.FILE_FLAG_OPEN_NO_RECALL | windows.FILE_FLAG_BACKUP_SEMANTICS)
	h, _, e := winReOpen.Call(f.Fd(), uintptr(access), uintptr(share), uintptr(flags))
	if windows.Handle(h) == windows.InvalidHandle {
		return nil, e
	}
	return os.NewFile(h, "source-handle"), nil
}
func winDup(f *os.File) (*os.File, error) {
	defer runtime.KeepAlive(f)
	var h windows.Handle
	e := windows.DuplicateHandle(windows.CurrentProcess(), windows.Handle(f.Fd()), windows.CurrentProcess(), &h, 0, false, windows.DUPLICATE_SAME_ACCESS)
	if e != nil {
		return nil, e
	}
	return os.NewFile(uintptr(h), "source-pin"), nil
}
func (s *source) remember(f *os.File) (*pinned, error) {
	// Takes ownership on all paths; hard bounds include root-selection ancestors.
	defer f.Close()
	meta, e := winMeta(f)
	if e != nil {
		return nil, e
	}
	if meta.Volume != s.volume || meta.Attributes&(windows.FILE_ATTRIBUTE_OFFLINE|windows.FILE_ATTRIBUTE_RECALL_ON_OPEN|windows.FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS|windows.FILE_ATTRIBUTE_DEVICE|windows.FILE_ATTRIBUTE_ENCRYPTED) != 0 {
		return nil, syscall.EXDEV
	}
	kind, e := windows.GetFileType(windows.Handle(f.Fd()))
	if e != nil || kind != windows.FILE_TYPE_DISK {
		return nil, syscall.EXDEV
	}
	info, e := f.Stat()
	if e != nil {
		return nil, e
	}
	after, e := winMeta(f)
	if e != nil || meta != after {
		return nil, fail("source_changed")
	}
	if meta.Attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		// Only actual symlinks have portable link semantics. Junctions/mount points,
		// cloud placeholders, AF_UNIX and unknown filters never reach a data open.
		var tag struct{ Attributes, Tag uint32 }
		if windows.GetFileInformationByHandleEx(windows.Handle(f.Fd()), windows.FileAttributeTagInfo, (*byte)(unsafe.Pointer(&tag)), uint32(unsafe.Sizeof(tag))) != nil || tag.Tag != windows.IO_REPARSE_TAG_SYMLINK || info.Mode()&os.ModeSymlink == 0 {
			return nil, syscall.EXDEV
		}
	}
	if o := s.records[meta]; o != nil {
		dup, e := winDup(o.file)
		if e != nil {
			return nil, e
		}
		return &pinned{dup, o.info}, nil
	}
	if len(s.records) >= 20000 {
		return nil, fail("entry_limit")
	}
	share := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_DELETE)
	access := uint32(windows.FILE_READ_ATTRIBUTES | windows.DELETE)
	if info.IsDir() {
		// Listing access participates in share accounting without requiring DELETE
		// permission on drive/root-selection ancestors. This is acquired only
		// after metadata proves a disk directory, and always by the same HANDLE.
		access = windows.FILE_READ_ATTRIBUTES | windows.FILE_LIST_DIRECTORY
		share &^= windows.FILE_SHARE_DELETE
	}
	// DELETE is an access right, not an operation. Unlike READ_ATTRIBUTES alone,
	// it participates in share accounting. Denying WRITE prevents new data
	// writers/reparse setters; denying DELETE on directories holds ancestry.
	// Existing incompatible handles cause an availability failure, no fallback.
	held, e := winReopen(f, access, share)
	if e != nil {
		return nil, e
	}
	locked, e := winMeta(held)
	if e != nil || locked != meta {
		held.Close()
		return nil, fail("source_changed")
	}
	dup, e := winDup(held)
	if e != nil {
		held.Close()
		return nil, e
	}
	o := &winObservation{held, info, meta}
	s.records[meta] = o
	winInfos.Lock()
	winInfos.m[info] = o
	winInfos.Unlock()
	return &pinned{dup, info}, nil
}
func openSource(name string) (_ *source, err error) {
	name = strings.ReplaceAll(name, "/", `\`)
	// Bootstrap ONLY a literal fixed drive root. The remainder never goes through
	// Win32 path cleaning. In particular junction/../x is evaluated in order.
	if len(name) < 3 || name[1] != ':' || name[2] != '\\' || !((name[0] >= 'A' && name[0] <= 'Z') || (name[0] >= 'a' && name[0] <= 'z')) {
		return nil, fail("root_unreadable")
	}
	drive := name[:3]
	u, e := windows.UTF16PtrFromString(drive)
	if e != nil {
		return nil, fail("root_unreadable")
	}
	if windows.GetDriveType(u) != windows.DRIVE_FIXED {
		return nil, fail("filesystem_unavailable")
	}
	f, e := winOpen(0, `\??\`+drive, true)
	if e != nil {
		return nil, fail("root_unreadable")
	}
	s := &source{records: make(map[winSnapshot]*winObservation)}
	defer func() {
		if err != nil {
			_ = s.close()
		}
	}()
	// Own the bootstrap immediately, including all capability failures.
	s.anchor = f
	var fs [32]uint16
	if windows.GetVolumeInformationByHandle(windows.Handle(f.Fd()), nil, 0, &s.volume, nil, nil, &fs[0], uint32(len(fs))) != nil || windows.UTF16ToString(fs[:]) != "NTFS" {
		return nil, fail("filesystem_unavailable")
	}
	p, e := s.rememberMustDuplicate(f)
	if e != nil {
		return nil, fail("platform_unavailable")
	}
	s.anchor.Close()
	s.anchor = p.file
	rest := strings.TrimRight(name[3:], `\`)
	if rest != "" {
		p, e = s.walk(rest, true, true)
		if e != nil {
			if stateOf(e) == Absent {
				return nil, fail("root_absent")
			}
			return nil, fail("root_unreadable")
		}
		if !p.info.IsDir() {
			p.file.Close()
			return nil, fail("root_wrong_kind")
		}
		s.anchor.Close()
		s.anchor = p.file
	}
	return s, nil
}
func (s *source) rememberMustDuplicate(f *os.File) (*pinned, error) {
	dup, e := winDup(f)
	if e != nil {
		return nil, e
	}
	return s.remember(dup)
}
func (s *source) close() error {
	var es []error
	// same holds the read lock while querying handles; close cannot race it.
	winInfos.Lock()
	for _, o := range s.records {
		delete(winInfos.m, o.info)
		es = append(es, o.file.Close())
	}
	s.records = nil
	winInfos.Unlock()
	if s.anchor != nil {
		es = append(es, s.anchor.Close())
		s.anchor = nil
	}
	return errors.Join(es...)
}
func (s *source) pin(rel string, nofollow bool) (*pinned, error) { return s.walk(rel, nofollow, false) }
func winParts(p string) ([]string, error) {
	if p == "" || len(p) > 4096 || !utf8.ValidString(p) || strings.ContainsAny(p, ":\x00*?\"") || p[0] == '/' || p[0] == '\\' {
		return nil, syscall.EXDEV
	}
	return strings.Split(strings.ReplaceAll(p, `\`, "/"), "/"), nil
}
func (s *source) walk(rel string, nofollow, rootSelection bool) (*pinned, error) {
	todo, e := winParts(rel)
	if e != nil {
		return nil, e
	}
	first, e := winDup(s.anchor)
	if e != nil {
		return nil, e
	}
	stack := []*os.File{first}
	defer func() {
		for _, f := range stack {
			f.Close()
		}
	}()
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
			return s.rememberMustDuplicate(stack[len(stack)-1])
		}
		if n == ".." {
			if len(stack) == 1 {
				return nil, syscall.EXDEV
			}
			stack[len(stack)-1].Close()
			stack = stack[:len(stack)-1]
			if len(todo) > 0 {
				continue
			}
			return s.rememberMustDuplicate(stack[len(stack)-1])
		}
		// The shared scratch-overlap check uses Win32 EvalSymlinks. Root names
		// must therefore have matching NT/Win32 spellings; reject DOS devices and
		// terminal dots/spaces instead of allowing that check to name another root.
		if rootSelection && (!filepath.IsLocal(n) || strings.HasSuffix(n, ".") || strings.HasSuffix(n, " ") || strings.ContainsAny(n, "<>|")) {
			return nil, syscall.EXDEV
		}
		parent := windows.Handle(stack[len(stack)-1].Fd())
		// First pin has attributes-only access, regardless of the replaceable type.
		f, e := winOpen(parent, n, false)
		if e != nil {
			return nil, e
		}
		p, e := s.remember(f)
		if e != nil {
			return nil, e
		}
		if p.info.Mode()&os.ModeSymlink != 0 {
			if rootSelection {
				p.file.Close()
				return nil, syscall.EXDEV
			}
			if !nofollow || len(todo) > 0 {
				links++
				if links > 40 {
					p.file.Close()
					return nil, syscall.ELOOP
				}
				target, relative, e := p.reparse(4096)
				ce := p.file.Close()
				if e != nil {
					return nil, e
				}
				if ce != nil {
					return nil, ce
				}
				if !relative {
					return nil, syscall.EXDEV
				}
				parts, e := winParts(target)
				if e != nil {
					return nil, e
				}
				todo = append(parts, todo...)
				continue
			}
		}
		if len(todo) == 0 {
			return p, nil
		}
		if !p.info.IsDir() {
			p.file.Close()
			return nil, syscall.ENOTDIR
		}
		stack = append(stack, p.file)
	}
	return nil, syscall.ENOENT
}
func (p *pinned) reparse(limit int64) (string, bool, error) {
	defer runtime.KeepAlive(p)
	if limit < 0 {
		return "", false, fail("byte_limit")
	}
	// Windows returns the complete reparse record. The kernel maximum is 16 KiB;
	// validate all offsets before allocating/decoding at most 4096 UTF-16 units.
	b := make([]byte, 16*1024)
	var n uint32
	e := windows.DeviceIoControl(windows.Handle(p.file.Fd()), windows.FSCTL_GET_REPARSE_POINT, nil, 0, &b[0], uint32(len(b)), &n, nil)
	if e != nil {
		return "", false, e
	}
	if n < 20 || n > uint32(len(b)) || binary.LittleEndian.Uint32(b[:4]) != windows.IO_REPARSE_TAG_SYMLINK {
		return "", false, syscall.EXDEV
	}
	end := 8 + int(binary.LittleEndian.Uint16(b[4:6]))
	off := 20 + int(binary.LittleEndian.Uint16(b[8:10]))
	size := int(binary.LittleEndian.Uint16(b[10:12]))
	flags := binary.LittleEndian.Uint32(b[16:20])
	if end > int(n) || end < 20 || off < 20 || size%2 != 0 || off%2 != 0 || off+size > end || size > 8192 || flags&^uint32(1) != 0 {
		return "", false, syscall.EXDEV
	}
	u := make([]uint16, size/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[off+i*2:])
	}
	// Reject invalid surrogate pairs instead of changing the target identity.
	for i := 0; i < len(u); i++ {
		v := u[i]
		if v == 0 {
			return "", false, syscall.EXDEV
		}
		if v >= 0xd800 && v <= 0xdbff {
			if i+1 == len(u) || u[i+1] < 0xdc00 || u[i+1] > 0xdfff {
				return "", false, syscall.EXDEV
			}
			i++
		} else if v >= 0xdc00 && v <= 0xdfff {
			return "", false, syscall.EXDEV
		}
	}
	target := string(utf16.Decode(u))
	if int64(len(target)) > limit {
		return "", false, fail("byte_limit")
	}
	return target, flags == 1, nil
}
func (p *pinned) link(limit int64) (string, error) { v, _, e := p.reparse(limit); return v, e }
func (p *pinned) reopen(directory bool) (*os.File, error) {
	if (directory && !p.info.IsDir()) || (!directory && !p.info.Mode().IsRegular()) {
		return nil, fail("wrong_kind")
	}
	// Refuse any attribute/type/reparse/link-count change before upgrading access.
	if !same(p.info, p.info) {
		return nil, fail("source_changed")
	}
	f, e := winReopen(p.file, windows.GENERIC_READ, winShare)
	if e != nil {
		return nil, e
	}
	info, se := f.Stat()
	if se != nil || !same(p.info, info) {
		f.Close()
		return nil, fail("source_changed")
	}
	return f, nil
}
func same(a, b os.FileInfo) bool {
	if a == nil || b == nil || !os.SameFile(a, b) || a.Mode() != b.Mode() || a.Size() != b.Size() || !a.ModTime().Equal(b.ModTime()) {
		return false
	}
	x, ok := a.Sys().(*syscall.Win32FileAttributeData)
	y, ok2 := b.Sys().(*syscall.Win32FileAttributeData)
	if !ok || !ok2 || x.FileAttributes != y.FileAttributes || x.CreationTime != y.CreationTime {
		return false
	}
	winInfos.RLock()
	defer winInfos.RUnlock()
	found := false
	for _, i := range []os.FileInfo{a, b} {
		if o := winInfos.m[i]; o != nil {
			found = true
			m, e := winMeta(o.file)
			if e != nil || m != o.meta {
				return false
			}
		}
	}
	return found // no guessed ctime/nlink semantics for unregistered FileInfo
}
func multipleLinks(i os.FileInfo) bool {
	winInfos.RLock()
	defer winInfos.RUnlock()
	o := winInfos.m[i]
	if o == nil {
		return true
	}
	now, e := winMeta(o.file)
	return e != nil || now != o.meta || now.Links != 1
}
func stateOf(e error) State {
	if errors.Is(e, syscall.ENOENT) || errors.Is(e, windows.ERROR_FILE_NOT_FOUND) || errors.Is(e, windows.ERROR_PATH_NOT_FOUND) || errors.Is(e, windows.STATUS_OBJECT_NAME_NOT_FOUND) || errors.Is(e, windows.STATUS_OBJECT_PATH_NOT_FOUND) {
		return Absent
	}
	if errors.Is(e, syscall.ENOTDIR) || errors.Is(e, windows.STATUS_NOT_A_DIRECTORY) {
		return WrongKind
	}
	if errors.Is(e, syscall.EXDEV) || errors.Is(e, syscall.ELOOP) || errors.Is(e, windows.STATUS_REPARSE_POINT_ENCOUNTERED) || errors.Is(e, windows.STATUS_IO_REPARSE_TAG_NOT_HANDLED) {
		return Blocked
	}
	return Unreadable
}
