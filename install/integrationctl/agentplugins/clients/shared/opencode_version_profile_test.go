package shared

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCodeVersionProfileRejectsNonemptyOrAliasedDirectory(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	canary := filepath.Join(root, "foreign")
	if err := os.WriteFile(canary, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareOpenCodeVersionProbeEnvironment(root); err == nil {
		t.Fatal("accepted a nonempty directory")
	}
	if body, err := os.ReadFile(canary); err != nil || string(body) != "keep" {
		t.Fatalf("foreign file changed: %q, %v", body, err)
	}
	if _, err := PrepareOpenCodeVersionProbeEnvironment(filepath.Join(root, "missing")); err == nil {
		t.Fatal("accepted a missing directory")
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err == nil {
		if _, err := PrepareOpenCodeVersionProbeEnvironment(alias); err == nil {
			t.Fatal("accepted a symlink directory")
		}
	}
}
