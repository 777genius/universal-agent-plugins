package transaction

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectRootMovedPreparedStageRetainsOwnedMarker(t *testing.T) {
	_, _, spec, _ := projectRootFixture(t)
	prepared, err := prepareNativeRoot(t.Context(), spec)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := prepared.close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()
	marker, err := filepath.Rel(prepared.staging, filepath.Join(prepared.selected.path, prepared.markerName))
	if err != nil {
		t.Fatal(err)
	}
	if err := prepared.sealForPublication(); err != nil {
		t.Fatal(err)
	}
	moved := prepared.staging + "-moved"
	if err := os.Rename(prepared.staging, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(prepared.staging, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := prepared.cleanup(); err == nil {
		t.Fatal("substituted staging cleanup was accepted")
	}
	body, err := os.ReadFile(filepath.Join(moved, marker))
	if err != nil || string(body) != string(prepared.markerBody) {
		t.Fatalf("refused cleanup changed moved original marker: %q %v", body, err)
	}
}

func TestProjectRootCleanupRetainsForeignMarkerAndReleasesHandles(t *testing.T) {
	_, _, spec, _ := projectRootFixture(t)
	prepared, err := prepareNativeRoot(t.Context(), spec)
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(prepared.selected.path, prepared.markerName)
	if err := prepared.closeMarkerHandle(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(marker, marker+"-original"); err != nil {
		t.Fatal(err)
	}
	foreign := []byte("foreign replacement must survive cleanup")
	if err := os.WriteFile(marker, foreign, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := prepared.cleanup(); err == nil {
		t.Fatal("cleanup did not report retained staging")
	}
	body, err := os.ReadFile(marker)
	if err != nil || string(body) != string(foreign) {
		t.Fatalf("cleanup changed replacement: %q %v", body, err)
	}
	body, err = os.ReadFile(marker + "-original")
	if err != nil || string(body) != string(prepared.markerBody) {
		t.Fatalf("cleanup changed original: %q %v", body, err)
	}
	for _, held := range []*heldNativeRoot{prepared.baseRoot, prepared.staged, prepared.selected} {
		if held.root != nil {
			t.Fatal("retention kept an open directory")
		}
	}
	if prepared.markerFile != nil {
		t.Fatal("retention kept the writer open")
	}
	if err := prepared.close(); err != nil {
		t.Fatal("retention close was not idempotent", err)
	}
}

func TestProjectRootMovedBaseRetainsStageBeforeMarkerRemoval(t *testing.T) {
	for _, boundary := range []string{"marker-present", "marker-absent"} {
		t.Run(boundary, func(t *testing.T) {
			_, _, spec, _ := projectRootFixture(t)
			prepared, err := prepareNativeRoot(t.Context(), spec)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := prepared.close(); err != nil {
					t.Error(err)
				}
			}()
			marker := filepath.Join(prepared.selected.path, prepared.markerName)
			before, err := os.Stat(marker)
			if err != nil {
				t.Fatal(err)
			}
			if boundary == "marker-absent" {
				if err := prepared.closeMarkerHandle(); err != nil {
					t.Fatal(err)
				}
				if err := prepared.selected.root.Remove(prepared.markerName); err != nil {
					t.Fatal(err)
				}
			}
			relative, err := filepath.Rel(prepared.base, marker)
			if err != nil {
				t.Fatal(err)
			}
			moved := prepared.base + "-moved"
			if !moveNativeRootFixture(t, prepared.baseRoot, moved) {
				if err := prepared.verifyNamed(); err != nil {
					t.Fatal(err)
				}
				if prepared.staged != prepared.selected {
					entries, err := os.ReadDir(prepared.staged.path)
					if err != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(prepared.selected.path) {
						t.Fatalf("denied replacement changed original staging contents: %v %v", entries, err)
					}
				}
				entries, err := os.ReadDir(prepared.selected.path)
				if boundary == "marker-absent" {
					if err != nil || len(entries) != 0 {
						t.Fatalf("denied replacement changed original empty stage: %v %v", entries, err)
					}
				} else {
					body, readErr := prepared.selected.readMarker(prepared.markerName)
					current, statErr := prepared.selected.root.Lstat(prepared.markerName)
					if err != nil || len(entries) != 1 || entries[0].Name() != prepared.markerName || readErr != nil || statErr != nil ||
						!sameNativeMarker(before, current) || string(body) != string(prepared.markerBody) {
						t.Fatalf("denied replacement changed original marker: %v %v %v", err, readErr, statErr)
					}
				}
				return
			}
			if err := os.Mkdir(prepared.base, 0o700); err != nil {
				t.Fatal(err)
			}
			err = prepared.cleanup()
			if err == nil {
				t.Fatal("cleanup accepted a moved base")
			}
			if boundary == "marker-absent" {
				info, err := os.Lstat(filepath.Dir(filepath.Join(moved, relative)))
				if err != nil || !sameNativeRoot(prepared.selected.info, info) {
					t.Fatalf("refused cleanup removed the moved empty directory: %v", err)
				}
			} else {
				body, err := os.ReadFile(filepath.Join(moved, relative))
				if err != nil || string(body) != string(prepared.markerBody) {
					t.Fatalf("refused cleanup changed the moved original marker: %q %v", body, err)
				}
			}
			entries, err := os.ReadDir(prepared.base)
			if err != nil || len(entries) != 0 {
				t.Fatalf("refused cleanup touched replacement base: %v %v", entries, err)
			}
		})
	}
}
