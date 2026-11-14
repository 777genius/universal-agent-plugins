package platformexec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveImportedOpenCodeConfigSourceCarriesDisplayPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "opencode.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	source, ok, err := resolveImportedOpenCodeConfigSource(root, ".opencode")
	if err != nil {
		t.Fatalf("resolveImportedOpenCodeConfigSource: %v", err)
	}
	if !ok {
		t.Fatal("expected config source to be detected")
	}
	if source.displayPath != ".opencode/opencode.json" {
		t.Fatalf("displayPath = %q", source.displayPath)
	}
}

func TestReadImportedOpenCodeConfigSourceDecodesBody(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "opencode.json")
	if err := os.WriteFile(path, []byte("{\"instructions\":[\"demo\"]}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	config, err := readImportedOpenCodeConfigSource(importedOpenCodeConfigSource{path: path})
	if err != nil {
		t.Fatalf("readImportedOpenCodeConfigSource: %v", err)
	}
	if !config.InstructionsSet || len(config.Instructions) != 1 || config.Instructions[0] != "demo" {
		t.Fatalf("config = %#v", config)
	}
}
