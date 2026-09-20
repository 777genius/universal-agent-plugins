package platformexec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImportDirectoryArtifactsWithWarningsWarnsOnlyWhenUsed(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "commands")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "demo.toml"), []byte("name='demo'\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, warnings, err := importDirectoryArtifactsWithWarnings([]opencodeImportSource{{
		dir:       dir,
		warnOnUse: true,
		warnPath:  ".opencode/commands",
		warnMsg:   "demo warning",
	}}, "plugin/targets/opencode/commands", func(rel string) bool { return rel == "demo.toml" })
	if err != nil {
		t.Fatalf("importDirectoryArtifactsWithWarnings: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %#v", warnings)
	}
	if warnings[0].Path != ".opencode/commands" {
		t.Fatalf("warning path = %q", warnings[0].Path)
	}
}
