package providers

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var openCodeLogicalKeys = []string{"api/server", "api server", "con", `api"server`, `api\server`, "сервер", "api-server"}

func TestOpenCodeLogicalKeysStageKeepsHealthySibling(t *testing.T) {
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range openCodeLogicalKeys {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source")
			manifest, _ := json.Marshal(map[string]any{"$schema": domain.PluginSchemaV1, "name": "probe"})
			writeTestFile(t, filepath.Join(source, "plugin.json"), string(manifest))
			mcp, _ := json.Marshal(map[string]any{"$schema": domain.MCPSchemaV1, "mcpServers": map[string]any{name: map[string]any{"type": "streamable-http", "url": "https://example.test/mcp"}}})
			writeTestFile(t, filepath.Join(source, "mcp.json"), string(mcp))
			writeTestFile(t, filepath.Join(source, "skills", "healthy", "SKILL.md"), "---\nname: healthy\ndescription: Healthy\n---\n")
			envelope, err := (loader.Loader{Registry: registry}).Load(context.Background(), domain.LoadInput{SnapshotRoot: source})
			if err != nil {
				t.Fatal(err)
			}
			anchor := filepath.Join(root, "managed")
			target := filepath.Join(anchor, "clients", "opencode")
			plan := domain.DeliveryPlan{ClientID: domain.ClientOpenCode, Scope: domain.ScopeUser, Status: domain.PlanReady, PackageMode: domain.PackagePrepared, PhysicalArtifactID: "probe", TargetAnchor: anchor, TargetRoot: target, ActivePath: filepath.Join(target, "probe"), NativeRegistryRoot: filepath.Join(root, "native"), Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "healthy", Support: domain.SupportPrepared}, {Kind: domain.ComponentMCPServer, Name: name, Support: domain.SupportPrepared}}}
			delivery, err := (Stager{}).Stage(context.Background(), envelope, plan, "logical-key", domain.CompatibilityHints{})
			if err != nil {
				t.Fatal(err)
			}
			defer (Stager{}).Discard(context.Background(), delivery)
			if _, err := os.Stat(filepath.Join(delivery.StagingPath, "skills", "healthy", "SKILL.md")); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, obj := range delivery.NativeObjects {
				if obj.Kind == "opencode_global_mcp_server" && obj.LogicalName == name {
					found = true
				}
			}
			if !found {
				t.Fatalf("stage lost exact MCP key %q", name)
			}
		})
	}
}
