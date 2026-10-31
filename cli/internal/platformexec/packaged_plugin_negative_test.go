package platformexec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

// packagedNegativeLane identifies the Grok/Kimi packaged lanes under their
// managed manifest paths for the negative import/validate contracts.
type packagedNegativeLane struct {
	id          string
	adapter     func() packagedPluginAdapter
	manifestRel string
}

func packagedNegativeLanes() []packagedNegativeLane {
	return []packagedNegativeLane{
		{id: "grok", adapter: newGrokPluginAdapter, manifestRel: filepath.Join(".grok-plugin", "plugin.json")},
		{id: "kimi", adapter: newKimiPluginAdapter, manifestRel: filepath.Join(".kimi-plugin", "plugin.json")},
	}
}

func writePackagedNegativeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func packagedNegativeGraph(target string) pluginmodel.PackageGraph {
	return pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: target + "-demo", Version: "0.1.0", Description: target + " demo", Targets: []string{target}},
		Portable: pluginmodel.PortableComponents{Items: map[string][]string{}},
	}
}

// A managed manifest with malformed JSON must fail import and surface an
// "invalid JSON" diagnostic on validate instead of being treated as absent.
func TestPackagedPluginMalformedManagedManifestJSONFailsImportAndValidate(t *testing.T) {
	t.Parallel()
	for _, lane := range packagedNegativeLanes() {
		t.Run(lane.id, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writePackagedNegativeFile(t, root, lane.manifestRel, `{"name": "demo"`)
			_, err := lane.adapter().Import(root, ImportSeed{
				Manifest: pluginmodel.Manifest{Name: lane.id + "-demo", Targets: []string{lane.id}},
			})
			if err == nil || !strings.Contains(err.Error(), "invalid JSON") {
				t.Fatalf("import malformed managed manifest err = %v", err)
			}
			diagnostics, err := lane.adapter().Validate(root, packagedNegativeGraph(lane.id), pluginmodel.NewTargetState(lane.id))
			if err != nil {
				t.Fatal(err)
			}
			if text := diagnosticsText(diagnostics); !strings.Contains(text, "invalid JSON") {
				t.Fatalf("validate malformed managed manifest diagnostics = %s", text)
			}
		})
	}
}

// mcpServers must be a string ref or an inline object; a scalar like 5 is a
// contract violation, not a silent skip.
func TestPackagedPluginRejectsNonStringNonObjectMCPServers(t *testing.T) {
	t.Parallel()
	for _, lane := range packagedNegativeLanes() {
		t.Run(lane.id, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writePackagedNegativeFile(t, root, lane.manifestRel, `{"name":"demo","mcpServers":5}`)
			_, err := lane.adapter().Import(root, ImportSeed{
				Manifest: pluginmodel.Manifest{Name: lane.id + "-demo", Targets: []string{lane.id}},
			})
			if err == nil || !strings.Contains(err.Error(), "must be a string ref or object") {
				t.Fatalf("import scalar mcpServers err = %v", err)
			}
		})
	}
}

// Phase 1 rejects authored plugin/targets/<grok|kimi>/... trees: the packaged
// lanes only accept portable skills and MCP sources.
func TestPackagedPluginRefineDiscoveryRejectsAuthoredTargetDir(t *testing.T) {
	t.Parallel()
	for _, lane := range packagedNegativeLanes() {
		t.Run(lane.id, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writePackagedNegativeFile(t, root, filepath.Join("plugin", "targets", lane.id, "rules", "extra.mdc"), "authored\n")
			state := pluginmodel.NewTargetState(lane.id)
			err := lane.adapter().RefineDiscovery(root, &state)
			if err == nil || !strings.Contains(err.Error(), "does not support") {
				t.Fatalf("authored plugin/targets/%s err = %v", lane.id, err)
			}
			if !strings.Contains(err.Error(), "plugin/targets/"+lane.id) {
				t.Fatalf("refusal must name the authored tree: %v", err)
			}
		})
	}
}
