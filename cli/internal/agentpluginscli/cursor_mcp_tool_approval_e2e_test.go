package agentpluginscli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/plannertest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
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
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(root, "project")
	if err := os.MkdirAll(project, 0700); err != nil {
		t.Fatal(err)
	}
	fixture := newCLIFixtureAt(t, []domain.DetectedClient{client}, root)
	configureCursorProjectCLI(t, &fixture, project)
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

func configureCursorProjectCLI(t *testing.T, f *cliFixture, project string) {
	t.Helper()
	// Match the existing production Project composition: all durable state,
	// ownership, data and transaction journals belong to this exact .uap.
	stateRoot := filepath.Join(project, ".uap")
	f.store = statev2.Store{Path: filepath.Join(stateRoot, "state-v2.json"), ProjectRoot: project}
	f.app.Scope, f.app.ProjectRoot, f.app.ManagedRoot, f.app.StateStore =
		domain.ScopeProject, project, filepath.Join(stateRoot, "managed"), f.store
	f.app.EnvironmentLockPath = filepath.Join(stateRoot, "environment.lock")
	f.app.EnvironmentStoreRoot, f.app.DependenciesRoot = filepath.Join(stateRoot, "store"), project
	scoped := plannertest.NewPlanner(clientplanner.Planner{
		ProjectRoot: project, ManagedRoot: f.app.ManagedRoot,
	})
	f.app.Planner, f.app.Targets = scoped, scoped
	f.app.Lifecycle.ProjectRoot, f.app.Lifecycle.StateStore = project, f.store
	f.app.Lifecycle.Planner, f.app.Lifecycle.Targets = scoped, scoped
	runner := &refusingClientRunner{}
	f.app.Lifecycle.Activator = providerstest.NewActivator(providers.Activator{Runner: runner})
	f.app.Lifecycle.Lock = processlock.Lock{Path: filepath.Join(stateRoot, "mutation.lock")}
	f.app.Lifecycle.Kernel = transaction.Kernel{
		StateStore: f.store, Directory: dirswap.Manager{JournalDir: filepath.Join(stateRoot, "operations")},
	}
	f.app.Lifecycle.PluginData = providers.PluginDataManager{Base: filepath.Join(stateRoot, "plugin-data")}
	var err error
	f.app.Installer, err = installer.New(installer.Config{
		ProjectRoot: project, StateRoot: stateRoot, Registry: f.app.ClientRegistry,
		Runner: runner, TrustedLocalPackages: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	f.app.Lifecycle.CheckNamespace = f.app.Installer.CheckProjectNamespace
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
