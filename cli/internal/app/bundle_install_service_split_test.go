package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathEmptyReturnsFalseForFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	empty, err := pathEmpty(path)
	if err != nil {
		t.Fatal(err)
	}
	if empty {
		t.Fatal("expected file path to be treated as non-empty")
	}
}

func TestPathExistsReturnsFalseForMissingPath(t *testing.T) {
	t.Parallel()

	exists, err := pathExists(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("expected missing path")
	}
}
