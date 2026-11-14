package providers

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Qoder's plugin contract is an OpenAI-shaped .mcp.json: a remote
// streamable-http MCP server must reach it with the OpenAI transport spelling,
// its url, and the catalog OpenAI auth hint, mirroring the Codex projection.
func TestStagerProjectsQoderRemoteMCPWithOpenAIAuthHints(t *testing.T) {
	t.Parallel()
	envelope := stagingEnvelope(t)
	plan := stagingPlan(t, domain.ClientQoder, domain.PackageNative)
	delivery, err := testStager(Stager{}).Stage(context.Background(), envelope, plan, "operation-qoder-auth", domain.CompatibilityHints{
		OpenAIMCPAuth: map[string]domain.OpenAIMCPAuthHint{
			"notion": {OAuthResource: "https://mcp.notion.com"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest := readObject(t, filepath.Join(delivery.StagingPath, ".qoder-plugin", "plugin.json"))
	if manifest["mcpServers"] != "./.mcp.json" {
		t.Fatalf("Qoder manifest = %+v", manifest)
	}
	mcp := readObject(t, filepath.Join(delivery.StagingPath, ".mcp.json"))
	servers, ok := mcp["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("Qoder .mcp.json must wrap servers in mcpServers: %+v", mcp)
	}
	notion, ok := servers["notion"].(map[string]any)
	if !ok {
		t.Fatalf("Qoder .mcp.json missing notion server: %+v", servers)
	}
	if notion["type"] != "http" || notion["url"] != "https://mcp.notion.com/mcp" || notion["oauth_resource"] != "https://mcp.notion.com" {
		t.Fatalf("Qoder remote MCP projection = %+v", notion)
	}
}

// A generic catalog compatibility entry for qoder makes the legacy OpenAI auth
// hints non-authoritative (the usecase openAIOAuthApplies gate), so the hint
// must never reach the staged .mcp.json while the server projection stays.
func TestStagerSuppressesQoderOpenAIAuthHintsWithCompatibilityEntry(t *testing.T) {
	t.Parallel()
	envelope := stagingEnvelope(t)
	plan := stagingPlan(t, domain.ClientQoder, domain.PackageNative)
	delivery, err := testStager(Stager{}).Stage(context.Background(), envelope, plan, "operation-qoder-compat", domain.CompatibilityHints{
		Compatibility: map[string]domain.CatalogCompatibility{
			"qoder": {Package: "native", Verification: "tested", Authentication: domain.AuthenticationRequirementNotRequired},
		},
		OpenAIMCPAuth: map[string]domain.OpenAIMCPAuthHint{
			"notion": {OAuthResource: "https://mcp.notion.com"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	mcp := readObject(t, filepath.Join(delivery.StagingPath, ".mcp.json"))
	servers, ok := mcp["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("Qoder .mcp.json must wrap servers in mcpServers: %+v", mcp)
	}
	notion, ok := servers["notion"].(map[string]any)
	if !ok {
		t.Fatalf("Qoder .mcp.json missing notion server: %+v", servers)
	}
	if notion["type"] != "http" || notion["url"] != "https://mcp.notion.com/mcp" {
		t.Fatalf("Qoder remote MCP projection = %+v", notion)
	}
	if _, suppressed := notion["oauth_resource"]; suppressed {
		t.Fatalf("hints.Compatibility[qoder] must suppress the auth hint: %+v", notion)
	}
}
