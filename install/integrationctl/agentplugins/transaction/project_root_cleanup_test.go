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
