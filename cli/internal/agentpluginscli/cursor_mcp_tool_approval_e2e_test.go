package agentpluginscli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
)

// Exercises the public add command: manifest loader -> planner -> staged
// Cursor projection -> owned native config -> exact project tool preference.
// Native account login, Editor workspace trust and model calls are not faked.
func TestCursorPublisherToolApprovalFullCLI(t *testing.T) {
	client := fixtureClient(t, domain.ClientCursor)
	fixture := newCLIFixture(t, []domain.DetectedClient{client})
	project, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fixture.app.Scope, fixture.app.ProjectRoot = domain.ScopeProject, project
	fixture.app.Lifecycle.Activator = providerstest.NewActivator(providers.Activator{})
	source := writeCLIPlugin(t)
	writeCLIMCP(t, source)
	manifestPath := filepath.Join(source, "plugin.json")
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(body, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest["extensions"] = map[string]any{
		"com.universal-agent-plugins.cursor-mcp-approval": map[string]any{
			"version": 1,
			"mcp_auto_approve": map[string]any{"demo": []string{"inspect_runtime"}},
		},
	}
	updated, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, updated, 0600); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(project, ".cursor", "cli.json")
	nativePath := filepath.Join(project, ".cursor", "mcp.json")
	argv := []string{"add", source, "--target", "cursor", "--scope", "project",
		"--project-root", project, "--yes", "--format", "json"}
	for _, label := range []string{"first", "repeat"} {
		out, stderr, err := fixture.execute(false, argv...)
		if err != nil {
			t.Fatalf("%s Cursor project add: %v; stdout=%s stderr=%s", label, err, out, stderr)
		}
		if !strings.Contains(out, "prepared") {
			t.Fatalf("%s claimed native editor/session readiness without evidence: %s", label, out)
		}
		requireCursorNamedToolPreference(t, configPath)
		var mcp map[string]any
		body, err := os.ReadFile(nativePath)
		if err != nil || json.Unmarshal(body, &mcp) != nil {
			t.Fatalf("%s native MCP configuration missing: %v", label, err)
		}
		servers, ok := mcp["mcpServers"].(map[string]any)
		if !ok || servers["demo"] == nil {
			t.Fatalf("%s selected MCP server missing from Cursor: %s", label, body)
		}
	}

	// Removal retracts the exact owned server, not durable user consent or
	// any foreign preference in the Cursor CLI project policy.
	out, stderr, err := fixture.execute(false, "remove", "demo", "--target", "cursor",
		"--scope", "project", "--project-root", project, "--yes", "--format", "json")
	if err != nil {
		t.Fatalf("Cursor owned project remove: %v; stdout=%s stderr=%s", err, out, stderr)
	}
	requireCursorNamedToolPreference(t, configPath)
	after, err := os.ReadFile(nativePath)
	if err != nil {
		t.Fatal(err)
	}
	var mcp map[string]any
	if err := json.Unmarshal(after, &mcp); err != nil {
		t.Fatal(err)
	}
	if entries, ok := mcp["mcpServers"].(map[string]any); ok && entries["demo"] != nil {
		t.Fatalf("removed Cursor MCP server was left active: %s", after)
	}
}

func requireCursorNamedToolPreference(t *testing.T, path string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatal(err)
	}
	permissions, ok := document["permissions"].(map[string]any)
	if !ok || !reflect.DeepEqual(permissions["allow"], []any{"Mcp(demo:inspect_runtime)"}) {
		t.Fatalf("Cursor exact MCP tool approval absent or too broad: %s", body)
	}
	if _, exists := document["mcpAllowlist"]; exists {
		t.Fatal("Cursor CLI project grant unexpectedly rewrote Editor permissions")
	}
}
