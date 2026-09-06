//go:build windows && amd64

package packageview

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

// The existing replacement, reparse, hardlink, ancestry and failure-cleanup
// contracts run unchanged. These cases add byte/list reads and exact denial
// mapping at the NT seam, which the parameter-only native matrix did not prove.
func TestWindowsNTSelfOpenAccessAndRead(t *testing.T) {
	for _, directory := range []bool{false, true} {
		t.Run(map[bool]string{false: "regular", true: "directory"}[directory], func(t *testing.T) {
			before := winRecordCount()
			root := t.TempDir()
			name := "candidate"
			if directory {
				name += "/child"
			}
			nativeWrite(t, root, name, "original bytes")
			s := nativeSource(t, root)
			probe, err := winOpen(windows.Handle(s.anchor.Fd()), "candidate", false)
			bootstrapCheck(t, "initial metadata", err)
			defer probe.Close()
			directoryGuardMetadataAccess(t, probe)
			meta := reopenDiagnosticKind(t, probe, directory)
			if !directory {
				var b [1]byte
				var n uint32
				err := windows.ReadFile(windows.Handle(probe.Fd()), b[:], &n, nil)
				if n != 0 || !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
					t.Fatalf("metadata probe read: n=%d err=%v", n, err)
				}
			}
			pin, err := s.rememberMustDuplicate(probe)
			bootstrapCheck(t, "protected pin", err)
			defer pin.file.Close()
			access := uint32(windows.FILE_READ_ATTRIBUTES | windows.DELETE)
			if directory {
				access = windows.FILE_READ_ATTRIBUTES | windows.FILE_LIST_DIRECTORY
			}
			reopenDiagnosticAccess(t, pin.file, access)
			if got := reopenDiagnosticKind(t, pin.file, directory); got != meta {
				t.Fatal("protected pin changed identity/metadata")
			}
			data, err := pin.reopen(directory)
			bootstrapCheck(t, "verified data reopen", err)
			defer data.Close()
			reopenDiagnosticAccess(t, data, windows.FILE_GENERIC_READ&^windows.SYNCHRONIZE)
			if got := reopenDiagnosticKind(t, data, directory); got != meta {
				t.Fatal("data reopen changed identity/metadata")
			}
			if directory {
				entries, err := data.ReadDir(2)
				if err != nil || len(entries) != 1 || entries[0].Name() != "child" {
					t.Fatalf("bounded directory read: %v, %v", entries, err)
				}
			} else {
				b, err := io.ReadAll(io.LimitReader(data, 100))
				if err != nil || string(b) != "original bytes" {
					t.Fatalf("bounded data read: %q, %v", b, err)
				}
			}
			bootstrapCheck(t, "data close", data.Close())
			bootstrapCheck(t, "pin duplicate close", pin.file.Close())
			bootstrapCheck(t, "probe close", probe.Close())
			bootstrapCheck(t, "source close", s.close())
			if winRecordCount() != before {
				t.Fatal("successful reads retained metadata records")
			}
			// An exclusive read open must be possible after every owned pin closes.
			// This detects leaked protected handles independently of the side table.
			h := ntSelfFixtureHandle(t, filepath.Join(root, "candidate"), windows.GENERIC_READ, 0)
			bootstrapCheck(t, "exclusive handle close", windows.CloseHandle(h))
		})
	}
}

