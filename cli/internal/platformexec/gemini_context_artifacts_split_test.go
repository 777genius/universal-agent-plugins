package platformexec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestReadGeminiPrimaryContextArtifactUsesArtifactName(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sourcePath := filepath.Join("targets", "gemini", "contexts", "GEMINI.md")
	if err := os.MkdirAll(filepath.Join(root, "targets", "gemini", "contexts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, sourcePath), []byte("primary"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readGeminiPrimaryContextArtifact(root, geminiContextSelection{
		ArtifactName: "GEMINI.md",
		SourcePath:   sourcePath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.RelPath != "GEMINI.md" || string(got.Content) != "primary" {
		t.Fatalf("artifact = %+v", got)
	}
}

func TestSortGeminiContextArtifactsOrdersByRelPath(t *testing.T) {
	t.Parallel()

	got := sortGeminiContextArtifacts([]pluginmodel.Artifact{
		{RelPath: "contexts/zeta.md"},
		{RelPath: "contexts/alpha.md"},
	})
	if got[0].RelPath != "contexts/alpha.md" || got[1].RelPath != "contexts/zeta.md" {
		t.Fatalf("artifacts = %+v", got)
	}
}
