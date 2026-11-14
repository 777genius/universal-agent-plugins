package transaction

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

func TestProjectRootSubstitutedBaseCannotCreateStaging(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	held, err := holdNativeRoot(base)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := held.root.Close(); err != nil {
			t.Error(err)
		}
	}()
	if !moveNativeRootFixture(t, held, base+"-owned") {
		entries, err := os.ReadDir(base)
		if err != nil || len(entries) != 0 {
			t.Fatalf("denied replacement changed original contents: %v %v", entries, err)
		}
		return
	}
	t.Cleanup(func() { _ = os.RemoveAll(base + "-owned") })
	if err := os.Mkdir(base, 0o700); err != nil {
		t.Fatal(err)
	}
	prepared := &preparedNativeRoot{base: base, active: filepath.Join(base, "selected"), baseRoot: held}
	if err := prepared.create(prepared.active); err == nil {
		t.Fatal("created staging through substituted base")
	}
	for _, path := range []string{base, base + "-owned"} {
		entries, err := os.ReadDir(path)
		if err != nil || len(entries) != 0 {
			t.Fatalf("refused staging created files: %v %v", entries, err)
		}
	}
}

// A production os.OpenRoot handle prevents its own replacement on Windows.
// That precise protection is a successful outcome; other rename errors are not.
func moveNativeRootFixture(t *testing.T, held *heldNativeRoot, destination string) bool {
	t.Helper()
	if err := held.verifyNamed(); err != nil {
		t.Fatal(err)
	}
	before, err := held.root.Stat(".")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(held.path, destination); err == nil {
		return true
	} else if runtime.GOOS != "windows" || !errors.Is(err, syscall.Errno(32)) {
		t.Fatal(err)
	}
	if err := held.verifyNamed(); err != nil {
		t.Fatal(err)
	}
	after, err := held.root.Stat(".")
	if err != nil || !sameNativeRoot(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("denied replacement changed original directory: %v", err)
	}
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		t.Fatalf("denied replacement created a destination: %v", err)
	}
	return false
}

func TestProjectRootSubstitutedChildCannotWriteOrReadMarker(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	parent, err := holdNativeRoot(base)
	if err != nil {
		t.Fatal(err)
	}
	if err := parent.root.Mkdir("stage", 0o700); err != nil {
		t.Fatal(err)
	}
	child, err := parent.child("stage")
	if err != nil {
		t.Fatal(err)
	}
	prepared := &preparedNativeRoot{base: base, staging: child.path, baseRoot: parent,
		staged: child, selected: child, markerName: ".owner", markerBody: []byte("owned")}
	defer func() {
		if err := prepared.close(); err != nil {
			t.Error(err)
		}
	}()
	if err := os.Rename(child.path, child.path+"-owned"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(child.path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := prepared.writeMarker(); err == nil {
		t.Fatal("wrote marker through substituted child")
	}
	for _, path := range []string{child.path, child.path + "-owned"} {
		if _, err := os.Lstat(filepath.Join(path, prepared.markerName)); !os.IsNotExist(err) {
			t.Fatalf("refused marker write had an effect: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(child.path, prepared.markerName), []byte("foreign bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if body, err := child.readMarker(prepared.markerName); err == nil || len(body) != 0 {
		t.Fatalf("read marker from substituted directory: %q %v", body, err)
	}
	if err := prepared.cleanup(); err == nil {
		t.Fatal("cleanup adopted substituted child")
	}
	if body, err := os.ReadFile(filepath.Join(child.path, prepared.markerName)); err != nil || string(body) != "foreign bytes" {
		t.Fatalf("cleanup touched foreign marker: %q %v", body, err)
	}
}

func TestProjectRootPreJournalCleanupRemovesOnlyFreshStaging(t *testing.T) {
	_, _, spec, _ := projectRootFixture(t)
	prepared, err := prepareNativeRoot(t.Context(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := prepared.cleanup(); err != nil {
		t.Fatal(err)
	}
	if err := prepared.close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(prepared.staging); !os.IsNotExist(err) {
		t.Fatalf("unpublished staging survived exact cleanup: %v", err)
	}
	if _, err := os.Lstat(spec.Path); !os.IsNotExist(err) {
		t.Fatalf("cleanup published native root: %v", err)
	}
}
