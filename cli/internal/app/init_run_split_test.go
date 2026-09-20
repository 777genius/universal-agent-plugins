package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/scaffold"
)

func TestBuildInitScaffoldDataSetsCodexModelForRuntime(t *testing.T) {
	t.Parallel()

	data := buildInitScaffoldData("demo", InitOptions{}, resolvedInitConfig{
		TemplateName: scaffold.InitTemplateCustomLogic,
		Platform:     "codex-runtime",
		Runtime:      scaffold.RuntimeGo,
	})
	if data.CodexModel != scaffold.DefaultCodexModel {
		t.Fatalf("codex model = %q", data.CodexModel)
	}
}

func TestShouldGenerateInitArtifactsDetectsAuthoredManifest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, pluginmodel.SourceDirName, "plugin.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("api_version: v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !shouldGenerateInitArtifacts(root) {
		t.Fatal("expected authored manifest to trigger generation")
	}
}
