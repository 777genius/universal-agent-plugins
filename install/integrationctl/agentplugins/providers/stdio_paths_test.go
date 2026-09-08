package providers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStdioPathProjectionPreservesTraversalAndAnchor(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"bin", "data", "real/deep"} {
		if e := os.MkdirAll(filepath.Join(root, dir), 0700); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.WriteFile(filepath.Join(root, "bin/server"), []byte("inert"), 0600); e != nil {
		t.Fatal(e)
	}
	active := filepath.Join(t.TempDir(), "future")
	data := t.TempDir()
	for _, cwd := range []string{"./", "./data/..", "${PLUGIN_ROOT}"} {
		command, dir, e := resolveStdioPaths("./bin/../bin/server", cwd, active, data, root)
		if e != nil {
			t.Fatal(e)
		}
		if command != filepath.Join(active, "bin/server") || dir != active {
			t.Fatalf("%s %s", command, dir)
		}
	}
	for _, input := range []struct{ command, cwd string }{{"./missing/../bin/server", "./"}, {"node", "${PLUGIN_ROOT}/../" + filepath.Base(data)}, {"node", "${PLUGIN_DATA}/../" + filepath.Base(root)}} {
		if _, _, e := resolveStdioPaths(input.command, input.cwd, root, data); e == nil {
			t.Fatalf("accepted %+v", input)
		}
	}
	if e := os.Symlink(filepath.Join(root, "real/deep"), filepath.Join(root, "link")); e != nil {
		t.Skip(e)
	}
	_, dir, e := resolveStdioPaths("node", "./link/..", root, data)
	if e != nil || dir != filepath.Join(root, "real") {
		t.Fatalf("symlink/.. = %s %v", dir, e)
	}
	if e := os.Symlink(data, filepath.Join(root, "out")); e != nil {
		t.Fatal(e)
	}
	if _, _, e := resolveStdioPaths("node", "./out", root, data); e == nil {
		t.Fatal("cross-anchor symlink accepted")
	}
}
