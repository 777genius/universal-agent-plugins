package hermes

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
)

func TestHermesNativeUserMCPAddUpdateRemove(t *testing.T) {
	root := t.TempDir()
	profile := filepath.Join(root, ".hermes")
	if err := os.MkdirAll(profile, 0o700); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(profile, "config.yaml")
	source := "# Provider-owned config\nmodel: demo\nmcp_servers:\n  foreign:\n    command: echo\n"
	if err := os.WriteFile(config, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(root, "managed", "plugin")
	if err := os.MkdirAll(active, 0o700); err != nil {
		t.Fatal(err)
	}
	plan := domain.DeliveryPlan{
		ClientID: domain.ClientHermes, Scope: domain.ScopeUser,
		TargetContext:      domain.TargetContext{Scope: domain.ScopeUser, UserConfigRoot: profile},
		NativeRegistryRoot: profile, ActivePath: active,
		Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "remote", Support: domain.SupportNative}},
	}
	env := domain.PackageEnvelope{
		Manifest: domain.PluginManifest{Name: "test-hermes", Version: "1.0.0"},
		MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{"remote": {
			Name: "remote", Type: "streamable-http",
			Decoded: map[string]any{"type": "streamable-http", "url": "https://example.test/mcp"},
		}}},
	}
	adapter := New()
	objects, err := adapter.Project(context.Background(), clients.ProjectionInput{
		StagingPath: active, Plan: plan, Envelope: env,
	})
	if err != nil || len(objects) != 1 || objects[0].Kind != userMCPKind {
		t.Fatalf("user MCP projection did not produce ownership: %+v %v", objects, err)
	}
	layout, err := components(plan, active)
	if err != nil {
		t.Fatal(err)
	}
	servers, err := hermesDesiredServers(active, objects)
	if err != nil {
		t.Fatal(err)
	}
	run := func(previous, desired []domain.NativeObjectOwnership) error {
		return (shared.ProjectComponentMutation{
			Layout: layout, ActivePath: active, Previous: previous, Desired: desired,
			Servers: func() map[string]nativeconfig.Server {
				if len(desired) == 0 {
					return nil
				}
				return servers
			}(), Kernel: nativeconfig.New(),
		}).Apply(context.Background())
	}
	if err := run(nil, objects); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := layout.Verify(objects, false, nativeconfig.New()); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := run(objects, objects); err != nil {
		t.Fatalf("idempotent update: %v", err)
	}
	if err := run(objects, nil); err != nil {
		t.Fatalf("remove: %v", err)
	}
	body, err := os.ReadFile(config)
	if err != nil || !strings.Contains(string(body), "model: demo") ||
		!strings.Contains(string(body), "foreign:") || strings.Contains(string(body), "remote:") {
		t.Fatalf("Hermes config modified foreign values: %s %v", body, err)
	}
}

