package installer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestProjectNamespaceRejectsAnotherProjectsSymlink(t *testing.T) {
	root := t.TempDir()
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	for _, dir := range []string{a, filepath.Join(b, ".uap")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(b, ".uap", "foreign-marker")
	if err := os.WriteFile(marker, []byte("preserve-project-b"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(b, ".uap"), filepath.Join(a, ".uap")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := New(testConfig(t, Config{ProjectRoot: a, StateRoot: filepath.Join(a, ".uap")})); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("another project's namespace was accepted: %v", err)
	}
	if body, err := os.ReadFile(marker); err != nil || string(body) != "preserve-project-b" {
		t.Fatalf("foreign project changed: %q, %v", body, err)
	}
	entries, err := os.ReadDir(filepath.Join(b, ".uap"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("namespace rejection created effects: %v, %v", entries, err)
	}
}

func TestProjectNamespacePinsExistingDirectoryIdentity(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(root, ".uap")
	if err := os.Mkdir(state, 0o700); err != nil {
		t.Fatal(err)
	}
	engine, err := New(testConfig(t, Config{ProjectRoot: root, StateRoot: state}))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(state, filepath.Join(root, ".uap-original")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(state, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := engine.CheckProjectNamespace(); !errors.Is(err, ErrPlanChanged) {
		t.Fatalf("replaced namespace identity was accepted: %v", err)
	}
}
