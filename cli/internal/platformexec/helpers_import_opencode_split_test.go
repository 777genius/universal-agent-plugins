package platformexec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveOpenCodeConfigPathReturnsRelativePath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "opencode.jsonc"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	path, warnings, ok, err := resolveOpenCodeConfigPath(root)
	if err != nil {
		t.Fatalf("resolveOpenCodeConfigPath: %v", err)
	}
	if !ok {
		t.Fatal("expected config to be detected")
	}
	if path != "opencode.jsonc" {
		t.Fatalf("path = %q, want %q", path, "opencode.jsonc")
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
}

func TestReadImportedOpenCodeConfigFromFileDefaultsDisplayPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "opencode.json")
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, displayPath, warnings, ok, err := readImportedOpenCodeConfigFromFile(path, "")
	if err != nil {
		t.Fatalf("readImportedOpenCodeConfigFromFile: %v", err)
	}
	if !ok {
		t.Fatal("expected config to be read")
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
	if displayPath != filepath.ToSlash(path) {
		t.Fatalf("displayPath = %q, want %q", displayPath, filepath.ToSlash(path))
	}
}