func ntSelfFixtureHandle(t *testing.T, name string, access, share uint32) windows.Handle {
	// Only fresh fixture setup/verification uses a pathname; acquisition under
	// test uses winOpen metadata followed by winReopen relative to that handle.
	t.Helper()
	u, err := windows.UTF16PtrFromString(name)
	bootstrapCheck(t, "fixture UTF16", err)
	h, err := windows.CreateFile(u, access, share, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	bootstrapCheck(t, "fixture CreateFile", err)
	return h
}

func TestWindowsNTSelfOpenDirectoryAfterNameReplacement(t *testing.T) {
	root := t.TempDir()
	nativeWrite(t, root, "candidate/original-child", "original")
	s := nativeSource(t, root)
	probe, err := winOpen(windows.Handle(s.anchor.Fd()), "candidate", false)
	bootstrapCheck(t, "directory metadata", err)
	defer probe.Close()
	directoryGuardMetadataAccess(t, probe)
	original, err := probe.Stat()
	bootstrapCheck(t, "original identity", err)
	bootstrapCheck(t, "replace directory name", os.Rename(filepath.Join(root, "candidate"), filepath.Join(root, "moved")))
	nativeWrite(t, root, "candidate/replacement-child", "replacement")
	// Directory rename is possible before acquiring the ancestry-protecting pin.
	// Prove the first protected self-open and subsequent list still select the
	// held directory. Existing file replacement tests cover both read boundaries
	// and the production requirement to reject changed capture metadata.
	pin, err := s.rememberMustDuplicate(probe)
	bootstrapCheck(t, "protect renamed directory", err)
	defer pin.file.Close()
	if !os.SameFile(original, pin.info) {
		t.Fatal("protected pin selected replacement directory")
	}
	data, err := pin.reopen(true)
	bootstrapCheck(t, "reopen renamed directory for list", err)
	defer data.Close()
	info, err := data.Stat()
	bootstrapCheck(t, "list handle identity", err)
	if !os.SameFile(original, info) {
		t.Fatal("list handle selected replacement directory")
	}
	entries, err := data.ReadDir(2)
	if err != nil || len(entries) != 1 || entries[0].Name() != "original-child" {
		t.Fatalf("renamed directory list selected replacement: %v, %v", entries, err)
	}
}

func TestWindowsNTSelfOpenExistingConflictsAndCleanup(t *testing.T) {
	for _, directory := range []bool{false, true} {
		for _, setter := range []bool{false, true} {
			label := map[bool]string{false: "regular", true: "directory"}[directory] + "/" +
				map[bool]string{false: "writer", true: "reparse-setter-access"}[setter]
			t.Run(label, func(t *testing.T) {
				root := t.TempDir()
				name := "candidate"
				if directory {
					name += "/child"
				}
				nativeWrite(t, root, name, "inert fixture")
				s := nativeSource(t, root)
				before := winRecordCount()
				access := uint32(windows.FILE_WRITE_DATA)
				if setter {
					// Same writable, no-follow access used by FSCTL_SET_REPARSE_POINT
					// fixtures. No special endpoint or privilege-dependent FSCTL here.
					access = windows.GENERIC_WRITE
				}
				h := ntSelfFixtureHandle(t, filepath.Join(root, "candidate"), access, winShare)
				defer func() {
					if h != windows.InvalidHandle {
						windows.CloseHandle(h)
					}
				}()
				for i := 0; i < 8; i++ {
					probe, err := winOpen(windows.Handle(s.anchor.Fd()), "candidate", false)
					bootstrapCheck(t, "metadata despite existing writer", err)
					defer probe.Close()
					directoryGuardMetadataAccess(t, probe)
					// remember owns/consumes probe even on failed protection. Require
					// NTSTATUS conversion to the precise existing Win32 contract.
					pin, err := s.remember(probe)
					if pin != nil {
						pin.file.Close()
						t.Fatal("protected pin accepted incompatible existing handle")
					}
					if !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
						t.Fatalf("expected Win32 sharing violation, got %T: %v", err, err)
					}
					if _, err := probe.Stat(); !errors.Is(err, os.ErrClosed) {
						t.Fatalf("failure did not close consumed metadata probe: %v", err)
					}
					if winRecordCount() != before {
						t.Fatal("failed protection retained metadata records")
					}
				}
				bootstrapCheck(t, "conflicting handle close", windows.CloseHandle(h))
				h = windows.InvalidHandle
				pin, err := s.pin("candidate", false)
				bootstrapCheck(t, "pin after writer released", err)
				bootstrapCheck(t, "pin close", pin.file.Close())
				bootstrapCheck(t, "source close", s.close())
				h = ntSelfFixtureHandle(t, filepath.Join(root, "candidate"), windows.GENERIC_READ, 0)
			})
		}
	}
}

func TestWindowsNTSelfOpenInvalidHandles(t *testing.T) {
	root := t.TempDir()
	nativeWrite(t, root, "candidate", "inert")
	closed, err := winOpen(0, `\??\`+filepath.Join(root, "candidate"), false)
	bootstrapCheck(t, "closed fixture open", err)
	bootstrapCheck(t, "closed fixture close", closed.Close())
	zero := os.NewFile(0, "null-handle")
	if zero != nil {
		defer zero.Close()
	}
	for _, test := range []struct {
		name string
		file *os.File
	}{
		{"nil", nil},
		{"null", zero},
		{"invalid", os.NewFile(uintptr(windows.InvalidHandle), "invalid-handle")},
		{"closed", closed},
	} {
		t.Run(test.name, func(t *testing.T) {
			out, err := winReopen(test.file, windows.FILE_READ_ATTRIBUTES|windows.DELETE, windows.FILE_SHARE_READ)
			if out != nil {
				out.Close()
				t.Fatal("invalid input returned a handle")
			}
			if !errors.Is(err, windows.ERROR_INVALID_HANDLE) {
				t.Fatalf("invalid input: %T: %v", err, err)
			}
		})
	}
}
