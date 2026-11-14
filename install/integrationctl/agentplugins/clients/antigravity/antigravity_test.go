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
	root, canonicalErr := filepath.EvalSymlinks(t.TempDir())
	if canonicalErr != nil {
		t.Fatal(canonicalErr)
	}
	config := filepath.Join(root, ".gemini", "config")
	detected := adapter.DetectSurfaces(clients.NewHost(clients.HostProbes{
		HomeDir: root, Environment: map[string]string{},
		Lstat: os.Lstat, EvalSymlinks: filepath.EvalSymlinks,
		LookPath: func(name string) (string, error) {
			if name != "agy" {
				t.Fatalf("unrecognized probe %q", name)
			}
			return filepath.Join(root, "agy"), nil
		},
	}))
	if detected.Err != nil || detected.ConfigRoot != config || detected.NativeConfigRoot != config {
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
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	// Real source staging has a portable manifest; only the staged copy becomes native.
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"test-plugin"}`), 0o600); err != nil {
		t.Fatal(err)
	}
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
	if err := projectPlugin(root, envelope, plan, ""); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(raw, &manifest); err != nil || manifest["name"] != "test-plugin" ||
		manifest["$schema"] != "https://antigravity.google/schemas/v1/plugin.json" {
		t.Fatalf("unexpected native manifest: %v, %v", manifest, err)
	}
	raw, err = os.ReadFile(filepath.Join(root, "mcp_config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Servers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	if config.Servers["api"]["serverUrl"] != "https://example.test/mcp" {
		t.Fatalf("remote MCP serverUrl mapping invalid: %+v", config.Servers)
	}
	if _, exists := config.Servers["api"]["url"]; exists {
		t.Fatal("legacy URL field must not be included")
	}
	// A second run is performed on an independently staged source. An already
	// generated native manifest cannot be silently overwritten or adopted.
	other := filepath.Join(t.TempDir(), "plugin")
	if err := os.MkdirAll(other, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := New().Project(context.Background(), clients.ProjectionInput{StagingPath: other, Envelope: envelope, Plan: plan}); err != nil {
		t.Fatalf("native projector on a clean stage: %v", err)
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

func TestAntigravityRefusesUnselectedHookAndMCPFiles(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"hooks.json", "mcp_config.json"} {
		t.Run(raw, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, raw), []byte("{}"), 0o600); err != nil {
				t.Fatal(err)
			}
			envelope := domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "test-plugin"}}
			plan := domain.DeliveryPlan{ActivePath: filepath.Join(t.TempDir(), "active")}
			if err := projectPlugin(root, envelope, plan, ""); err == nil {
				t.Fatalf("unselected %s must not be installed natively", raw)
			}
			if _, err := os.Stat(filepath.Join(root, "plugin.json")); !os.IsNotExist(err) && raw == "hooks.json" {
				t.Fatalf("hook preflight must not create native manifest: %v", err)
			}
		})
	}
}

func TestAntigravityRefusesUnsafeRemoteMCP(t *testing.T) {
	t.Parallel()
	for _, endpoint := range []string{"http://remote.example.test/mcp", "file:///secret", "https://"} {
		t.Run(endpoint, func(t *testing.T) {
			servers := map[string]map[string]any{}
			envelope := domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{
				"bad": {Type: "streamable-http", Decoded: map[string]any{"type": "streamable-http", "url": endpoint}},
			}}}
			if err := appendNativeServer(servers, "bad", envelope, t.TempDir(), t.TempDir(), ""); err == nil {
				t.Fatal("invalid remote endpoint accepted")
			}
		})
	}
}

func TestAntigravityAllowsOnlySelectedStdioHelperScripts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	scripts := filepath.Join(root, "scripts")
	if err := os.MkdirAll(scripts, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scripts, "helper.js"), []byte("process.stdout.write('ok')"), 0o600); err != nil {
		t.Fatal(err)
	}
	envelope := domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "scripted-plugin"}}
	plan := domain.DeliveryPlan{ActivePath: filepath.Join(t.TempDir(), "active")}
	if err := validateAntigravityProjection(root, envelope, plan); err == nil {
		t.Fatal("unselected scripts must not be enabled")
	}
	envelope.MCP.Servers = map[string]domain.MCPServer{
		"helper": {Name: "helper", Type: "stdio", Decoded: map[string]any{
			"type": "stdio", "command": "node", "args": []any{"${PLUGIN_ROOT}/scripts/helper.js"},
		}},
	}
	plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer,
		Name: "helper", Support: domain.SupportNative}}
	if err := validateAntigravityProjection(root, envelope, plan); err != nil {
		t.Fatalf("selected verified stdio helper scripts must remain usable: %v", err)
	}
}

func TestAntigravityRejectsInvalidNativeIdentityBeforeStaging(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"bad_name", "Has Uppercase", "contains/slash", ""} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			plan := domain.DeliveryPlan{
				ClientID: domain.ClientAntigravity, DeclaredName: name,
				Scope:         domain.ScopeProject,
				TargetContext: domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: root},
				Status:        domain.PlanReady,
			}
			input := clients.PlanInput{Envelope: domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: name}}}
			if err := New().CheckPlanPrecondition(input, &plan); err != nil || plan.Status != domain.PlanUnsupported {
				t.Fatalf("invalid identity was not refused before staging: %+v %v", plan, err)
			}
			if _, err := os.Lstat(filepath.Join(root, ".agents")); !os.IsNotExist(err) {
				t.Fatalf("identity preflight mutated workspace: %v", err)
			}
		})
	}
}

func TestAntigravityMCPRejectsMixedTransportFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		kind  string
		entry map[string]any
	}{
		{"remote-command", "streamable-http", map[string]any{"type": "streamable-http", "url": "https://example.test/mcp", "command": "node"}},
		{"remote-args", "streamable-http", map[string]any{"type": "streamable-http", "url": "https://example.test/mcp", "args": []any{"run"}}},
		{"remote-env", "streamable-http", map[string]any{"type": "streamable-http", "url": "https://example.test/mcp", "env": map[string]any{"TOKEN": "private"}}},
		{"stdio-url", "stdio", map[string]any{"type": "stdio", "command": "node", "url": "https://example.test/mcp"}},
		{"stdio-server-url", "stdio", map[string]any{"type": "stdio", "command": "node", "serverUrl": "https://example.test/mcp"}},
		{"stdio-headers", "stdio", map[string]any{"type": "stdio", "command": "node", "headers": map[string]any{"Secret": "private"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := make(map[string]map[string]any)
			envelope := domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{
				"mixed": {Name: "mixed", Type: tc.kind, Decoded: tc.entry},
			}}}
			if err := appendNativeServer(result, "mixed", envelope, t.TempDir(), t.TempDir(), ""); err == nil {
				t.Fatal("conflicting MCP transport was admitted")
			}
			if len(result) != 0 {
				t.Fatal("refused transport leaked native server configuration")
			}
		})
	}
}

func TestOriginalNativeProfileIsHostDerived(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	selected := filepath.Join(t.TempDir(), "other-profile")
	adapter := New()
	observed := adapter.DetectSurfaces(clients.NewHost(clients.HostProbes{
		HomeDir: root, Environment: map[string]string{"HOME": selected, "USERPROFILE": selected},
		Lstat: os.Lstat, EvalSymlinks: filepath.EvalSymlinks,
		LookPath: func(string) (string, error) { return "", os.ErrNotExist },
	}))
	original := filepath.Join(root, ".gemini", "config")
	if observed.Err != nil || observed.NativeConfigRoot != original || observed.ConfigRoot != original {
		t.Fatal("host provenance lost", observed)
	}
	resolved, err := adapter.ResolveProfileRoot(selected)
	if err != nil {
		t.Fatal(err)
	}
	if resolved == observed.NativeConfigRoot {
		t.Fatal("explicit profile became original native authority")
	}
	if observed.ExecutablePath != "" {
		t.Fatal("provenance must not grant executable authority")
	}
}

type aliasedProfileHost struct {
	clients.Host
	canonical string
}

func (host aliasedProfileHost) CanonicalDirectory(string) (string, error) {
	return host.canonical, nil
}

func TestAliasedDefaultSpellingDoesNotQualifyNativeHome(t *testing.T) {
	home := t.TempDir()
	canonical := filepath.Join(t.TempDir(), ".gemini", "config")
	host := clients.NewHost(clients.HostProbes{
		HomeDir: home, Environment: map[string]string{},
		LookPath: func(string) (string, error) { return "", os.ErrNotExist },
	})
	observed := New().DetectSurfaces(aliasedProfileHost{Host: host, canonical: canonical})
	if observed.Err != nil || observed.ConfigRoot != canonical || observed.NativeConfigRoot != "" {
		t.Fatal("canonicalized alias silently granted a different HOME", observed)
	}
}
