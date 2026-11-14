package publicationexec

import (
	"encoding/json"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/publishschema"
)

func TestRender_CodexMarketplaceArtifact(t *testing.T) {
	graph := pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{
			APIVersion:  "v1",
			Name:        "demo-plugin",
			Version:     "0.1.0",
			Description: "demo plugin",
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

	artifacts, err := Generate(graph, publication, []string{"codex-package"})
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 || artifacts[0].RelPath != CodexMarketplaceArtifactPath {
		t.Fatalf("artifacts = %+v", artifacts)
	}
	var payload map[string]any
	if err := json.Unmarshal(artifacts[0].Content, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["name"] != "local-repo" {
		t.Fatalf("payload.name = %+v", payload["name"])
	}
	iface, ok := payload["interface"].(map[string]any)
	if !ok || iface["displayName"] != "Local Repo" {
		t.Fatalf("payload.interface = %+v", payload["interface"])
	}
	plugins, ok := payload["plugins"].([]any)
	if !ok || len(plugins) != 1 {
		t.Fatalf("payload.plugins = %+v", payload["plugins"])
	}
	plugin, ok := plugins[0].(map[string]any)
	if !ok {
		t.Fatalf("payload.plugins[0] = %+v", plugins[0])
	}
	if plugin["name"] != "demo-plugin" || plugin["category"] != "Productivity" {
		t.Fatalf("plugin entry = %+v", plugin)
	}
	source, ok := plugin["source"].(map[string]any)
	if !ok || source["source"] != "local" || source["path"] != "./" {
		t.Fatalf("plugin source = %+v", plugin["source"])
	}
	policy, ok := plugin["policy"].(map[string]any)
	if !ok || policy["installation"] != "AVAILABLE" || policy["authentication"] != "ON_INSTALL" {
		t.Fatalf("plugin policy = %+v", plugin["policy"])
	}
}

func TestManagedPaths_CodexMarketplaceFollowsSelectedTargets(t *testing.T) {
	paths := ManagedPaths(publishschema.State{}, []string{"codex-package"})
	if len(paths) != 1 || paths[0] != CodexMarketplaceArtifactPath {
		t.Fatalf("paths = %+v", paths)
	}
	if other := ManagedPaths(publishschema.State{}, []string{"gemini"}); len(other) != 0 {
		t.Fatalf("managed paths for non-codex target = %+v", other)
	}
}

func TestRender_ClaudeMarketplaceArtifact(t *testing.T) {
	graph := pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{
			APIVersion:  "v1",
			Name:        "demo-plugin",
			Version:     "0.1.0",
			Description: "demo plugin",
			Targets:     []string{"claude"},
		},
	}
	publication := publishschema.State{
		Claude: &publishschema.ClaudeMarketplace{
			MarketplaceName: "acme-tools",
			OwnerName:       "ACME Team",
			SourceRoot:      "./",
		},
	}

	artifacts, err := Generate(graph, publication, []string{"claude"})
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 || artifacts[0].RelPath != ClaudeMarketplaceArtifactPath {
		t.Fatalf("artifacts = %+v", artifacts)
	}
	var payload map[string]any
	if err := json.Unmarshal(artifacts[0].Content, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["name"] != "acme-tools" {
		t.Fatalf("payload.name = %+v", payload["name"])
	}
	owner, ok := payload["owner"].(map[string]any)
	if !ok || owner["name"] != "ACME Team" {
		t.Fatalf("payload.owner = %+v", payload["owner"])
	}
	plugins, ok := payload["plugins"].([]any)
	if !ok || len(plugins) != 1 {
		t.Fatalf("payload.plugins = %+v", payload["plugins"])
	}
	plugin, ok := plugins[0].(map[string]any)
	if !ok {
		t.Fatalf("payload.plugins[0] = %+v", plugins[0])
	}
	if plugin["name"] != "demo-plugin" || plugin["source"] != "./" || plugin["description"] != "demo plugin" || plugin["version"] != "0.1.0" {
		t.Fatalf("plugin entry = %+v", plugin)
	}
}

func TestManagedPaths_ClaudeMarketplaceFollowsSelectedTargets(t *testing.T) {
	paths := ManagedPaths(publishschema.State{}, []string{"claude"})
	if len(paths) != 1 || paths[0] != ClaudeMarketplaceArtifactPath {
		t.Fatalf("paths = %+v", paths)
	}
	if other := ManagedPaths(publishschema.State{}, []string{"cursor"}); len(other) != 0 {
		t.Fatalf("managed paths for non-claude target = %+v", other)
	}
}
