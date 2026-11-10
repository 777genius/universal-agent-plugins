package shared

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDigestSkillDirectoryRequiresRealDirectoryRoot(t *testing.T) {
	root := t.TempDir()
	owned := filepath.Join(root, "managed-skill")
	if err := os.Mkdir(owned, 0o700); err != nil {
		t.Fatal(err)
	}
	const content = "hello\n"
	file := filepath.Join(owned, "SKILL.md")
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	const expected = "sha256:7ebaa6bdbac050e4f801a1c9082e6057f430d8b1c73765c2040e1e9f868778c4"
	if got, err := DigestSkillDirectory(owned); err != nil || got != expected {
		t.Fatalf("valid directory digest changed: %q, %v", got, err)
	}

	// Before this guard a regular file masquerading as a root was hashed as
	// the empty tree because WalkDir intentionally omits the root itself.
	if _, err := DigestSkillDirectory(file); err == nil || !strings.Contains(err.Error(), "not a real directory") {
		t.Fatalf("regular file root was accepted as a managed Skill: %v", err)
	}

	link := filepath.Join(root, "root-link")
	if err := os.Symlink(owned, link); err != nil {
		if runtime.GOOS != "windows" {
			t.Fatalf("unable to exercise symlink-root rejection: %v", err)
		}
		// The Windows runner may lack CreateSymbolicLink privilege. Its
		// regular-file negative case is still mandatory above.
		t.Logf("Windows symlink creation unavailable; root-file negative case executed: %v", err)
		return
	}
	if _, err := DigestSkillDirectory(link); err == nil || !strings.Contains(err.Error(), "not a real directory") {
		t.Fatalf("symlink root was accepted as a managed Skill: %v", err)
	}
}
