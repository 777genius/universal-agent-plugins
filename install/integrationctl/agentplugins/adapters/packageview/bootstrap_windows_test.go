//go:build windows && (amd64 || arm64)

package packageview

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Raw native errors belong exclusively in test output. Keep this diagnostic in
// a _test.go file: openSource deliberately collapses several stages into safe
// public codes. Do not change that contract to diagnose a native CI failure.
func bootstrapCheck(t *testing.T, stage string, err error) {
	t.Helper()
	if err == nil {
		t.Logf("bootstrap stage=%s OK", stage)
		return
	}
	var status windows.NTStatus
	var errno syscall.Errno
	switch {
	case errors.As(err, &status):
		t.Fatalf("bootstrap stage=%s NTSTATUS=0x%08x raw=%T: %v", stage, uint32(status), err, err)
	case errors.As(err, &errno):
		t.Fatalf("bootstrap stage=%s Win32=%d (0x%08x) raw=%T: %v", stage, uint32(errno), uint32(errno), err, err)
	default:
		t.Fatalf("bootstrap stage=%s raw=%T: %v", stage, err, err)
	}
}

// Split the two queries hidden inside winMeta so that a failure identifies the
// actual API. These are handle metadata queries, never source data opens/writes.
func bootstrapMetadata(t *testing.T, stage string, f *os.File) {
	t.Helper()
	var info windows.ByHandleFileInformation
	bootstrapCheck(t, stage+"/GetFileInformationByHandle", windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &info))
	var basic winBasic
	bootstrapCheck(t, stage+"/GetFileInformationByHandleEx(FileBasicInfo)", windows.GetFileInformationByHandleEx(windows.Handle(f.Fd()), windows.FileBasicInfo, (*byte)(unsafe.Pointer(&basic)), uint32(unsafe.Sizeof(basic))))
	kind, err := windows.GetFileType(windows.Handle(f.Fd()))
	bootstrapCheck(t, stage+"/GetFileType", err)
	t.Logf("bootstrap stage=%s attributes=0x%08x type=%d links=%d", stage, info.FileAttributes, kind, info.NumberOfLinks)
	if kind != windows.FILE_TYPE_DISK || info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		t.Fatalf("bootstrap stage=%s expected an ordinary disk directory", stage)
	}
	_, err = f.Stat()
	bootstrapCheck(t, stage+"/File.Stat", err)
	// Exercise the exact directory upgrade separately to identify ReOpenFile
	// errors that remember otherwise returns without an API label. Type has
	// already been proven; no regular/special object receives listing access.
	if info.FileAttributes&(windows.FILE_ATTRIBUTE_OFFLINE|windows.FILE_ATTRIBUTE_RECALL_ON_OPEN|windows.FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS|windows.FILE_ATTRIBUTE_DEVICE|windows.FILE_ATTRIBUTE_ENCRYPTED) != 0 {
		t.Fatalf("bootstrap stage=%s unsupported directory attributes", stage)
	}
	held, err := winReopen(f, windows.FILE_READ_ATTRIBUTES|windows.FILE_LIST_DIRECTORY, windows.FILE_SHARE_READ)
	bootstrapCheck(t, stage+"/ReOpenFile(directory-pin)", err)
	defer held.Close()
	_, err = winMeta(held)
	bootstrapCheck(t, stage+"/winMeta(reopened-directory)", err)
	dup, err := winDup(held)
	bootstrapCheck(t, stage+"/DuplicateHandle(directory-pin)", err)
	bootstrapCheck(t, stage+"/close-diagnostic-duplicate", dup.Close())
}

