package antigravity

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestAdapterIdentityAndScopes(t *testing.T) {
	t.Parallel()
	adapter := New()
	if adapter.ID() != domain.ClientAntigravity {
		t.Fatal("Antigravity target id mismatch")
	}
	root := t.TempDir()
	config := filepath.Join(root, ".gemini", "config")
	detected := adapter.DetectSurfaces(clients.NewHost(clients.HostProbes{
		HomeDir: root, Environment: map[string]string{},
		Lstat: os.Lstat, EvalSymlinks: filepath.EvalSymlinks,
		LookPath: func(name string) (string, error) {
			if name != "agy" { t.Fatalf("unrecognized probe %q", name) }
			return filepath.Join(root, "agy"), nil
		},
	}))
	if detected.Err != nil || detected.ConfigRoot != config {
		t.Fatalf("Antigravity profile discovery: %+v", detected)
	}
	userAnchor, userRoot, err := adapter.TargetRoot(domain.DetectedClient{ConfigRoot: config}, domain.PackageNative, "")
	if err != nil || userRoot != filepath.Join(userAnchor, "plugins") {
		t.Fatalf("user target: %q %q %v", userAnchor, userRoot, err)
	}
	projRoot := filepath.Join(root, "demo")
	project := domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: projRoot}
	anchor, nativeRoot, err := adapter.ProjectTargetRoot(domain.DetectedClient{}, domain.PackageNative, filepath.Join(projRoot, ".uap"), project)
	if err != nil || anchor != filepath.Join(projRoot, ".agents") ||
		nativeRoot != filepath.Join(anchor, "plugins") {
		t.Fatalf("project target: %q %q %v", anchor, nativeRoot, err)
	}
}

func TestAntigravityNativeManifestAndMCPProjection(t *testing.T) {
	t.Parallel()
	staged := t.TempDir()
	root := filepath.Join(staged, "plugin")
	if err := os.MkdirAll(root, 0o700); err != nil { t.Fatal(err) }
	envelope := domain.PackageEnvelope{
		Manifest: domain.PluginManifest{Name: "test-plugin", Version: "1.2.3"},
		MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{
			"api": {Name: "api", Type: "streamable-http", Decoded: map[string]any{
				"type": "streamable-http", "url": "https://example.test/mcp",
			}},
		}},
	}
	plan := domain.DeliveryPlan{
		ClientID: domain.ClientAntigravity, Scope: domain.ScopeProject,
		ActivePath: filepath.Join(staged, "active"),
		Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "api", Support: domain.SupportNative}},
	}
	if err := projectPlugin(root, envelope, plan, ""); err != nil { t.Fatal(err) }
	raw, err := os.ReadFile(filepath.Join(root, ".antigravity-plugin", "plugin.json"))
	if err != nil { t.Fatal(err) }
	var manifest map[string]any
	if err := json.Unmarshal(raw, &manifest); err != nil || manifest["name"] != "test-plugin" {
		t.Fatalf("unexpected native manifest: %v, %v", manifest, err)
	}
	raw, err = os.ReadFile(filepath.Join(root, "mcp_config.json"))
	if err != nil { t.Fatal(err) }
	var config struct { Servers map[string]map[string]any `json:"mcpServers"` }
	if err := json.Unmarshal(raw, &config); err != nil { t.Fatal(err) }
	if config.Servers["api"]["serverUrl"] != "https://example.test/mcp" {
		t.Fatalf("remote MCP serverUrl mapping invalid: %+v", config.Servers)
	}
	if _, exists := config.Servers["api"]["url"]; exists {
		t.Fatal("legacy URL field must not be included")
	}
	if err := New().Project(context.Background(), clients.ProjectionInput{StagingPath: root, Envelope: envelope, Plan: plan}); err != nil {
		_ = err
	}
}

func TestAntigravityRejectsUnqualifiedComponents(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	envelope := domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "bad_name"}}
	plan := domain.DeliveryPlan{ActivePath: filepath.Join(root, "active")}
	if err := projectPlugin(root, envelope, plan, ""); err == nil {
		t.Fatal("noncanonical native plugin identity must be refused")
	}
	envelope.Manifest.Name = "valid-plugin"
	plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentExtension, Name: "unsafe", Support: domain.SupportNative}}
	if err := projectPlugin(root, envelope, plan, ""); err == nil {
		t.Fatal("foreign extension/hook must not be silently enabled")
	}
}
