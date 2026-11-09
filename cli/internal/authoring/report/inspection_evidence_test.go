package report

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Keep the display projection independent of private package declarations and map iteration.
func TestInspectionMixedComponentEvidence(t *testing.T) {
	if inspection(project.Result{}) != nil {
		t.Fatal("missing package must not create an inspection")
	}
	pkg := &domain.PackageEnvelope{
		SchemaURI: domain.PluginSchemaV1,
		Manifest: domain.PluginManifest{Name: "token-private", Version: "preview-1"},
	}
	pkg.MCP.Servers = map[string]domain.MCPServer{
		"bare": {
			Type: "stdio", StdioRequirement: &domain.StdioRequirement{Command: "node", Kind: domain.ExecutableBare},
			Decoded: map[string]any{"args": []string{"PRIVATE_ARGV"}, "env": map[string]any{"TOKEN": "PRIVATE_ENV"}},
		},
		"bundled": {
			Type: "stdio", StdioRequirement: &domain.StdioRequirement{
				Command: "/PRIVATE_ROOT/PRIVATE_BINARY", Kind: domain.ExecutableBundled, BundledRelativePath: "PRIVATE_PATH",
			},
		},
		"remote": {
			Type: "http", Decoded: map[string]any{"url": "https://PRIVATE_HOST", "headers": map[string]any{"Authorization": "PRIVATE_HEADER"}},
		},
	}
	pkg.MCP.InvalidServer = map[string]domain.Diagnostic{"broken": {}}
	pkg.Skills = map[string]domain.Skill{"safe-skill": {}}
	pkg.Inventory.InvalidSkills = []string{"bad-skill"}
	pkg.Manifest.Extensions = map[string]json.RawMessage{"ext.safe": json.RawMessage(`{"secret":"PRIVATE_EXTENSION"}`)}
	input := project.Result{Facts: conformance.Facts{Package: pkg}}
	got := inspection(input)
	if got == nil || got.Name != "" || got.Version != "preview-1" || got.Schema != domain.PluginSchemaV1 || got.Truncated {
		t.Fatalf("unsafe or incomplete inspection header: %+v", got)
	}
	want := []DisplayComponent{
		{ID: opaque("mcp:bare"), Type: "mcp_stdio", Name: "bare", Executable: "node", ExecutableKind: "bare"},
		{ID: opaque("mcp:bundled"), Type: "mcp_stdio", Name: "bundled", ExecutableKind: "bundled"},
		{ID: opaque("mcp:remote"), Type: "mcp_http", Name: "remote"},
		{ID: opaque("mcp:broken"), Type: "mcp_server", Name: "broken"},
		{ID: opaque("skill:safe-skill"), Type: "skill", Name: "safe-skill"},
		{ID: opaque("skill:bad-skill"), Type: "skill", Name: "bad-skill"},
		{ID: opaque("extension:ext.safe"), Type: "opaque_extension", Namespace: "ext.safe"},
	}
	sort.Slice(want, func(i, j int) bool { return want[i].ID < want[j].ID })
	if !reflect.DeepEqual(got.Components, want) || !reflect.DeepEqual(got, inspection(input)) {
		t.Fatalf("inspection contents or stable ordering changed: %+v", got)
	}
	body, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"token-private", "PRIVATE_ARGV", "PRIVATE_ENV", "PRIVATE_ROOT", "PRIVATE_BINARY", "PRIVATE_PATH", "PRIVATE_HOST", "PRIVATE_HEADER", "PRIVATE_EXTENSION"} {
		if strings.Contains(string(body), secret) {
			t.Fatalf("private package field disclosed in inspection JSON")
		}
	}
}
