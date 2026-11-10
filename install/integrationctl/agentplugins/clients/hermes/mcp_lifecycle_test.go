package hermes

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
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
		Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "remote", Support: domain.SupportUnsupported}},
	}
	if err := New().QualifyPlan(clients.PlanInput{}, &plan); err != nil || plan.Status != domain.PlanUnsupported {
		t.Fatalf("project MCP was admitted: %+v %v", plan, err)
	}
	body, err := os.ReadFile(path)
	if err != nil || string(body) != "model: demo\n" {
		t.Fatalf("user config changed: %s %v", body, err)
	}
}
