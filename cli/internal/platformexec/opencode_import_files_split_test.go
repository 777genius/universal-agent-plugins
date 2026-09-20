package platformexec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportedOpenCodeConfigDisplayPathUsesDisplayBase(t *testing.T) {
	t.Parallel()

	got := importedOpenCodeConfigDisplayPath(filepath.Join("/tmp/demo", "opencode.json"), ".opencode")
	if got != ".opencode/opencode.json" {
		t.Fatalf("display path = %q", got)
	}
}

func TestMergeOpenCodeObjectRecursesNestedMaps(t *testing.T) {
	t.Parallel()

	dst := map[string]any{
		"agent": map[string]any{"planner": "keep"},
	}
	src := map[string]any{
		"agent": map[string]any{"reviewer": "add"},
	}
	mergeOpenCodeObject(dst, src)
	agent, _ := dst["agent"].(map[string]any)
	if agent["planner"] != "keep" || agent["reviewer"] != "add" {
		t.Fatalf("merged agent = %#v", agent)
	}
}

func TestImportDirectoryArtifactsRejectingSymlinksRejectsSymlink(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "target.txt")
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	sourceDir := filepath.Join(root, "tools")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(sourceDir, "linked.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	_, _, err := importDirectoryArtifactsRejectingSymlinks([]opencodeImportSource{{
		dir:     sourceDir,
		display: ".opencode/tools",
	}}, "plugin/targets/opencode/tools", nil)
	if err == nil || !strings.Contains(err.Error(), "does not support symlinks") {
		t.Fatalf("error = %v", err)
	}
}
