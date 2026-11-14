package filetree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopyFilePreservesExecutableIntentAndStripsSpecialBits(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	src := filepath.Join(root, "src", "server")
	dest := filepath.Join(root, "dest", "server")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(src, []byte("server\n"), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Chmod(src, 0o4755); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	if err := CopyFile(src, dest); err != nil {
		t.Fatalf("copy: %v", err)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Fatalf("destination mode = %o, want 755", got)
	}
	if info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		t.Fatalf("destination retained special bits: %v", info.Mode())
	}
}

func TestCopyPathRejectsSymlink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	target := filepath.Join(root, "target")
	link := filepath.Join(root, "link")
	if err := os.WriteFile(target, []byte("target"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := CopyPathIfExists(link, filepath.Join(root, "dest")); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("copy error = %v, want symlink rejection", err)
	}
}
