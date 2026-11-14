package platformexec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestMergeGeminiManifestAssetSectionSkipsEmptyValues(t *testing.T) {
	t.Parallel()

	manifest := map[string]any{}
	mergeGeminiManifestAssetSection(manifest, "settings", nil)
	if len(manifest) != 0 {
		t.Fatalf("manifest = %#v", manifest)
	}

	mergeGeminiManifestAssetSection(manifest, "settings", []map[string]any{{"name": "Demo"}})
	if _, ok := manifest["settings"]; !ok {
		t.Fatalf("manifest = %#v", manifest)
	}
}

func TestBuildGeminiContextArtifactsReturnsPrimaryThenExtraArtifacts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	contextsDir := filepath.Join(root, "targets", "gemini", "contexts")
	if err := os.MkdirAll(contextsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeGeminiValidateFile(t, filepath.Join(contextsDir, "GEMINI.md"), "# Primary\n")
	writeGeminiValidateFile(t, filepath.Join(contextsDir, "extra.md"), "# Extra\n")

	state := pluginmodel.NewTargetState("gemini")
	state.AddComponent("contexts", filepath.Join("targets", "gemini", "contexts", "GEMINI.md"))
	state.AddComponent("contexts", filepath.Join("targets", "gemini", "contexts", "extra.md"))

	name, artifacts, ok, err := buildGeminiContextArtifacts(root, pluginmodel.PackageGraph{}, state, geminiPackageMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected context artifacts")
	}
	if name != "GEMINI.md" {
		t.Fatalf("context name = %q", name)
	}
	if len(artifacts) != 2 {
		t.Fatalf("artifacts = %+v", artifacts)
	}
	if artifacts[0].RelPath != "GEMINI.md" {
		t.Fatalf("primary relpath = %q", artifacts[0].RelPath)
	}
	if artifacts[1].RelPath != filepath.ToSlash(filepath.Join("contexts", "extra.md")) {
		t.Fatalf("extra relpath = %q", artifacts[1].RelPath)
	}
}
