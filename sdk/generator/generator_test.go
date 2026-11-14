package generator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratedArtifactsUpToDate(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := FindRepoRoot(cwd)
	if err != nil {
		t.Fatal(err)
	}
	arts, err := RenderArtifacts()
	if err != nil {
		t.Fatal(err)
	}
	for _, art := range arts {
		if art.Path == "docs/generated/target_support_matrix.md" {
			continue
		}
		got, err := os.ReadFile(filepath.Join(root, art.Path))
		if err != nil {
			t.Fatalf("read %s: %v", art.Path, err)
		}
		if string(got) != string(art.Content) {
			t.Fatalf("generated artifact out of date: %s", art.Path)
		}
	}
}
