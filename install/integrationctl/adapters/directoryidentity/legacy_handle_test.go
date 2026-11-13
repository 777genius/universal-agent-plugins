//go:build linux || darwin || windows

package directoryidentity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyHeldIdentitySurvivesNamedDirectoryReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "selected")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	want, err := LegacyIdentity(path, before)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := os.Rename(path, path+"-owned"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	got, err := LegacyHandleIdentity(file)
	if err != nil || got != want {
		t.Fatalf("held identity reopened replaced name: %q want %q, %v", got, want, err)
	}
	other, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	current, err := LegacyIdentity(path, other)
	if err != nil || current == got {
		t.Fatalf("replacement has held identity: %q %v", current, err)
	}
}
