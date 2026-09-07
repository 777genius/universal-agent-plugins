//go:build windows && amd64

package packageview

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// These diagnostics exercise the actual acquisition comparison on fresh NTFS
// fixtures. A passing test proves the current rejection, not a corrected policy.
func TestWindowsAcquisitionMutableMetadataDiagnostic(t *testing.T) {
	for _, directory := range []bool{true, false} {
		label := "regular-file-control"
		if directory {
			label = "directory-child-create"
		}
		t.Run(label, func(t *testing.T) {
			root := t.TempDir()
			name := filepath.Join(root, "subject")
			if directory {
				if err := os.Mkdir(name, 0700); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(name, []byte("before"), 0600); err != nil {
				t.Fatal(err)
			}
			// Give the directory a clearly distinct starting timestamp without a sleep.
			// Mutation during remember is still an actual child create and close.
			acquisitionDiagnosticOldWrite(t, name)
			f, err := winOpen(0, `\??\`+name, directory)
			if err != nil {
				t.Fatalf("direct fixture open: %v", err)
			}
			defer f.Close()
			meta, err := winMeta(f)
			if err != nil {
				t.Fatal(err)
			}
			var fs [32]uint16
			if err := windows.GetVolumeInformationByHandle(windows.Handle(f.Fd()), nil, 0, nil, nil, nil, &fs[0], uint32(len(fs))); err != nil {
				t.Fatal(err)
			}
			if windows.UTF16ToString(fs[:]) != "NTFS" {
				t.Fatal("diagnostic requires NTFS")
			}
			drive, err := windows.UTF16PtrFromString(filepath.VolumeName(name) + `\`)
			if err != nil {
				t.Fatal(err)
			}
			if windows.GetDriveType(drive) != windows.DRIVE_FIXED {
				t.Fatal("diagnostic requires local fixed drive")
			}
			s := &source{volume: meta.Volume, records: make(map[winSnapshot]*winObservation)}
			defer s.close()
			mutated, observed := false, false
			var before, after winSnapshot
			s.acquisitionHook = func(stage string, initial, current winSnapshot, queryErr error) {
				switch stage {
				case "before-after-stat":
					if mutated {
						t.Fatal("mutation hook invoked more than once")
					}
					mutated = true
					target := name
					if directory {
						target = filepath.Join(name, "new-child")
					}
					if err := os.WriteFile(target, []byte("actual fixture mutation, larger than before"), 0600); err != nil {
						t.Fatal(err)
					}
				case "after-stat":
					observed = true
					before, after = initial, current
					t.Logf("stage=%s before=%+v after=%+v query_error=%v", stage, before, after, queryErr)
					if queryErr != nil {
						t.Fatalf("metadata query failed instead of comparison: %v", queryErr)
					}
				default:
					t.Fatalf("unexpected acquisition stage %q", stage)
				}
			}
			pin, err := s.remember(f)
			if pin != nil {
				pin.file.Close()
				t.Fatal("mutated fixture unexpectedly acquired")
			}
			t.Logf("remember returned: %v", err)
			var safe *Error
			if !errors.As(err, &safe) || safe.Code != "source_changed" {
				t.Fatalf("want source_changed, got %v", err)
			}
			if !mutated || !observed {
				t.Fatalf("missing real comparison: mutated=%t observed=%t", mutated, observed)
			}
			if before == after {
				t.Fatal("fixture mutation did not change observed metadata")
			}
			// Identity/type/link facts remain stable. Only size/write/change may differ.
			stable := after
			stable.Size, stable.Write, stable.Change = before.Size, before.Write, before.Change
			if stable != before {
				t.Fatalf("mutation changed immutable facts: before=%+v after=%+v", before, after)
			}
			if directory && before.Write == after.Write && before.Change == after.Change {
				t.Fatal("child creation did not change directory timestamps")
			}
			if !directory && before.Size == after.Size {
				t.Fatal("regular-file mutation did not change size")
			}
			if len(s.records) != 0 {
				t.Fatal("rejected acquisition retained a record")
			}
		})
	}
}

func acquisitionDiagnosticOldWrite(t *testing.T, name string) {
	t.Helper()
	path, err := windows.UTF16PtrFromString(name)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(path, windows.FILE_WRITE_ATTRIBUTES, winShare, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		t.Fatal(err)
	}
	old := windows.NsecToFiletime(time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano())
	setErr := windows.SetFileTime(h, nil, nil, &old)
	closeErr := windows.CloseHandle(h)
	if setErr != nil {
		t.Fatal(setErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
}
