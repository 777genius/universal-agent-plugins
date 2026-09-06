//go:build windows && amd64

package packageview

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Inspect granted access without attempting any source data or timestamp write.
func directoryGuardMetadataAccess(t *testing.T, f *os.File) {
	t.Helper()
	defer runtime.KeepAlive(f)
	var access uint32
	var status windows.IO_STATUS_BLOCK
	query := windows.NewLazySystemDLL("ntdll.dll").NewProc("NtQueryInformationFile")
	const fileAccessInformation = 8
	r, _, _ := query.Call(f.Fd(), uintptr(unsafe.Pointer(&status)), uintptr(unsafe.Pointer(&access)), unsafe.Sizeof(access), fileAccessInformation)
	if windows.NTStatus(r) != 0 {
		t.Fatalf("query granted access: NTSTATUS=%#x", r)
	}
	if want := uint32(windows.FILE_READ_ATTRIBUTES | windows.SYNCHRONIZE); access != want {
		t.Fatalf("metadata probe granted access=%#x, want %#x (no list/data/write access)", access, want)
	}
}

func TestWindowsBootstrapDirectoryGuard(t *testing.T) {
	root := t.TempDir()
	before := winRecordCount()
	// Exercise the literal drive bootstrap, not just a relative child open.
	drive := filepath.VolumeName(root) + `\`
	f, err := winOpen(0, `\??\`+drive, true)
	if err != nil {
		t.Fatalf("ordinary drive bootstrap: %v", err)
	}
	defer f.Close()
	directoryGuardMetadataAccess(t, f)
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	// Production must proceed through NTFS inventory and protected root pins.
	s, err := openSource(root)
	if err != nil {
		t.Fatalf("production bootstrap: %v", err)
	}
	defer s.close()
	if err := winCheckDirectory(s.anchor); err != nil {
		t.Fatalf("promoted root: %v", err)
	}
	if len(s.records) == 0 {
		t.Fatal("root promoted without protected metadata pins")
	}
	if err := s.close(); err != nil {
		t.Fatal(err)
	}
	if winRecordCount() != before {
		t.Fatal("bootstrap close retained metadata records")
	}
}

func TestWindowsBootstrapDirectoryGuardWrongKind(t *testing.T) {
	name := filepath.Join(t.TempDir(), "ordinary-file")
	if err := os.WriteFile(name, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	// The same metadata acquisition must still accept ordinary source files.
	f, err := winOpen(0, `\??\`+name, false)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	directoryGuardMetadataAccess(t, f)
	if err := winCheckDirectory(f); !errors.Is(err, syscall.ENOTDIR) {
		t.Fatalf("regular file directory guard: %v", err)
	}
	f.Close()
	f, err = winOpen(0, `\??\`+name, true)
	if f != nil {
		f.Close()
		t.Fatal("wrong-kind bootstrap returned a handle")
	}
	if !errors.Is(err, syscall.ENOTDIR) {
		t.Fatalf("wrong-kind bootstrap: %v", err)
	}
}

func TestWindowsBootstrapDirectoryGuardUnsafe(t *testing.T) {
	for _, fixture := range []string{"junction", "offline-directory"} {
		t.Run(fixture, func(t *testing.T) {
			root := t.TempDir()
			name := filepath.Join(root, "candidate")
			if fixture == "junction" {
				nativeJunction(t, name, t.TempDir())
			} else {
				if err := os.Mkdir(name, 0700); err != nil {
					t.Fatal(err)
				}
				u, err := windows.UTF16PtrFromString(name)
				if err != nil {
					t.Fatal(err)
				}
				if err := windows.SetFileAttributes(u, windows.FILE_ATTRIBUTE_OFFLINE); err != nil {
					t.Fatal(err)
				}
				defer windows.SetFileAttributes(u, windows.FILE_ATTRIBUTE_NORMAL)
			}
			// Verify fixture metadata on the non-following attributes-only handle.
			f, err := winOpen(0, `\??\`+name, false)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			directoryGuardMetadataAccess(t, f)
			if err := winCheckDirectory(f); !errors.Is(err, syscall.EXDEV) {
				t.Fatalf("unsafe directory metadata accepted: %v", err)
			}
			f.Close()
			f, err = winOpen(0, `\??\`+name, true)
			if f != nil {
				f.Close()
				t.Fatal("unsafe bootstrap returned a handle")
			}
			if !errors.Is(err, syscall.EXDEV) {
				t.Fatalf("unsafe directory bootstrap: %v", err)
			}
		})
	}
}
