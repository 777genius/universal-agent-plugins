//go:build windows && amd64

package packageview

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func nativeFixture(t *testing.T, build func(string)) string {
	t.Helper()
	root := t.TempDir()
	build(root)
	return root
}
func winRecordCount() int { winInfos.RLock(); defer winInfos.RUnlock(); return len(winInfos.m) }
func TestWindowsModeAndChangeMetadata(t *testing.T) {
	root := nativeFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "core") })
	s := nativeSource(t, root)
	p, e := s.pin("plugin.json", false)
	if e != nil {
		t.Fatal(e)
	}
	defer p.file.Close()
	if p.info.Mode()&0111 != 0 {
		t.Fatal("invented POSIX executable bits")
	}
	if multipleLinks(p.info) {
		t.Fatal("single-link file blocked")
	}
	if !same(p.info, p.info) {
		t.Fatal("Windows metadata cannot compare itself")
	}
	// Metadata pins deny concurrent data writers, including conversion to reparse.
	f, e := os.OpenFile(filepath.Join(root, "plugin.json"), os.O_WRONLY, 0)
	if e == nil {
		f.Close()
		t.Fatal("data writer opened while native pin held")
	}
	if !errors.Is(e, windows.ERROR_SHARING_VIOLATION) {
		t.Fatalf("write denial was not sharing protection: %v", e)
	}
	// Windows attributes are compared honestly, without a POSIX chmod claim.
	if e := os.Chmod(filepath.Join(root, "plugin.json"), 0400); e != nil {
		t.Fatal(e)
	}
	if same(p.info, p.info) {
		t.Fatal("attribute/change-time mutation missed")
	}
}
func TestWindowsHandleLifetimeAndFailureCleanup(t *testing.T) {
	before := winRecordCount()
	root := nativeFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "core") })
	for i := 0; i < 10; i++ {
		scratch := t.TempDir()
		_, e := (Reader{TempDir: scratch, Limits: Limits{PluginBytes: 1}}).Open(context.Background(), root)
		var safe *Error
		if !errors.As(e, &safe) || safe.Code != "byte_limit" {
			t.Fatal(e)
		}
		if winRecordCount() != before {
			t.Fatal("failure retained metadata handles")
		}
	}
	// Windows denies removal if a directory handle leaked without delete sharing.
	s, e := openSource(root)
	if e != nil {
		t.Fatal(e)
	}
	if e := s.close(); e != nil {
		t.Fatal(e)
	}
	if e := s.close(); e != nil {
		t.Fatal(e)
	}
	if winRecordCount() != before {
		t.Fatal("close retained metadata table")
	}
	if e := os.Rename(root, root+"-moved"); e != nil {
		t.Fatal("root handle leaked", e)
	}
	if e := os.Rename(root+"-moved", root); e != nil {
		t.Fatal(e)
	}
}
func TestWindowsReplacementBeforeAndAfterNameCheck(t *testing.T) {
	for _, after := range []bool{false, true} {
		t.Run(map[bool]string{false: "before", true: "after"}[after], func(t *testing.T) {
			root := nativeFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "original") })
			scratch := t.TempDir()
			done := false
			replace := func(p string) {
				if p != "plugin.json" || done {
					return
				}
				done = true
				if e := os.Rename(filepath.Join(root, p), filepath.Join(root, "held-original")); e != nil {
					t.Fatal(e)
				}
				nativeWrite(t, root, p, "replacement")
			}
			hooks := &captureHooks{}
			if after {
				hooks.afterNameCheck = replace
			} else {
				hooks.beforeDataOpen = replace
			}
			l, e := (Reader{TempDir: scratch}).open(context.Background(), root, hooks)
			if l != nil {
				l.Close()
			}
			var safe *Error
			if !done || !errors.As(e, &safe) || safe.Code != "source_changed" {
				t.Fatalf("replacement accepted: %v", e)
			}
			entries, e := os.ReadDir(scratch)
			if e != nil || len(entries) != 0 {
				t.Fatal("replacement failure leaked scratch", e)
			}
		})
	}
}
func TestWindowsReplacementWithPipeNamespaceLink(t *testing.T) {
	root := nativeFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "original") })
	// Probe privilege before entering the hook. This creates no pipe/server.
	nativeLink(t, root, `\\.\pipe\packageview-disposable-nonexistent`, "pipe-link")
	l, e := (Reader{TempDir: t.TempDir()}).open(context.Background(), root, &captureHooks{afterNameCheck: func(p string) {
		if p != "plugin.json" {
			return
		}
		if e := os.Rename(filepath.Join(root, p), filepath.Join(root, "old")); e != nil {
			t.Fatal(e)
		}
		if e := os.Rename(filepath.Join(root, "pipe-link"), filepath.Join(root, p)); e != nil {
			t.Fatal(e)
		}
	}})
	if l != nil {
		l.Close()
	}
	var safe *Error
	if !errors.As(e, &safe) || safe.Code != "source_changed" {
		t.Fatalf("pipe substitution: %v", e)
	}
}
func setNativeReparse(t *testing.T, path string, tag uint32, data []byte) {
	t.Helper()
	u, e := windows.UTF16PtrFromString(path)
	if e != nil {
		t.Fatal(e)
	}
	h, e := windows.CreateFile(u, windows.GENERIC_WRITE, winShare, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if e != nil {
		t.Fatal(e)
	}
	defer windows.CloseHandle(h)
	b := make([]byte, 8+len(data))
	binary.LittleEndian.PutUint32(b, tag)
	binary.LittleEndian.PutUint16(b[4:], uint16(len(data)))
	copy(b[8:], data)
	var n uint32
	e = windows.DeviceIoControl(h, windows.FSCTL_SET_REPARSE_POINT, &b[0], uint32(len(b)), nil, 0, &n, nil)
	if errors.Is(e, windows.ERROR_PRIVILEGE_NOT_HELD) || errors.Is(e, windows.ERROR_ACCESS_DENIED) {
		t.Skipf("UNPROVEN reparse fixture privilege: %v", e)
	}
	if e != nil {
		t.Fatal(e)
	}
}
func nativeJunction(t *testing.T, path, target string) {
	t.Helper()
	if e := os.Mkdir(path, 0700); e != nil {
		t.Fatal(e)
	}
	u, e := windows.UTF16FromString(`\??\` + target)
	if e != nil {
		t.Fatal(e)
	}
	// Substitute name includes a NUL terminator outside its declared length;
	// print name is empty and has its own in-buffer NUL terminator.
	// All source and target directories are new fixtures.
	data := make([]byte, 8+2*(len(u)+1))
	binary.LittleEndian.PutUint16(data[2:], uint16((len(u)-1)*2))
	binary.LittleEndian.PutUint16(data[4:], uint16(len(u)*2))
	for i, v := range u {
		binary.LittleEndian.PutUint16(data[8+2*i:], v)
	}
	setNativeReparse(t, path, windows.IO_REPARSE_TAG_MOUNT_POINT, data)
}
func TestWindowsJunctionsAndNamespaceRoots(t *testing.T) {
	root := nativeFixture(t, func(root string) {
		nativeWrite(t, root, "plugin.json", "core")
		nativeWrite(t, root, "inside/x", "inside")
	})
	outside := t.TempDir()
	nativeWrite(t, outside, "x", "outside")
	nativeJunction(t, filepath.Join(root, "contained-junction"), filepath.Join(root, "inside"))
	nativeJunction(t, filepath.Join(root, "escape-junction"), outside)
	s := nativeSource(t, root)
	for _, name := range []string{"contained-junction", "escape-junction", "escape-junction/../plugin.json"} {
		p, e := s.pin(name, false)
		if e == nil {
			p.file.Close()
			t.Fatal("junction followed", name)
		}
		if stateOf(e) != Blocked {
			t.Fatal("junction classification", e)
		}
	}
	for _, name := range []string{filepath.Join(root, "contained-junction"), `\\.\pipe\packageview-disposable-nonexistent`, `\\?\GLOBALROOT\Device\NamedPipe`, `C:relative`, `\\server\share`} {
		other, e := openSource(name)
		if e == nil {
			other.close()
			t.Fatal("namespace/reparse root accepted", name)
		}
	}
}
func TestWindowsInertFIFOReparseRejected(t *testing.T) {
	root := nativeFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "core"); nativeWrite(t, root, "fifo", "") })
	// IO_REPARSE_TAG_LX_FIFO is an inert NTFS reparse fixture, not a named-pipe
	// endpoint. Microsoft-defined tag and no payload, as used by WSL metadata.
	setNativeReparse(t, filepath.Join(root, "fifo"), 0x80000024, nil)
	s := nativeSource(t, root)
	p, e := s.pin("fifo", true)
	if e == nil {
		p.file.Close()
		t.Fatal("special reparse accepted")
	}
	if stateOf(e) != Blocked {
		t.Fatal(e)
	}
}
func TestWindowsDirectoryAncestryHeld(t *testing.T) {
	root := nativeFixture(t, func(root string) { nativeWrite(t, root, "sub/x", "data") })
	s := nativeSource(t, root)
	p, e := s.pin("sub/x", false)
	if e != nil {
		t.Fatal(e)
	}
	p.file.Close()
	if e := os.Rename(filepath.Join(root, "sub"), filepath.Join(t.TempDir(), "moved")); e == nil {
		t.Fatal("pinned ancestor moved outside root")
	}
}

func TestWindowsInventoryMutationRejected(t *testing.T) {
	root := nativeFixture(t, func(root string) {
		nativeWrite(t, root, "plugin.json", "core")
		nativeWrite(t, root, "z", "last inventory entry")
	})
	scratch := t.TempDir()
	changed := false
	l, err := (Reader{TempDir: scratch}).open(context.Background(), root, &captureHooks{afterChunk: func(path string) {
		if path == "z" && !changed {
			changed = true
			nativeWrite(t, root, "late", "added after enumeration")
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	_, err = l.Capture(context.Background())
	var safe *Error
	if !changed || !errors.As(err, &safe) || safe.Code != "source_changed" {
		t.Fatalf("inventory mutation accepted: changed=%t error=%v", changed, err)
	}
	entries, err := os.ReadDir(scratch)
	if err != nil || len(entries) != 0 {
		t.Fatal("inventory failure leaked scratch", err)
	}
}

// Ensure the hand-declared FILE_BASIC_INFO ABI remains 40 bytes on amd64.
func TestWindowsBasicInfoABI(t *testing.T) {
	if unsafe.Sizeof(winBasic{}) != 40 {
		t.Fatal("FILE_BASIC_INFO ABI mismatch")
	}
}

func TestWindowsExistingWriterAndReparseSetterDenied(t *testing.T) {
	root := nativeFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "original") })
	writer, e := os.OpenFile(filepath.Join(root, "plugin.json"), os.O_WRONLY, 0)
	if e != nil {
		t.Fatal(e)
	}
	defer writer.Close() // Release the fixture handle even if acquisition fails.
	s := nativeSource(t, root)
	p, e := s.pin("plugin.json", false)
	writer.Close()
	if e == nil {
		p.file.Close()
		t.Fatal("pin accepted preexisting writer")
	}
	p, e = s.pin("plugin.json", false)
	if e != nil {
		t.Fatal(e)
	}
	defer p.file.Close()
	u, e := windows.UTF16PtrFromString(filepath.Join(root, "plugin.json"))
	if e != nil {
		t.Fatal(e)
	}
	// FSCTL_SET_REPARSE_POINT requires a writable handle. Probe precisely that
	// access with no-follow; do not create a pipe, socket or device endpoint.
	h, e := windows.CreateFile(u, windows.GENERIC_WRITE, winShare, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if e == nil {
		windows.CloseHandle(h)
		t.Fatal("reparse setter access allowed while pin held")
	}
	if !errors.Is(e, windows.ERROR_SHARING_VIOLATION) {
		t.Fatalf("reparse setter denial was not sharing protection: %v", e)
	}
}

func TestWindowsSameObjectReopenAfterNameReplacement(t *testing.T) {
	root := nativeFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "original") })
	s := nativeSource(t, root)
	p, e := s.pin("plugin.json", false)
	if e != nil {
		t.Fatal(e)
	}
	defer p.file.Close()
	if e := os.Rename(filepath.Join(root, "plugin.json"), filepath.Join(root, "old")); e != nil {
		t.Fatal(e)
	}
	nativeWrite(t, root, "plugin.json", "replacement")
	// Isolate the native ReOpenFile contract; production additionally rejects
	// the change-time/name mismatch rather than accepting a changed capture.
	f, e := winReopen(p.file, windows.GENERIC_READ, winShare)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, 100))
	if e != nil || string(b) != "original" {
		t.Fatalf("ReOpenFile selected replacement: %q, %v", b, e)
	}
	if f, e := p.reopen(false); e == nil {
		f.Close()
		t.Fatal("production accepted changed metadata")
	}
}

func TestWindowsMetadataProbeReplacement(t *testing.T) {
	root := nativeFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "original") })
	s := nativeSource(t, root)
	f, e := winOpen(windows.Handle(s.anchor.Fd()), "plugin.json", false)
	if e != nil {
		t.Fatal(e)
	}
	before, e := f.Stat()
	if e != nil {
		f.Close()
		t.Fatal(e)
	}
	if e := os.Rename(filepath.Join(root, "plugin.json"), filepath.Join(root, "old")); e != nil {
		f.Close()
		t.Fatal(e)
	}
	nativeWrite(t, root, "plugin.json", "replacement")
	p, e := s.remember(f) // consumes f, upgrades by handle, never by replaced name
	if e != nil {
		t.Fatal(e)
	}
	defer p.file.Close()
	if !os.SameFile(before, p.info) {
		t.Fatal("metadata pin selected replacement")
	}
}

func TestWindowsRootAndIntermediateReparseRejected(t *testing.T) {
	root := nativeFixture(t, func(root string) { nativeWrite(t, root, "real/plugin.json", "core") })
	nativeLink(t, root, "real", "link")
	for _, path := range []string{root + `\link\..\real`, root + `\link\.`} {
		s, e := openSource(path)
		if e == nil {
			s.close()
			t.Fatalf("root traversal followed reparse: %s", path)
		}
	}
	before := winRecordCount()
	for i := 0; i < 10; i++ {
		s, e := openSource(root + `\missing\root`)
		if e == nil {
			s.close()
			t.Fatal("accepted missing root")
		}
		if winRecordCount() != before {
			t.Fatal("partial root acquisition leaked metadata pins")
		}
	}
}

func TestWindowsAttributesOnlyHandleCannotSetReparse(t *testing.T) {
	root := nativeFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "core") })
	s := nativeSource(t, root)
	// Retain an attributes-only handle from before the protected pin, so the
	// test checks NTFS's FSCTL authorization, not just new-handle share denial.
	f, e := winOpen(windows.Handle(s.anchor.Fd()), "plugin.json", false)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	p, e := s.pin("plugin.json", false)
	if e != nil {
		t.Fatal(e)
	}
	defer p.file.Close()
	b := make([]byte, 8)
	binary.LittleEndian.PutUint32(b, 0x80000024) // inert LX_FIFO, no endpoint
	var n uint32
	e = windows.DeviceIoControl(windows.Handle(f.Fd()), windows.FSCTL_SET_REPARSE_POINT, &b[0], uint32(len(b)), nil, 0, &n, nil)
	if !errors.Is(e, windows.ERROR_ACCESS_DENIED) {
		t.Fatalf("attributes-only FSCTL authorization gate: %v", e)
	}
	if !same(p.info, p.info) {
		t.Fatal("reparse mutation occurred despite denial")
	}
}

func TestWindowsOfflineFileHasNoDataOpen(t *testing.T) {
	root := nativeFixture(t, func(root string) { nativeWrite(t, root, "plugin.json", "core") })
	u, e := windows.UTF16PtrFromString(filepath.Join(root, "plugin.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e := windows.SetFileAttributes(u, windows.FILE_ATTRIBUTE_OFFLINE); e != nil {
		t.Fatal(e)
	}
	defer windows.SetFileAttributes(u, windows.FILE_ATTRIBUTE_NORMAL)
	l, e := (Reader{TempDir: t.TempDir()}).open(context.Background(), root, &captureHooks{beforeDataOpen: func(string) { t.Fatal("data-opened offline object") }})
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	if l.Data().Plugin.State != Blocked {
		t.Fatalf("offline file availability: %+v", l.Data().Plugin)
	}
}

func nativeLinkPrivilegeError(e error) bool {
	return os.IsPermission(e) || errors.Is(e, windows.ERROR_PRIVILEGE_NOT_HELD)
}
