package platformexec

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestInitialGeminiManagedPathSetIncludesGeneratedHooksOnlyWhenNeeded(t *testing.T) {
	t.Parallel()

	got := initialGeminiManagedPathSet(pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Targets: []string{"gemini"}},
		Launcher: &pluginmodel.Launcher{Entrypoint: "./bin/demo"},
	}, pluginmodel.NewTargetState("gemini"))
	if _, ok := got[filepath.ToSlash(filepath.Join("hooks", "hooks.json"))]; !ok {
		t.Fatalf("managed paths = %+v", got)
	}
}

func TestGeminiManagedPathsPreservesPackageMetadataParsePath(t *testing.T) {
	t.Parallel()

	state := pluginmodel.NewTargetState("gemini")
	state.SetDoc("package_metadata", filepath.Join("targets", "gemini", "package.yaml"))
	_, err := (geminiAdapter{}).ManagedPaths(t.TempDir(), pluginmodel.PackageGraph{}, state)
	if err == nil || !strings.Contains(err.Error(), "parse targets/gemini/package.yaml:") {
		t.Fatalf("error = %v", err)
	}
}

func TestBuildGeminiManagedPathsAddsPrimaryAndExtraContexts(t *testing.T) {
	t.Parallel()

	seen := map[string]struct{}{}
	state := pluginmodel.NewTargetState("gemini")
	state.AddComponent("contexts", filepath.Join("targets", "gemini", "contexts", "GEMINI.md"))
	state.AddComponent("contexts", filepath.Join("targets", "gemini", "contexts", "extra.md"))

	got := buildGeminiManagedPaths(seen, state, geminiContextSelection{
		ArtifactName: "GEMINI.md",
		SourcePath:   filepath.Join("targets", "gemini", "contexts", "GEMINI.md"),
	}, true)
	if len(got) != 2 || got[0] != "GEMINI.md" || got[1] != filepath.ToSlash(filepath.Join("contexts", "extra.md")) {
		t.Fatalf("paths = %+v", got)
	}
}

func TestGeminiGeneratedHooksPathUsesCanonicalLocation(t *testing.T) {
	t.Parallel()

	if got := geminiGeneratedHooksPath(); got != filepath.ToSlash(filepath.Join("hooks", "hooks.json")) {
		t.Fatalf("path = %q", got)
	}
}
