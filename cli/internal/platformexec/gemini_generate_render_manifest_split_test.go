package platformexec

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestBuildGeminiManifestBaseProjectsManifestIdentity(t *testing.T) {
	t.Parallel()

	got := buildGeminiManifestBase(pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{
			Name:        "demo",
			Version:     "0.1.0",
			Description: "sample",
		},
	})
	if got["name"] != "demo" || got["version"] != "0.1.0" || got["description"] != "sample" {
		t.Fatalf("manifest = %+v", got)
	}
}

func TestMergeGeminiManifestMetaSkipsBlankFields(t *testing.T) {
	t.Parallel()

	manifest := map[string]any{}
	mergeGeminiManifestMeta(manifest, geminiPackageMeta{})
	if len(manifest) != 0 {
		t.Fatalf("manifest = %+v", manifest)
	}
}

func TestMergeGeminiManifestPlanProjectsDirectory(t *testing.T) {
	t.Parallel()

	manifest := map[string]any{}
	mergeGeminiManifestPlan(manifest, geminiPackageMeta{PlanDirectory: "plans"})
	plan, ok := manifest["plan"].(map[string]any)
	if !ok || plan["directory"] != "plans" {
		t.Fatalf("manifest = %+v", manifest)
	}
}

func TestMergeGeminiManifestMigratedToSkipsBlankValues(t *testing.T) {
	t.Parallel()

	manifest := map[string]any{}
	mergeGeminiManifestMigratedTo(manifest, geminiPackageMeta{MigratedTo: "   "})
	if len(manifest) != 0 {
		t.Fatalf("manifest = %+v", manifest)
	}
}
