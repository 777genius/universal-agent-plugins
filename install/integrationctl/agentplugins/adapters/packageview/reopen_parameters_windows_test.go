//go:build windows && amd64

package packageview

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

const reopenDiagnosticFlags = windows.FILE_FLAG_OPEN_REPARSE_POINT | windows.FILE_FLAG_OPEN_NO_RECALL | windows.FILE_FLAG_BACKUP_SEMANTICS

// Test-only parameter evidence, not a fallback or an alternative read profile.
// Every initial open is attributes-only. Every upgrade uses only its HANDLE;
// directory listing access follows same-handle type/attribute checks, and data
// access follows a protected pin. No variant removes reparse/no-recall flags or
// relaxes sharing. No call reads bytes, lists entries, or writes timestamps.
func TestWindowsReopenProtectedParameters(t *testing.T) {
	root := t.TempDir()
	name := filepath.Join(root, "regular Ω")
	if err := os.WriteFile(name, []byte("disposable reopen fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	drive := filepath.VolumeName(root) + `\`
	u, err := windows.UTF16PtrFromString(drive)
	bootstrapCheck(t, "reopen/drive-UTF16", err)
	if windows.GetDriveType(u) != windows.DRIVE_FIXED {
		t.Fatal("reopen diagnostic requires the fixture's local fixed drive")
	}
	if reopenDiagnosticFlags != 0x02300000 || winShare != 7 {
		t.Fatal("Win32 flag/share domains changed")
	}
	bootstrapCheck(t, "reopen/ReOpenFile-export", winReOpen.Find())
	for _, object := range []struct {
		label, name string
		directory   bool
	}{
		{"drive", drive, true},
		{"directory", root, true},
		{"regular", name, false},
	} {
		t.Run(object.label, func(t *testing.T) {
			// The CreateFileW origin is an independent attributes-only control on
			// the literal drive/fresh fixture, never a source-path retry after
			// a failed upgrade. Each case owns a new initial handle.
			for _, origin := range []string{"NtCreateFile", "CreateFileW"} {
				t.Run(origin, func(t *testing.T) {
					for _, method := range []string{"production", "SyscallN", "explicit-synchronize", "overlapped", "nt-empty-name"} {
						t.Run(method, func(t *testing.T) {
							var f *os.File
							var err error
							if origin == "NtCreateFile" {
								f, err = winOpen(0, `\??\`+object.name, object.directory)
							} else {
								path, e := windows.UTF16PtrFromString(object.name)
								bootstrapCheck(t, "reopen/CreateFileW-UTF16", e)
								var h windows.Handle
								h, err = windows.CreateFile(path, windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE, winShare, nil, windows.OPEN_EXISTING, reopenDiagnosticFlags, 0)
								if err == nil {
									f = os.NewFile(uintptr(h), "reopen-initial-metadata")
								}
							}
							bootstrapCheck(t, "reopen/initial-attributes-only", err)
							defer reopenDiagnosticClose(t, f)
							directoryGuardMetadataAccess(t, f)
							meta := reopenDiagnosticKind(t, f, object.directory)
							var fs [32]uint16
							bootstrapCheck(t, "reopen/volume", windows.GetVolumeInformationByHandle(windows.Handle(f.Fd()), nil, 0, nil, nil, nil, &fs[0], uint32(len(fs))))
							if windows.UTF16ToString(fs[:]) != "NTFS" {
								t.Fatal("reopen diagnostic requires NTFS")
							}
							access := uint32(windows.FILE_READ_ATTRIBUTES | windows.DELETE)
							share := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_DELETE)
							if object.directory {
								access = windows.FILE_READ_ATTRIBUTES | windows.FILE_LIST_DIRECTORY
								share = windows.FILE_SHARE_READ
							}
							pin, err := reopenDiagnosticCall(t, f, method, "protected-pin", access, share)
							if err != nil {
								// Require the real seam separately; alternate failures are
								// observations and cannot terminate the remaining matrix.
								if origin == "NtCreateFile" && method == "production" {
									t.Errorf("production protected pin remains unavailable: %v", err)
								}
								return
							}
							defer reopenDiagnosticClose(t, pin)
							if got := reopenDiagnosticKind(t, pin, object.directory); got != meta {
								t.Fatal("protected pin changed object identity or metadata")
							}
							// Confirm regular pins did not accidentally acquire data,
							// and proven-directory pins acquired list but not DELETE.
							reopenDiagnosticAccess(t, pin, access)
							data, err := reopenDiagnosticCall(t, pin, method, "protected-to-read", windows.GENERIC_READ, winShare)
							if err != nil {
								if origin == "NtCreateFile" && method == "production" {
									t.Errorf("production data reopen remains unavailable: %v", err)
								}
								return
							}
							defer reopenDiagnosticClose(t, data)
							if got := reopenDiagnosticKind(t, data, object.directory); got != meta {
								t.Fatal("read reopen changed object identity or metadata")
							}
							t.Log("same-object metadata verified; no byte-read or enumeration claim")
						})
					}
				})
			}
		})
	}
}

func reopenDiagnosticKind(t *testing.T, f *os.File, directory bool) winSnapshot {
	t.Helper()
	kind, err := windows.GetFileType(windows.Handle(f.Fd()))
	bootstrapCheck(t, "reopen/GetFileType", err)
	meta, err := winMeta(f)
	bootstrapCheck(t, "reopen/winMeta", err)
	info, err := f.Stat()
	bootstrapCheck(t, "reopen/File.Stat", err)
	if kind != windows.FILE_TYPE_DISK || meta.Attributes&(winUnsafeAttributes|windows.FILE_ATTRIBUTE_REPARSE_POINT) != 0 ||
		(meta.Attributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0) != directory || info.IsDir() != directory || (!directory && !info.Mode().IsRegular()) {
		t.Fatal("no upgrade allowed for unsafe or unexpected object type")
	}
	if directory {
		bootstrapCheck(t, "reopen/same-handle-directory-guard", winCheckDirectory(f))
	}
	return meta
}

func reopenDiagnosticCall(t *testing.T, f *os.File, method, stage string, access, share uint32) (*os.File, error) {
	t.Helper()
	defer runtime.KeepAlive(f)
	flags := uint32(reopenDiagnosticFlags)
	if method == "explicit-synchronize" || method == "nt-empty-name" {
		access |= windows.SYNCHRONIZE
	}
	if method == "overlapped" {
		flags |= windows.FILE_FLAG_OVERLAPPED
	}
	t.Logf("stage=%s method=%s access=%#08x share=%#08x Win32-flags=%#08x", stage, method, access, share, flags)
	var out *os.File
	var err error
	switch method {
	case "production":
		out, err = winReopen(f, access, share)
	case "nt-empty-name":
		// Experimental native parameter probe ONLY: empty ObjectName rooted at
		// the original HANDLE; no pathname, file ID lookup, or directory flag.
		// Success/equal metadata does not establish a production API contract.
		u, e := windows.NewNTUnicodeString("")
		bootstrapCheck(t, "reopen/empty-NTUnicodeString", e)
		oa := windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(f.Fd()), ObjectName: u}
		oa.Length = uint32(unsafe.Sizeof(oa))
		const options = windows.FILE_OPEN_REPARSE_POINT | windows.FILE_OPEN_NO_RECALL | windows.FILE_OPEN_FOR_BACKUP_INTENT | windows.FILE_SYNCHRONOUS_IO_NONALERT
		t.Logf("NT root=original-handle name-length=%d access=%#08x share=%#08x options=%#08x disposition=FILE_OPEN attributes=0", u.Length, access, share, options)
		var h windows.Handle
		var iosb windows.IO_STATUS_BLOCK
		err = windows.NtCreateFile(&h, access, &oa, &iosb, nil, 0, share, windows.FILE_OPEN, options, 0, 0)
		runtime.KeepAlive(u)
		if err == nil {
			out = os.NewFile(uintptr(h), "reopen-native-diagnostic")
			t.Logf("result NTSTATUS=0 iosb.status=%#08x information=%d", uint32(iosb.Status), iosb.Information)
		} else {
			t.Logf("result NTSTATUS=%#08x", uint32(err.(windows.NTStatus)))
		}
	default:
		// Independent SyscallN also checks the production LazyProc.Call binding.
		h, _, e := syscall.SyscallN(winReOpen.Addr(), f.Fd(), uintptr(access), uintptr(share), uintptr(flags))
		if windows.Handle(h) == windows.InvalidHandle {
			err = e
		} else {
			out = os.NewFile(h, "reopen-Win32-diagnostic")
		}
	}
	if err != nil {
		t.Logf("stage=%s result raw=%T: %v", stage, err, err)
		if e, ok := err.(syscall.Errno); ok {
			t.Logf("result Win32=%d (%#08x)", uint32(e), uint32(e))
		}
	} else {
		t.Logf("stage=%s result SUCCESS", stage)
	}
	return out, err
}

func reopenDiagnosticAccess(t *testing.T, f *os.File, want uint32) {
	t.Helper()
	defer runtime.KeepAlive(f)
	var access uint32
	var iosb windows.IO_STATUS_BLOCK
	query := windows.NewLazySystemDLL("ntdll.dll").NewProc("NtQueryInformationFile")
	r, _, _ := query.Call(f.Fd(), uintptr(unsafe.Pointer(&iosb)), uintptr(unsafe.Pointer(&access)), unsafe.Sizeof(access), 8 /* FileAccessInformation */)
	if windows.NTStatus(r) != 0 {
		t.Fatalf("protected access query NTSTATUS=%#08x", uint32(r))
	}
	t.Logf("protected granted-access=%#08x", access)
	if access&^uint32(windows.SYNCHRONIZE) != want {
		t.Fatalf("protected granted-access=%#08x, want %#08x with optional SYNCHRONIZE", access, want)
	}
}

func reopenDiagnosticClose(t *testing.T, f *os.File) {
	t.Helper()
	if err := f.Close(); err != nil {
		t.Errorf("diagnostic handle close: %v", err)
	}
}