func TestHermesNativeProjectMCPRefusesBeforeGlobalWrite(t *testing.T) {
	project := t.TempDir()
	profile := filepath.Join(t.TempDir(), ".hermes")
	if err := os.MkdirAll(profile, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(profile, "config.yaml")
	if err := os.WriteFile(path, []byte("model: demo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan := domain.DeliveryPlan{
		ClientID: domain.ClientHermes, Scope: domain.ScopeProject,
		TargetContext: domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: project,
			UserConfigRoot: profile},
		NativeRegistryRoot: filepath.Join(project, ".hermes"), Status: domain.PlanReady,
		Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "remote", Support: domain.SupportPrepared}},
	}
	source := filepath.Join(t.TempDir(), "hermes")
	if err := os.WriteFile(source, []byte("inert source-installed client"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := New().QualifyPlan(clients.PlanInput{Client: domain.DetectedClient{ExecutablePath: source}}, &plan); err != nil || plan.Status != domain.PlanUnsupported || plan.Components[0].Support != domain.SupportUnsupported {
		t.Fatalf("project MCP was admitted: %+v %v", plan, err)
	}
	body, err := os.ReadFile(path)
	if err != nil || string(body) != "model: demo\n" {
		t.Fatalf("user config changed: %s %v", body, err)
	}
}

type hermesInertLauncher struct{}

func (hermesInertLauncher) Deliver(root string) error {
	path := filepath.Join(root, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte("inert; never executed"), 0o700)
}

func TestHermesProjectMCPUsesPersistedEncodedSelection(t *testing.T) {
	project := t.TempDir()
	active := filepath.Join(project, ".uap", "managed", "clients", "hermes", "fixture")
	if err := os.MkdirAll(active, 0o700); err != nil {
		t.Fatal(err)
	}
	plan := domain.DeliveryPlan{ClientID: domain.ClientHermes, Scope: domain.ScopeProject,
		NativeRegistryRoot: filepath.Join(project, ".hermes"), ActivePath: active,
		TargetContext: domain.TargetContext{ProjectRoot: project}, Components: []domain.ComponentDecision{
			{Kind: domain.ComponentMCPServer, Name: "remote", Support: domain.SupportPrepared},
			{Kind: domain.ComponentMCPServer, Name: "stdio", Support: domain.SupportPrepared}}}
	envelope := domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{
		"remote": {Name: "remote", Type: "streamable-http", Decoded: map[string]any{"type": "streamable-http", "url": "https://example.test/mcp"}},
		"stdio": {Name: "stdio", Type: "stdio", Decoded: map[string]any{"type": "stdio", "command": "go",
			"args": []any{"run", "${PLUGIN_ROOT}/probe.go", "${HOME}", "${workspaceFolder}"}}},
	}}}
	objects, err := New().Project(t.Context(), clients.ProjectionInput{StagingPath: active, Plan: plan,
		Envelope: envelope, PluginDataPath: filepath.Join(project, ".uap", "plugin-data", "fixture"), Launcher: hermesInertLauncher{}})
	if err != nil || len(objects) != 3 {
		t.Fatal("Project projection failed", objects, err)
	}
	if _, err := New().componentObjects(plan, objects); err != nil {
		t.Fatal("projection lost its durable root/entry authority", err)
	}
	servers, err := hermesDesiredServers(active, objects)
	if err != nil {
		t.Fatal(err)
	}
	stdio := servers["stdio"]
	decoded, err := managedstdio.DecodeCommandArguments(stdio.Args[5], stdio.Args[6:])
	if err != nil || !slices.Equal(decoded, []string{"go", "run", active + "/probe.go", "${HOME}", "${workspaceFolder}"}) ||
		stdio.Command != filepath.Join(active, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName) {
		t.Fatal("opaque child arguments or managed launcher were not preserved", decoded, err)
	}
	envelope.MCP.Servers["stdio"].Decoded["args"] = []any{"source-changed"}
	readout, err := New().ProjectActiveNative(t.Context(), active, envelope, plan, "")
	if err != nil || !reflect.DeepEqual(readout, objects) {
		t.Fatal("active repair read authored recipe instead of persisted selection", readout, err)
	}
	selection := filepath.Join(active, "mcp.json")
	if err := os.WriteFile(selection, []byte(`{"mcpServers":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New().ProjectActiveNative(t.Context(), active, envelope, plan, ""); err == nil {
		t.Fatal("missing persisted entry fell back to authored recipe")
	}
	if err := os.Remove(selection); err != nil {
		t.Fatal(err)
	}
	if _, err := New().ProjectActiveNative(t.Context(), active, envelope, plan, ""); err == nil {
		t.Fatal("absent persisted file fell back to authored recipe")
	}
}

func TestHermesProjectRefusesNativeInterpolationOutsideEncodedArguments(t *testing.T) {
	for _, server := range []nativeconfig.Server{
		{Command: "${HOME}"}, {CWD: "${workspaceFolder}"}, {URL: "https://example.test/${TOKEN}"},
		{Env: map[string]string{"OPAQUE": "${env:SECRET}"}}, {Headers: map[string]string{"Authorization": "${TOKEN}"}},
	} {
		if err := hermesLiteralServer(server); err == nil {
			t.Fatal("mutable native interpolation was admitted")
		}
	}
	if err := hermesLiteralServer(nativeconfig.Server{Env: map[string]string{"LITERAL": "$HOME ${} ${unfinished"}}); err != nil {
		t.Fatal("native regex does not expand these literal values", err)
	}
}
