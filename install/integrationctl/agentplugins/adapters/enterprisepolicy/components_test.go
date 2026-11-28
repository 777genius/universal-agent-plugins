package enterprisepolicy

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestSourcePreflightDefersOnlyUnknownFinalFacts(t *testing.T) {
	t.Parallel()
	snapshot := testSnapshot(t, "schema_version: 2\ninstall:\n  allowed_scopes: [project]\n  allowed_targets: [codex]\ncomponents:\n  denied: [mcp]\n")
	req := domain.InstallPolicyRequest{}
	assertPolicyResult(t, snapshot, req, false)
	req.Envelope = &domain.PackageEnvelope{}
	assertPolicyResult(t, snapshot, req, true)
	req.Scope, req.Targets = domain.ScopeProject, []domain.ClientID{domain.ClientCodex}
	assertPolicyResult(t, snapshot, req, false)
	req.Envelope.MCP.Present = true
	assertPolicyResult(t, snapshot, req, true)
	req.Envelope.MCP.Present = false
	req.Targets = append(req.Targets, domain.ClientClaude)
	assertPolicyResult(t, snapshot, req, true)
}

func TestDeniedComponentsUseWholePackageInventory(t *testing.T) {
	t.Parallel()
	for name, envelope := range map[string]domain.PackageEnvelope{
		"mcp":        {Inventory: domain.ComponentInventory{MCPPresent: true}},
		"skills":     {Inventory: domain.ComponentInventory{InvalidSkillsRoot: true}},
		"commands":   {InvalidCommands: []string{"broken"}},
		"apps":       {App: domain.AppComponent{Declared: true}},
		"extensions": {Manifest: domain.PluginManifest{RawExtensions: json.RawMessage(`{"future.vendor":{"enabled":true}}`)}},
	} {
		t.Run(name, func(t *testing.T) {
			snapshot := testSnapshot(t, "schema_version: 2\ncomponents:\n  denied: ["+name+"]\n")
			assertPolicyResult(t, snapshot, domain.InstallPolicyRequest{Envelope: &envelope}, true)
		})
	}
}

func TestMCPHostRulesDoNotPretendStdioHasAnEndpoint(t *testing.T) {
	t.Parallel()
	snapshot := testSnapshot(t, "schema_version: 2\nmcp:\n  allowed_transports: [stdio, streamable-http]\n  allow_hosts: [mcp.corp.test]\n  deny_hosts: [bad.test]\n")
	envelope := domain.PackageEnvelope{MCP: domain.MCPComponent{Present: true, Enabled: true, Servers: map[string]domain.MCPServer{
		"local":  {Type: "stdio", Decoded: map[string]any{"command": "local-tool"}},
		"remote": {Type: "streamable-http", Decoded: map[string]any{"url": "https://mcp.corp.test/mcp?token=secret-not-in-errors"}},
	}}}
	req := domain.InstallPolicyRequest{Envelope: &envelope}
	assertPolicyResult(t, snapshot, req, false)
	for _, raw := range []string{"https://mcp.corp.test.evil.test/mcp", "https://bad.test/mcp", "https://${MCP_HOST}/mcp", "https://secret-not-in-errors@mcp.corp.test/mcp"} {
		envelope.MCP.Servers["remote"] = domain.MCPServer{Type: "streamable-http", Decoded: map[string]any{"url": raw}}
		_, err := snapshot.Check(req)
		var denied *domain.InstallPolicyError
		if !errors.As(err, &denied) || strings.Contains(err.Error(), "secret-not-in-errors") || strings.Contains(err.Error(), raw) {
			t.Fatalf("unsafe or missing endpoint denial: %v", err)
		}
	}
	envelope.MCP.Servers["remote"] = domain.MCPServer{Type: "sse", Decoded: map[string]any{"url": "https://mcp.corp.test/mcp"}}
	assertPolicyResult(t, snapshot, req, true)
}
