package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExampleFilesExistInExampleRoot(t *testing.T) {
	t.Helper()
	root := filepath.Join("..", "..")
	for _, rel := range []string{
		filepath.Join("skills", "lint-repo", "SKILL.md"),
		filepath.Join("generated", "skills", "claude", "lint-repo", "SKILL.md"),
		filepath.Join("generated", "skills", "codex", "lint-repo", "SKILL.md"),
		filepath.Join("commands", "lint-repo.md"),
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
}
