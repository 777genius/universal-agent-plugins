package report

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// TestComponentEvidenceStatusAndRedaction pins the old mixed-host projection
// while keeping all observations inert and private input bytes out of JSON.
func TestComponentEvidenceStatusAndRedaction(t *testing.T) {
	t.Parallel()
	const privateValue = "super-private-runtime-token"
	pkg := &domain.PackageEnvelope{
		MCP: domain.MCPComponent{
			Servers: map[string]domain.MCPServer{
				"local-private": {
					Type: "stdio",
					StdioRequirement: &domain.StdioRequirement{
						Kind: domain.ExecutableBundled, Command: privateValue,
						UsesPluginRoot: true, UsesPluginData: true,
					},
					Decoded: map[string]any{"env": map[string]any{"TOKEN": privateValue}},
				},
				"remote-private": {
					Type:    "http",
					Decoded: map[string]any{"headers": map[string]any{"Authorization": privateValue}},
				},
				"unknown-private": {Type: "sse"},
			},
			InvalidServer: map[string]domain.Diagnostic{"invalid-private": {}},
		},
		Skills: map[string]domain.Skill{
			"compat-private":   {Compatibility: privateValue, AllowedTools: privateValue},
			"ordinary-private": {},
		},
		Inventory: domain.ComponentInventory{InvalidSkills: []string{"invalid-skill-private"}},
		Manifest: domain.PluginManifest{
			Extensions: map[string]json.RawMessage{"extension-private": json.RawMessage(`{"token":"super-private-runtime-token"}`)},
		},
	}
	facts := conformance.Facts{
		Package: pkg,
		Findings: []conformance.Finding{
			{Code: "normative_error", Boundary: domain.BoundaryMCPServer, Item: opaque("remote-private"), Layer: conformance.Normative},
			{Code: "host_uncertainty", Boundary: domain.BoundaryMCPServer, Item: opaque("local-private"), Layer: conformance.HostSafety},
		},
	}
	projected := func(f conformance.Facts) Report {
		r := New("validate", "revision")
		r.components(project.Result{Facts: f})
		r.finish()
		return r
	}
	r := projected(facts)
	if len(r.Components) != 8 {
		t.Fatalf("component count changed: %d", len(r.Components))
	}
	want := map[string]Component{}
	for _, expected := range []Component{
		{
			ID: opaque("mcp:local-private"), Type: "mcp_stdio", Status: NotEvaluated,
			Requirements: []string{"executable_unresolved", "executable_bundled", "plugin_root", "plugin_data", "environment_values_redacted"},
		},
		{
			ID: opaque("mcp:remote-private"), Type: "mcp_http", Status: Fail,
			Requirements: []string{"remote_endpoint_uncontacted", "header_values_redacted"},
		},
		{
			ID: opaque("mcp:unknown-private"), Type: "mcp_sse", Status: Pass,
			Requirements: []string{"remote_endpoint_uncontacted"},
		},
		{
			ID: opaque("mcp:invalid-private"), Type: "mcp_server", Status: Fail, Requirements: []string{},
		},
		{
			ID: opaque("skill:compat-private"), Type: "skill", Status: Pass,
			Requirements: []string{"declared_compatibility_unresolved", "declared_tools_unresolved"},
		},
		{
			ID: opaque("skill:ordinary-private"), Type: "skill", Status: Pass, Requirements: []string{},
		},
		{
			ID: opaque("skill:invalid-skill-private"), Type: "skill", Status: Fail, Requirements: []string{},
		},
		{
			ID: opaque("extension:extension-private"), Type: "opaque_extension", Status: NotEvaluated, Requirements: []string{},
		},
	} {
		want[expected.ID] = expected
	}
	for i, c := range r.Components {
		expected, ok := want[c.ID]
		if !ok || c.Type != expected.Type || c.Status != expected.Status ||
			!slices.Equal(c.Requirements, expected.Requirements) {
			t.Fatalf("component %d mismatch: %+v expected %+v", i, c, expected)
		}
		if i > 0 && r.Components[i-1].ID >= c.ID {
			t.Fatal("component order is not stable")
		}
	}
	wire, err := json.Marshal(r.Components)
	if err != nil || strings.Contains(string(wire), privateValue) ||
		strings.Contains(string(wire), "local-private") ||
		strings.Contains(string(wire), "extension-private") {
		t.Fatalf("private evidence leaked into public components: %v", err)
	}
	for range 8 {
		next := projected(facts)
		again, err := json.Marshal(next.Components)
		if err != nil || string(again) != string(wire) {
			t.Fatal("component map iteration changed stable wire order")
		}
	}
	facts.Findings = append(facts.Findings, conformance.Finding{
		Code: "diagnostics_truncated", Boundary: domain.BoundaryPlugin, Layer: conformance.HostSafety,
	})
	truncated := projected(facts)
	for _, c := range truncated.Components {
		if c.ID == opaque("mcp:unknown-private") && c.Status != NotEvaluated {
			t.Fatalf("truncated evidence became a pass: %+v", c)
		}
		if c.ID == opaque("mcp:remote-private") && c.Status != Fail {
			t.Fatalf("normative failure softened by truncation: %+v", c)
		}
	}
}
