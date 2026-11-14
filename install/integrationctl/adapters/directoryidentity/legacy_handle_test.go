//go:build linux || darwin || windows

package directoryidentity

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
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
		// Windows does not allow this rename while the original os.Open
		// directory handle is live. Assert the native refusal and identity.
		if runtime.GOOS != "windows" || !errors.Is(err, syscall.Errno(32)) {
			t.Fatal(err)
		}
		got, heldErr := LegacyHandleIdentity(file)
		if heldErr != nil || got != want {
			t.Fatalf("failed rename changed held identity: %q want %q: %v", got, want, heldErr)
		}
		if _, statErr := os.Lstat(path + "-owned"); !os.IsNotExist(statErr) {
			t.Fatalf("failed rename published replacement: %v", statErr)
		}
		return
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
