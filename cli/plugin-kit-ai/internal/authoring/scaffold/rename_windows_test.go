//go:build windows

package scaffold

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsRenameExclusiveExistingDestination(t *testing.T) {
	parent := t.TempDir()
	for _, name := range []string{"source", "destination"} {
		path := filepath.Join(parent, name)
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "marker"), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	dir, err := os.Open(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	err = renameExclusive(dir, "source", dir, "destination")
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("expected os.ErrExist, got %T: %v", err, err)
	}
	for _, name := range []string{"source", "destination"} {
		contents, err := os.ReadFile(filepath.Join(parent, name, "marker"))
		if err != nil || string(contents) != name {
			t.Errorf("%s changed: contents=%q err=%v", name, contents, err)
		}
	}
}
