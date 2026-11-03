//go:build windows

package atomicfile

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestAdmittedWindowsReaderPreservesDeleteSharing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata.json")
	if err := Write(path, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := openRegularNoFollow(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := Write(path, []byte("after"), 0o600); err != nil {
		t.Fatalf("admitted reader blocked atomic publication: %v", err)
	}
	body, err := io.ReadAll(file)
	if err != nil || string(body) != "before" {
		t.Fatalf("reader lost held file: %q, %v", body, err)
	}
	if body, err := os.ReadFile(path); err != nil || string(body) != "after" {
		t.Fatalf("replacement missing: %q, %v", body, err)
	}
}
