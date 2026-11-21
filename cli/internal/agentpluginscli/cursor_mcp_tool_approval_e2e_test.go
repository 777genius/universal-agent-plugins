package agentpluginscli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
)

// Public add exercises manifest loader -> planner -> staged native projection ->
// native config ownership -> exact project preference, without vendor auth.
func TestCursorPublisherToolApprovalFullCLI(t *testing.T) {
	fixture, project, source := cursorToolCLIProject(t)
	config := filepath.Join(project, ".cursor", "cli.json")
	native := filepath.Join(project, ".cursor", "mcp.json")
	args := []string{"add", source, "--target", "cursor", "--scope", "project",
		"--project-root", project, "--yes", "--format", "json"}
	var previous []byte
	for _, phase := range []string{"first", "repeat"} {
		out, stderr, err := fixture.execute(false, args...)
		if err != nil {
			t.Fatalf("%s Cursor add failed: %v; stdout=%s stderr=%s", phase, err, out, stderr)
		}
		cursorAssertCLIState(t, fixture, out)
		requireCursorNamedToolPreference(t, config)
		cursorAssertInstalledMCP(t, native)
		current, err := os.ReadFile(config)
		if err != nil {
			t.Fatal(err)
		}
		if previous != nil && !reflect.DeepEqual(previous, current) {
			t.Fatal("repeat Cursor add rewrote existing native permission bytes")
		}
		previous = current
	}
	out, stderr, err := fixture.execute(false, "remove", "demo", "--target", "cursor",
		"--scope", "project", "--project-root", project, "--yes", "--format", "json")
	if err != nil {
		t.Fatalf("Cursor remove failed: %v; stdout=%s stderr=%s", err, out, stderr)
	}
	requireCursorNamedToolPreference(t, config)
	cursorAssertRemovedMCP(t, native)
}

func cursorToolCLIProject(t *testing.T) (cliFixture, string, string) {
	t.Helper()
	client := fixtureClient(t, domain.ClientCursor)
	fixture := newCLIFixture(t, []domain.DetectedClient{client})
	project, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fixture.app.Scope, fixture.app.ProjectRoot = domain.ScopeProject, project
	fixture.app.Lifecycle.ProjectRoot = project
	fixture.app.Lifecycle.Activator = providerstest.NewActivator(providers.Activator{})
	source := writeCLIPlugin(t)
	writeCLIMCP(t, source)
	path := filepath.Join(source, "plugin.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(body, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest["extensions"] = map[string]any{
		"com.universal-agent-plugins.cursor-mcp-approval": map[string]any{
			"version": 1, "mcp_auto_approve": map[string]any{"demo": []string{"inspect_runtime"}},
		},
	}
	updated, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, updated, 0600); err != nil {
		t.Fatal(err)
	}
	return fixture, project, source
}

func cursorAssertCLIState(t *testing.T, fixture cliFixture, output string) {
	t.Helper()
	state, err := fixture.store.Load()
	if err != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 ||
		state.Installations[0].Clients[0].Activation != domain.ActivationPrepared {
		t.Fatalf("Cursor project claimed native readiness or lost state: %+v (%v), output=%s", state, err, output)
	}
}

func cursorAssertInstalledMCP(t *testing.T, path string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatal(err)
	}
	servers, ok := document["mcpServers"].(map[string]any)
	if !ok || servers["demo"] == nil {
		t.Fatalf("Cursor project lost selected MCP server: %s", body)
	}
}

func cursorAssertRemovedMCP(t *testing.T, path string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return // removing the last owned MCP member may remove the whole document
	}
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatal(err)
	}
	if servers, ok := document["mcpServers"].(map[string]any); ok && servers["demo"] != nil {
		t.Fatalf("Cursor remove retained the owned MCP server: %s", body)
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
	policy, ok := document["permissions"].(map[string]any)
	if !ok || !reflect.DeepEqual(policy["allow"], []any{"Mcp(demo:inspect_runtime)"}) {
		t.Fatalf("Cursor exact tool approval missing or broadened: %s", body)
	}
	if _, exists := document["mcpAllowlist"]; exists {
		t.Fatal("Cursor CLI grant unexpectedly modified Editor permissions")
	}
}
