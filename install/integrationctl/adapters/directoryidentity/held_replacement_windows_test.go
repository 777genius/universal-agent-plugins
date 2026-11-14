//go:build windows && (amd64 || arm64)

package directoryidentity

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsCallerHeldDirectorySurvivesNamedReplacement(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(base, "selected")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	file, err := openHeldDirectoryForReplacement(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	}()
	want, err := LegacyHandleIdentity(file)
	if err != nil {
		t.Fatal(err)
	}
	if authority, err := CaptureHeld(t.Context(), path, file); err != nil || authority.IsZero() {
		t.Fatalf("original caller-held directory unavailable: %v", err)
	}
	if err := os.Rename(path, path+"-retained"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if authority, err := CaptureHeld(t.Context(), path, file); !errors.Is(err, ErrChanged) || !authority.IsZero() {
		t.Fatalf("caller-held capture adopted named replacement: %v", err)
	}
	got, err := LegacyHandleIdentity(file)
	if err != nil || got != want {
		t.Fatalf("caller-held original identity changed: %q want %q: %v", got, want, err)
	}
	other, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	current, err := LegacyIdentity(path, other)
	if err != nil || current == got {
		t.Fatalf("named replacement has caller-held identity: %q %v", current, err)
	}
}

// The caller's original handle permits the replacement that this test observes.
// Production directory handles and the ordinary os.Open refusal tests stay strict.
func openHeldDirectoryForReplacement(path string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}
