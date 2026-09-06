//go:build !linux

package packageview

import (
	"context"
	"errors"
	"os"
	"testing"
)

func TestUnsupportedPlatformFailsBeforeScratchEffects(t *testing.T) {
	root, parent := t.TempDir(), t.TempDir()
	_, err := (Reader{TempDir: parent}).Open(context.Background(), root)
	var e *Error
	if !errors.As(err, &e) || e.Code != "platform_unavailable" {
		t.Fatalf("capability = %v", err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatal("unsupported platform wrote scratch")
	}
}
func TestNativeCaptureReleaseGateIncomplete(t *testing.T) {
	t.Skip("mandatory native capture gate INCOMPLETE: no proven macOS/Windows rooted no-device-open backend; fail-closed behavior is not release acceptance")
}
