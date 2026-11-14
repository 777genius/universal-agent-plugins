package platformexec

import (
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestBuildOpenCodeBaseConfigIncludesPluginSection(t *testing.T) {
	t.Parallel()

	doc, err := buildOpenCodeBaseConfig(pluginmodel.PackageGraph{}, opencodePackageMeta{
		Plugins: []opencodePluginRef{{Name: "demo"}},
	})
	if err != nil {
		t.Fatalf("buildOpenCodeBaseConfig: %v", err)
	}
	if doc["$schema"] != "https://opencode.ai/config.json" {
		t.Fatalf("doc = %#v", doc)
	}
	if _, ok := doc["plugin"]; !ok {
		t.Fatalf("doc = %#v", doc)
	}
}

func TestAppendOpenCodeInstructionsTrimsEntries(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	rel := filepath.Join("plugin", "targets", "opencode", "instructions.yaml")
	writeOpenCodeImportFile(t, filepath.Join(root, rel), "- first.md\n- \" second.md \"\n")

	state := pluginmodel.NewTargetState("opencode")
	state.SetDoc("instructions_config", rel)
	doc := map[string]any{}
	if err := appendOpenCodeInstructions(root, state, doc); err != nil {
		t.Fatalf("appendOpenCodeInstructions: %v", err)
	}
	got, ok := doc["instructions"].([]string)
	if !ok {
		t.Fatalf("doc = %#v", doc)
	}
	if len(got) != 2 || got[0] != "first.md" || got[1] != "second.md" {
		t.Fatalf("instructions = %#v", got)
	}
}
