package loader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStdioPermittedDotSegments(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"bin", "data"} {
		if e := os.Mkdir(filepath.Join(root, d), 0700); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.WriteFile(filepath.Join(root, "bin/server"), []byte("inert"), 0600); e != nil {
		t.Fatal(e)
	}
	for _, cwd := range []string{"./", "./data/..", "${PLUGIN_ROOT}"} {
		config := map[string]any{"command": "./bin/../bin/server", "cwd": cwd}
		r, e := validateStdioServer(root, config)
		if e != nil {
			t.Fatal(e)
		}
		if r.BundledRelativePath != "bin/../bin/server" || config["cwd"] != cwd {
			t.Fatal("raw path changed")
		}
	}
	for _, cwd := range []string{"./../outside", "${PLUGIN_ROOT}/../data"} {
		if _, e := validateStdioServer(root, map[string]any{"command": "node", "cwd": cwd}); e == nil {
			t.Fatalf("accepted %s", cwd)
		}
	}
	if _, e := validateStdioServer(root, map[string]any{"command": "./missing/../bin/server"}); e != nil {
		t.Fatalf("missing is readiness: %v", e)
	}
}
