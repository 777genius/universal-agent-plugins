package securityscan

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCommandScannerIncludesNonzeroExitWhenStderrIsEmpty(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture")
	}
	root := t.TempDir()
	executable := filepath.Join(root, "lintai-exit-two")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nexit 2\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	_, err := (CommandScanner{Executable: executable}).Scan(context.Background(), root)
	if err == nil || !strings.Contains(err.Error(), "lintai security scan failed: exit status 2") {
		t.Fatalf("expected the actual process exit status, got %v", err)
	}
}