func TestWindowsBootstrapStages(t *testing.T) {
	// An ordinary absolute root on the CI temp drive (D: in the failing job).
	// No symlink privileges, second volume, service or system policy is needed.
	root := filepath.Join(t.TempDir(), "absolute source Ω", "nested")
	nativeWrite(t, root, "plugin.json", `{"name":"bootstrap-fixture"}`)
	if !filepath.IsAbs(root) || len(filepath.VolumeName(root)) != 2 {
		t.Fatalf("expected a disposable absolute drive fixture: %q", root)
	}
	t.Logf("bootstrap fixture=%q", root)

	t.Run("native-stages", func(t *testing.T) {
		before := winRecordCount()
		s := &source{records: make(map[winSnapshot]*winObservation)}
		defer func() {
			if err := s.close(); err != nil {
				t.Error(err)
			}
			if winRecordCount() != before {
				t.Error("bootstrap failure/success retained metadata records")
			}
		}()
		drive := root[:3]
		u, err := windows.UTF16PtrFromString(drive)
		bootstrapCheck(t, "drive/UTF16", err)
		kind := windows.GetDriveType(u)
		t.Logf("bootstrap stage=GetDriveType drive=%q type=%d", drive, kind)
		if kind != windows.DRIVE_FIXED {
			t.Fatal("bootstrap fixture requires a fixed drive; native success unproven")
		}
		// Use the production NtCreateFile wrapper, including its exact access,
		// sharing and no-follow/no-recall options. No alternative open on failure.
		f, err := winOpen(0, `\??\`+drive, true)
		bootstrapCheck(t, "drive/NtCreateFile", err)
		s.anchor = f
		var fs [32]uint16
		bootstrapCheck(t, "drive/GetVolumeInformationByHandle", windows.GetVolumeInformationByHandle(windows.Handle(f.Fd()), nil, 0, &s.volume, nil, nil, &fs[0], uint32(len(fs))))
		t.Logf("bootstrap filesystem=%q", windows.UTF16ToString(fs[:]))
		if windows.UTF16ToString(fs[:]) != "NTFS" {
			t.Fatal("bootstrap fixture requires NTFS; native success unproven")
		}
		bootstrapMetadata(t, "drive", f)
		p, err := s.rememberMustDuplicate(f, true)
		bootstrapCheck(t, "drive/remember(DuplicateHandle,ReOpenFile,metadata)", err)
		old := s.anchor
		s.anchor = p.file
		bootstrapCheck(t, "drive/close-probe", old.Close())

		// Walk the fresh absolute spelling one component at a time using the same
		// production helpers. This locates the failing ancestor/API before walk
		// and openSource erase its raw error. No directory is enumerated here.
		for i, part := range strings.Split(root[3:], `\`) {
			stage := fmt.Sprintf("component[%d]=%q", i, part)
			f, err := winOpen(windows.Handle(s.anchor.Fd()), part, false)
			bootstrapCheck(t, stage+"/NtCreateFile(relative)", err)
			func() {
				defer f.Close()
				bootstrapMetadata(t, stage, f)
				p, err = s.rememberMustDuplicate(f, true)
				bootstrapCheck(t, stage+"/remember(DuplicateHandle,ReOpenFile,metadata)", err)
			}()
			old = s.anchor
			s.anchor = p.file
			bootstrapCheck(t, stage+"/close-parent-duplicate", old.Close())
		}
		// Identity is another root_unreadable boundary in Reader.Open. Expose the
		// raw query before exercising the production GUID/path validation.
		buf := make([]uint16, 32768)
		n, err := windows.GetFinalPathNameByHandle(windows.Handle(s.anchor.Fd()), &buf[0], uint32(len(buf)), 1)
		bootstrapCheck(t, "root/GetFinalPathNameByHandle(VOLUME_NAME_GUID)", err)
		if n == 0 || n >= uint32(len(buf)) {
			t.Fatalf("bootstrap identity length=%d", n)
		}
		t.Logf("bootstrap normalized identity=%q", windows.UTF16ToString(buf[:n]))
		_, _, err = winDirectoryIdentity(s.anchor)
		bootstrapCheck(t, "root/winDirectoryIdentity", err)
	})

	// Run independently even if the staged probe failed. A staged success is
	// never a substitute for the real absolute-root walk and scratch separation.
	t.Run("production-capture", func(t *testing.T) {
		scratch := t.TempDir()
		before := winRecordCount()
		defer func() {
			if winRecordCount() != before {
				t.Error("production capture retained metadata records")
			}
			entries, err := os.ReadDir(scratch)
			if err != nil || len(entries) != 0 {
				t.Errorf("production capture leaked scratch: entries=%d err=%v", len(entries), err)
			}
		}()
		l, err := (Reader{TempDir: scratch}).Open(context.Background(), root)
		bootstrapCheck(t, "Reader.Open (public sanitized error)", err)
		defer l.Close()
		in, err := l.Capture(context.Background())
		bootstrapCheck(t, "Lease.Capture (public sanitized error)", err)
		if in.Plugin.State != Present || !in.Coverage.InventoryComplete || !in.Coverage.TreeComplete {
			t.Fatalf("bootstrap capture incomplete: plugin=%s coverage=%+v", in.Plugin.State, in.Coverage)
		}
		bootstrapCheck(t, "Lease.Close", l.Close())
	})
}
