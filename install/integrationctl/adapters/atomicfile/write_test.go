package atomicfile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteCreatesAndReplacesFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	if err := Write(path, []byte("first\n"), 0o640); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := Write(path, []byte("second\n"), 0o600); err != nil {
		t.Fatalf("second write: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "second\n" {
		t.Fatalf("body = %q", body)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
}

func TestWriteRejectsSymlinkTarget(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is not generally available without elevated privileges")
	}
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.WriteFile(target, []byte("safe"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := Write(link, []byte("unsafe"), 0o644); err == nil {
		t.Fatal("symlink target accepted")
	}
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "safe" {
		t.Fatalf("symlink target changed: %q", body)
	}
}
