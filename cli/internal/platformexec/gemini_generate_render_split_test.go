package platformexec

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestGeminiRenderEntrypointTrimsWhitespace(t *testing.T) {
	t.Parallel()

	got, err := geminiRenderEntrypoint(pluginmodel.PackageGraph{
		Launcher: &pluginmodel.Launcher{Entrypoint: "  ./bin/demo  "},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "./bin/demo" {
		t.Fatalf("entrypoint = %q", got)
	}
}

func TestLoadGeminiRenderMetaPreservesPackageMetadataPath(t *testing.T) {
	t.Parallel()

	state := pluginmodel.NewTargetState("gemini")
	state.SetDoc("package_metadata", filepath.Join("targets", "gemini", "package.yaml"))
	_, err := loadGeminiRenderMeta(t.TempDir(), state)
	if err == nil || !strings.Contains(err.Error(), "parse targets/gemini/package.yaml:") {
		t.Fatalf("error = %v", err)
	}
}
