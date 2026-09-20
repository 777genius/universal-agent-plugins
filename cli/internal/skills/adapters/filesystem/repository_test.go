package filesystem

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/skills/domain"
)

func TestCommandLineQuotesShellUnsafeArgs(t *testing.T) {
	t.Parallel()

	got := CommandLine(domain.SkillSpec{Command: "python", Args: []string{"-m", "demo tool", "O'Reilly"}})
	want := "python -m 'demo tool' 'O'\\''Reilly'"
	if got != want {
		t.Fatalf("command line = %q, want %q", got, want)
	}
}

func TestListManagedArtifactsIncludesGeneratedSkillsAndDocs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	mustWriteRepoFile(t, filepath.Join(root, "generated", "skills", "claude", "demo", "SKILL.md"), "# demo\n")
	mustWriteRepoFile(t, filepath.Join(root, "commands", "demo.md"), "This file is generated from `skills/demo/SKILL.md`\nRegenerate with `plugin-kit-ai skills generate`.\n")
	mustWriteRepoFile(t, filepath.Join(root, "commands", "notes.md"), "handwritten\n")

	artifacts, err := (Repository{}).ListManagedArtifacts(root, map[string]struct{}{"claude": {}})
	if err != nil {
		t.Fatalf("ListManagedArtifacts: %v", err)
	}
	if len(artifacts) != 2 {
		t.Fatalf("artifacts = %#v", artifacts)
	}
	if artifacts[0] != filepath.Join("commands", "demo.md") || artifacts[1] != filepath.Join("generated", "skills", "claude", "demo", "SKILL.md") {
		t.Fatalf("artifacts = %#v", artifacts)
	}
}

func mustWriteRepoFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
