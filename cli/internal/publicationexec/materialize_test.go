package publicationexec

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/publishschema"
)

func TestRenderLocalCatalogArtifact_CodexRewritesSourceRoot(t *testing.T) {
	graph := pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{
			APIVersion:  "v1",
			Name:        "demo-plugin",
			Version:     "0.1.0",
			Description: "demo",
			Targets:     []string{"codex-package"},
		},
	}
	publication := publishschema.State{
		Codex: &publishschema.CodexMarketplace{
			MarketplaceName:      "local-repo",
			DisplayName:          "Local Repo",
			SourceRoot:           "./",
			Category:             "Productivity",
			InstallationPolicy:   "AVAILABLE",
			AuthenticationPolicy: "ON_INSTALL",
		},
	}
	artifact, err := RenderLocalCatalogArtifact(graph, publication, "codex-package", "./plugins/demo-plugin")
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(artifact.Content, &payload); err != nil {
		t.Fatal(err)
	}
	plugins := payload["plugins"].([]any)
	source := plugins[0].(map[string]any)["source"].(map[string]any)
	if source["path"] != "./plugins/demo-plugin" {
		t.Fatalf("source.path = %+v", source["path"])
	}
}

func TestMergeCatalogArtifact_PreservesOtherPlugins(t *testing.T) {
	existing := []byte(`{
  "name": "local-repo",
  "plugins": [
    {"name": "alpha", "source": {"source": "local", "path": "./plugins/alpha"}, "policy": {"installation": "AVAILABLE", "authentication": "ON_INSTALL"}, "category": "Productivity"}
  ]
}`)
	generated := []byte(`{
  "name": "local-repo",
  "plugins": [
    {"name": "demo", "source": {"source": "local", "path": "./plugins/demo"}, "policy": {"installation": "AVAILABLE", "authentication": "ON_INSTALL"}, "category": "Productivity"}
  ]
}`)
	merged, err := MergeCatalogArtifact("codex-package", existing, generated)
	if err != nil {
		t.Fatal(err)
	}
	text := string(merged)
	for _, want := range []string{`"name": "alpha"`, `"name": "demo"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("merged catalog missing %q:\n%s", want, text)
		}
	}
}

func TestMergeCatalogArtifact_RejectsMarketplaceIdentityMismatch(t *testing.T) {
	existing := []byte(`{"name":"other-repo","plugins":[]}`)
	generated := []byte(`{"name":"local-repo","plugins":[{"name":"demo","source":{"source":"local","path":"./plugins/demo"},"policy":{"installation":"AVAILABLE","authentication":"ON_INSTALL"},"category":"Productivity"}]}`)
	_, err := MergeCatalogArtifact("codex-package", existing, generated)
	if err == nil || !strings.Contains(err.Error(), "existing marketplace artifact sets name differently") {
		t.Fatalf("err = %v", err)
	}
}

func TestMergeCatalogArtifact_ClaudeRejectsOwnerMismatch(t *testing.T) {
	existing := []byte(`{"name":"local-repo","owner":"acme","plugins":[]}`)
	generated := []byte(`{"name":"local-repo","owner":"other","plugins":[{"name":"demo","source":{"source":"local","path":"./plugins/demo"},"policy":{"installation":"AVAILABLE","authentication":"ON_INSTALL"},"category":"Productivity"}]}`)
	_, err := MergeCatalogArtifact("claude", existing, generated)
	if err == nil || !strings.Contains(err.Error(), "existing marketplace artifact sets owner differently") {
		t.Fatalf("err = %v", err)
	}
}

func TestRemoveCatalogArtifact_RemovesNamedPluginAndPreservesOthers(t *testing.T) {
	existing := []byte(`{
  "name": "local-repo",
  "plugins": [
    {"name": "alpha", "source": {"source": "local", "path": "./plugins/alpha"}, "policy": {"installation": "AVAILABLE", "authentication": "ON_INSTALL"}, "category": "Productivity"},
    {"name": "demo", "source": {"source": "local", "path": "./plugins/demo"}, "policy": {"installation": "AVAILABLE", "authentication": "ON_INSTALL"}, "category": "Productivity"}
  ]
}`)
	updated, removed, err := RemoveCatalogArtifact("codex-package", existing, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Fatal("expected removal")
	}
	text := string(updated)
	if strings.Contains(text, `"name": "demo"`) {
		t.Fatalf("demo entry still present:\n%s", text)
	}
	if !strings.Contains(text, `"name": "alpha"`) {
		t.Fatalf("alpha entry missing:\n%s", text)
	}
}

func TestDiagnoseCatalogArtifactDetectsMissingAndDriftedEntry(t *testing.T) {
	existing := []byte(`{
  "name": "local-repo",
  "plugins": [
    {"name": "demo", "source": {"source": "local", "path": "./plugins/other"}, "policy": {"installation": "AVAILABLE", "authentication": "ON_INSTALL"}, "category": "Productivity"}
  ]
}`)
	generated := []byte(`{
  "name": "local-repo",
  "plugins": [
    {"name": "demo", "source": {"source": "local", "path": "./plugins/demo"}, "policy": {"installation": "AVAILABLE", "authentication": "ON_INSTALL"}, "category": "Productivity"}
  ]
}`)
	issues, err := DiagnoseCatalogArtifact("codex-package", existing, generated, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 || issues[0].Code != "drifted_materialized_catalog_entry" {
		t.Fatalf("issues = %+v", issues)
	}

	missing, err := DiagnoseCatalogArtifact("codex-package", []byte(`{"name":"local-repo","plugins":[]}`), generated, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 || missing[0].Code != "missing_materialized_catalog_entry" {
		t.Fatalf("missing issues = %+v", missing)
	}
}
