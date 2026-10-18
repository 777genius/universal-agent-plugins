package shared

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
		command, dir, e := ResolveStdioPaths("./bin/../bin/server", cwd, active, data, root)
		if e != nil {
			t.Fatal(e)
		}
		if command != filepath.Join(active, "bin/server") || dir != active {
			t.Fatalf("%s %s", command, dir)
		}
	}
	for _, input := range []struct{ command, cwd string }{{"./missing/../bin/server", "./"}, {"node", "${PLUGIN_ROOT}/../" + filepath.Base(data)}, {"node", "${PLUGIN_DATA}/../" + filepath.Base(root)}} {
		if _, _, e := ResolveStdioPaths(input.command, input.cwd, root, data); e == nil {
			t.Fatalf("accepted %+v", input)
		}
	}
	if e := os.Symlink(filepath.Join(root, "real/deep"), filepath.Join(root, "link")); e != nil {
		t.Skip(e)
	}
	_, dir, e := ResolveStdioPaths("node", "./link/..", root, data)
	if e != nil || dir != filepath.Join(root, "real") {
		t.Fatalf("symlink/.. = %s %v", dir, e)
	}
	if e := os.Symlink(data, filepath.Join(root, "out")); e != nil {
		t.Fatal(e)
	}
	if _, _, e := ResolveStdioPaths("node", "./out", root, data); e == nil {
		t.Fatal("cross-anchor symlink accepted")
	}
}

func TestApplyStdioDataContractResolvesOnlyAuthoringTokenInCommand(t *testing.T) {
	active := filepath.Join(t.TempDir(), "future")
	data := t.TempDir()
	if e := os.MkdirAll(filepath.Join(data, "work"), 0700); e != nil {
		t.Fatal(e)
	}
	config := map[string]any{
		"command": "${package.root}/bin/server",
		"args":    []any{"${PLUGIN_ROOT}/bin/run", "${package.root}/share", "${PLUGIN_DATA}/cache"},
		"cwd":     "${PLUGIN_DATA}/work",
		"env":     map[string]any{"CACHE": "${PLUGIN_DATA}/cache"},
	}
	if e := ApplyStdioDataContract(config, active, data); e != nil {
		t.Fatal(e)
	}
	if config["command"] != filepath.Join(active, "bin/server") {
		t.Fatalf("command = %v", config["command"])
	}
	args := config["args"].([]any)
	if args[0] != filepath.Join(active, "bin/run") || args[1] != filepath.Join(active, "share") || args[2] != filepath.Join(data, "cache") {
		t.Fatalf("args = %v", args)
	}
	env := config["env"].(map[string]any)
	if env["CACHE"] != filepath.Join(data, "cache") || env["PLUGIN_ROOT"] != active || env["PLUGIN_DATA"] != data {
		t.Fatalf("env = %v", env)
	}

	// Client-managed placeholders in the command stay verbatim: the managed
	// launcher resolves command form itself and rejects rewritten commands.
	verbatim := map[string]any{"command": "${PLUGIN_ROOT}/must-not-expand", "cwd": "${PLUGIN_DATA}/work"}
	if e := ApplyStdioDataContract(verbatim, active, data); e != nil {
		t.Fatal(e)
	}
	if verbatim["command"] != "${PLUGIN_ROOT}/must-not-expand" {
		t.Fatalf("command = %v", verbatim["command"])
	}
}
