//go:build (linux && (amd64 || arm64)) || (darwin && arm64) || (windows && (amd64 || arm64))

package directoryidentity

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

func TestCaptureHeldJoinsOriginalDirectory(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "original")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	}()
	ordinary, err := Capture(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	held, err := CaptureHeld(t.Context(), root, file)
	if err != nil || !held.Equal(ordinary) {
		t.Fatalf("held capture changed facts: %v", err)
	}
	if err := os.Rename(root, root+"-retained"); err != nil {
		// A Windows directory handle opened by os.Open denies rename with
		// ERROR_SHARING_VIOLATION. This is a native protection, not a skip:
		// prove the selected original identity stayed attached to its name.
		if runtime.GOOS != "windows" || !errors.Is(err, syscall.Errno(32)) {
			t.Fatal(err)
		}
		still, captureErr := CaptureHeld(t.Context(), root, file)
		if captureErr != nil || !still.Equal(ordinary) {
			t.Fatalf("held directory changed after native rename refusal: %v", captureErr)
		}
		if _, statErr := os.Lstat(root + "-retained"); !os.IsNotExist(statErr) {
			t.Fatalf("native rename refusal published a new name: %v", statErr)
		}
		return
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if authority, err := CaptureHeld(t.Context(), root, file); !errors.Is(err, ErrChanged) || !authority.IsZero() {
		t.Fatalf("adopted replacement in place of original held directory: %v", err)
	}
}

func TestCaptureHeldRequiresLiveOriginalAndContext(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if authority, err := CaptureHeld(ctx, root, file); !errors.Is(err, context.Canceled) || !authority.IsZero() {
		t.Fatalf("canceled capture: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	for _, original := range []*os.File{nil, file} {
		if authority, err := CaptureHeld(t.Context(), root, original); err == nil || !authority.IsZero() {
			t.Fatal("accepted absent/closed original handle")
		}
	}
}
