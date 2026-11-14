package codex

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestApplyInstallMaterializesMarketplaceAndBundle(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(project, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := writeAuthoredCodexSource(t, "0.1.0", "original")
	adapter := Adapter{ProjectRoot: project, UserHome: home}
	manifest := domain.IntegrationManifest{
		IntegrationID: "codex-smoke",
		Version:       "0.1.0",
		Description:   "codex smoke",
	}
	result, err := adapter.ApplyInstall(context.Background(), ports.ApplyInput{
		Manifest: manifest,
		ResolvedSource: &ports.ResolvedSource{
			Kind:      "local_path",
			LocalPath: source,
		},
		Policy: domain.InstallPolicy{Scope: "project"},
	})
	if err != nil {
		t.Fatalf("ApplyInstall: %v", err)
	}
	if result.State != domain.InstallActivationPending {
		t.Fatalf("state = %s", result.State)
	}
	if result.ActivationState != domain.ActivationNativePending {
		t.Fatalf("activation = %s", result.ActivationState)
	}
	catalogPath := filepath.Join(project, ".agents", "plugins", "marketplace.json")
	body, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("read marketplace: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse marketplace: %v", err)
	}
	plugins := doc["plugins"].([]any)
	if len(plugins) != 1 {
		t.Fatalf("plugins = %+v", plugins)
	}
	entry := plugins[0].(map[string]any)
	sourceDoc := entry["source"].(map[string]any)
	if sourceDoc["path"] != "./plugins/codex-smoke" {
		t.Fatalf("source.path = %+v", sourceDoc["path"])
	}
	pluginBody, err := os.ReadFile(filepath.Join(project, ".agents", "plugins", "plugins", "codex-smoke", ".codex-plugin", "plugin.json"))
	if err != nil {
		t.Fatalf("read plugin manifest: %v", err)
	}
	if !strings.Contains(string(pluginBody), `"version": "0.1.0"`) {
		t.Fatalf("plugin manifest = %s", string(pluginBody))
	}
	if !strings.Contains(string(pluginBody), `"mcpServers": "./.mcp.json"`) {
		t.Fatalf("plugin manifest missing mcpServers ref: %s", string(pluginBody))
	}
	mcpBody, err := os.ReadFile(filepath.Join(project, ".agents", "plugins", "plugins", "codex-smoke", ".mcp.json"))
	if err != nil {
		t.Fatalf("read mcp config: %v", err)
	}
	if !strings.Contains(string(mcpBody), filepath.Join(source, "bin", "smoke.js")) {
		t.Fatalf("mcp config missing source-root command: %s", string(mcpBody))
	}
}

func TestApplyUpdateRefreshesBundleAndPreservesOtherMarketplaceEntries(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(project, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	catalogPath := filepath.Join(project, ".agents", "plugins", "marketplace.json")
	if err := os.MkdirAll(filepath.Dir(catalogPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalogPath, []byte("{\n  \"name\": \"local-repo\",\n  \"plugins\": [\n    {\"name\":\"alpha\",\"source\":{\"source\":\"local\",\"path\":\"./plugins/alpha\"},\"policy\":{\"installation\":\"AVAILABLE\",\"authentication\":\"ON_INSTALL\"},\"category\":\"Productivity\"}\n  ]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := writeAuthoredCodexSource(t, "0.2.0", "updated")
	adapter := Adapter{ProjectRoot: project, UserHome: home}
	record := domain.InstallationRecord{
		IntegrationID: "codex-smoke",
		Policy:        domain.InstallPolicy{Scope: "project"},
		Targets: map[domain.TargetID]domain.TargetInstallation{
			domain.TargetCodex: {
				TargetID:     domain.TargetCodex,
				DeliveryKind: domain.DeliveryCodexMarketplace,
				OwnedNativeObjects: []domain.NativeObjectRef{
					{Kind: "marketplace_catalog", Path: catalogPath},
					{Kind: "plugin_root", Path: filepath.Join(project, ".agents", "plugins", "plugins", "codex-smoke")},
				},
			},
		},
	}
	manifest := domain.IntegrationManifest{
		IntegrationID: "codex-smoke",
		Version:       "0.2.0",
		Description:   "codex smoke",
	}
	result, err := adapter.ApplyUpdate(context.Background(), ports.ApplyInput{
		Manifest: manifest,
		ResolvedSource: &ports.ResolvedSource{
			Kind:      "local_path",
			LocalPath: source,
		},
		Record: &record,
	})
	if err != nil {
		t.Fatalf("ApplyUpdate: %v", err)
	}
	if !result.RestartRequired || !result.NewThreadRequired {
		t.Fatalf("update result flags = %+v", result)
	}
	body, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("read marketplace: %v", err)
	}
	text := string(body)
	for _, want := range []string{`"name": "alpha"`, `"name": "codex-smoke"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("marketplace missing %q:\n%s", want, text)
		}
	}
	pluginBody, err := os.ReadFile(filepath.Join(project, ".agents", "plugins", "plugins", "codex-smoke", ".codex-plugin", "plugin.json"))
	if err != nil {
		t.Fatalf("read plugin manifest: %v", err)
	}
	if !strings.Contains(string(pluginBody), `"version": "0.2.0"`) || !strings.Contains(string(pluginBody), `"updated"`) {
		t.Fatalf("plugin manifest = %s", string(pluginBody))
	}
}

func TestApplyInstallRejectsManagedManifestExtraOverride(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(project, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := writeAuthoredCodexSource(t, "0.1.0", "original")
	writeCodexFile(t, filepath.Join(source, "plugin", "targets", "codex-package", "manifest.extra.json"), "{\n  \"mcpServers\": \"./override.json\"\n}\n")
	adapter := Adapter{ProjectRoot: project, UserHome: home}
	manifest := domain.IntegrationManifest{
		IntegrationID: "codex-smoke",
		Version:       "0.1.0",
		Description:   "codex smoke",
	}

	_, err := adapter.ApplyInstall(context.Background(), ports.ApplyInput{
		Manifest: manifest,
		ResolvedSource: &ports.ResolvedSource{
			Kind:      "local_path",
			LocalPath: source,
		},
		Policy: domain.InstallPolicy{Scope: "project"},
	})
	if err == nil {
		t.Fatal("expected managed manifest.extra override error")
	}
	if !strings.Contains(err.Error(), "Codex manifest.extra.json may not override managed key mcpServers") {
		t.Fatalf("err = %v", err)
	}
}

func TestInspectProjectScopeUsesPersistedWorkspaceRoot(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	workspaceA := filepath.Join(root, "workspace-a", "nested")
	workspaceB := filepath.Join(root, "workspace-b")
	if err := os.MkdirAll(filepath.Join(workspaceA, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	catalogPath := filepath.Join(workspaceA, ".agents", "plugins", "marketplace.json")
	if err := os.MkdirAll(filepath.Dir(catalogPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalogPath, []byte("{\n  \"name\": \"local-repo\",\n  \"plugins\": [\n    {\"name\":\"codex-demo\",\"source\":{\"source\":\"local\",\"path\":\"./plugins/codex-demo\"},\"policy\":{\"installation\":\"AVAILABLE\",\"authentication\":\"ON_INSTALL\"},\"category\":\"Productivity\"}\n  ]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pluginRoot := filepath.Join(workspaceA, ".agents", "plugins", "plugins", "codex-demo")
	if err := os.MkdirAll(pluginRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	adapter := Adapter{UserHome: home}
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(prevWD) }()
	if err := os.MkdirAll(workspaceB, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workspaceB); err != nil {
		t.Fatalf("chdir workspace-b: %v", err)
	}

	inspect, err := adapter.Inspect(context.Background(), ports.InspectInput{
		Scope: "project",
		Record: &domain.InstallationRecord{
			IntegrationID: "codex-demo",
			Policy:        domain.InstallPolicy{Scope: "project"},
			WorkspaceRoot: workspaceA,
		},
	})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if inspect.State != domain.InstallActivationPending {
		t.Fatalf("state = %s, want activation_pending", inspect.State)
	}
	if len(inspect.SettingsFiles) == 0 || inspect.SettingsFiles[0] != catalogPath {
		t.Fatalf("settings files = %#v, want %q first", inspect.SettingsFiles, catalogPath)
	}
}

func TestInspectTreatsExistingCatalogWithoutOwnedEntryAsRemoved(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace-a")
	if err := os.MkdirAll(filepath.Join(workspace, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	catalogPath := filepath.Join(workspace, ".agents", "plugins", "marketplace.json")
	if err := os.MkdirAll(filepath.Dir(catalogPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalogPath, []byte("{\n  \"name\": \"local-repo\",\n  \"plugins\": [\n    {\"name\":\"alpha\",\"source\":{\"source\":\"local\",\"path\":\"./plugins/alpha\"},\"policy\":{\"installation\":\"AVAILABLE\",\"authentication\":\"ON_INSTALL\"},\"category\":\"Productivity\"}\n  ]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	adapter := Adapter{UserHome: home}

	inspect, err := adapter.Inspect(context.Background(), ports.InspectInput{
		Scope: "project",
		Record: &domain.InstallationRecord{
			IntegrationID: "codex-demo",
			Policy:        domain.InstallPolicy{Scope: "project"},
			WorkspaceRoot: workspace,
		},
	})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if inspect.State != domain.InstallRemoved {
		t.Fatalf("state = %s, want removed", inspect.State)
	}
}

func TestApplyRemovePrunesOwnedEntryAndPluginRoot(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(project, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	catalogPath := filepath.Join(project, ".agents", "plugins", "marketplace.json")
	pluginRoot := filepath.Join(project, ".agents", "plugins", "plugins", "codex-smoke")
	if err := os.MkdirAll(filepath.Join(pluginRoot, ".codex-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, ".codex-plugin", "plugin.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(catalogPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalogPath, []byte("{\n  \"name\": \"local-repo\",\n  \"plugins\": [\n    {\"name\":\"alpha\",\"source\":{\"source\":\"local\",\"path\":\"./plugins/alpha\"},\"policy\":{\"installation\":\"AVAILABLE\",\"authentication\":\"ON_INSTALL\"},\"category\":\"Productivity\"},\n    {\"name\":\"codex-smoke\",\"source\":{\"source\":\"local\",\"path\":\"./plugins/codex-smoke\"},\"policy\":{\"installation\":\"AVAILABLE\",\"authentication\":\"ON_INSTALL\"},\"category\":\"Productivity\"}\n  ]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	adapter := Adapter{ProjectRoot: project, UserHome: home}
	record := domain.InstallationRecord{
		IntegrationID: "codex-smoke",
		Policy:        domain.InstallPolicy{Scope: "project"},
		Targets: map[domain.TargetID]domain.TargetInstallation{
			domain.TargetCodex: {
				TargetID:     domain.TargetCodex,
				DeliveryKind: domain.DeliveryCodexMarketplace,
				OwnedNativeObjects: []domain.NativeObjectRef{
					{Kind: "marketplace_catalog", Path: catalogPath},
					{Kind: "plugin_root", Path: pluginRoot},
				},
			},
		},
	}
	result, err := adapter.ApplyRemove(context.Background(), ports.ApplyInput{Record: &record})
	if err != nil {
		t.Fatalf("ApplyRemove: %v", err)
	}
	if result.State != domain.InstallRemoved {
		t.Fatalf("state = %s", result.State)
	}
	if _, err := os.Stat(pluginRoot); !os.IsNotExist(err) {
		t.Fatalf("plugin root still exists: %v", err)
	}
	body, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("read marketplace: %v", err)
	}
	text := string(body)
	if strings.Contains(text, `"name": "codex-smoke"`) {
		t.Fatalf("owned entry still present:\n%s", text)
	}
	if !strings.Contains(text, `"name": "alpha"`) {
		t.Fatalf("unmanaged entry missing:\n%s", text)
	}
}

func TestRepairUsesUpdateSemantics(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(project, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := writeAuthoredCodexSource(t, "0.3.0", "repair")
	pluginRoot := filepath.Join(project, ".agents", "plugins", "plugins", "codex-smoke")
	record := domain.InstallationRecord{
		IntegrationID: "codex-smoke",
		Policy:        domain.InstallPolicy{Scope: "project"},
		Targets: map[domain.TargetID]domain.TargetInstallation{
			domain.TargetCodex: {
				TargetID:     domain.TargetCodex,
				DeliveryKind: domain.DeliveryCodexMarketplace,
				OwnedNativeObjects: []domain.NativeObjectRef{
					{Kind: "marketplace_catalog", Path: filepath.Join(project, ".agents", "plugins", "marketplace.json")},
					{Kind: "plugin_root", Path: pluginRoot},
				},
			},
		},
	}
	manifest := domain.IntegrationManifest{
		IntegrationID: "codex-smoke",
		Version:       "0.3.0",
		Description:   "codex smoke",
	}
	adapter := Adapter{ProjectRoot: project, UserHome: home}
	result, err := adapter.Repair(context.Background(), ports.RepairInput{
		Record:   record,
		Manifest: &manifest,
		ResolvedSource: &ports.ResolvedSource{
			Kind:      "local_path",
			LocalPath: source,
		},
	})
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}
	if result.State != domain.InstallActivationPending {
		t.Fatalf("state = %s", result.State)
	}
	if !result.RestartRequired {
		t.Fatalf("repair should require restart")
	}
}

func TestPlanUpdateUsesPersistedOwnedPaths(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	project := filepath.Join(t.TempDir(), "repo")
	adapter := Adapter{ProjectRoot: project, UserHome: home}
	record := domain.InstallationRecord{
		IntegrationID: "codex-smoke",
		Policy:        domain.InstallPolicy{Scope: "project"},
		WorkspaceRoot: filepath.Join(project, "workspace"),
		Targets: map[domain.TargetID]domain.TargetInstallation{
			domain.TargetCodex: {
				OwnedNativeObjects: []domain.NativeObjectRef{
					{Kind: "marketplace_catalog", Path: filepath.Join(project, ".managed", "marketplace.json")},
					{Kind: "plugin_root", Path: filepath.Join(project, ".managed", "plugins", "codex-smoke")},
				},
			},
		},
	}

	plan, err := adapter.PlanUpdate(context.Background(), ports.PlanUpdateInput{CurrentRecord: record})
	if err != nil {
		t.Fatalf("PlanUpdate: %v", err)
	}
	if got, want := plan.PathsTouched[0], filepath.Join(project, ".managed", "marketplace.json"); got != want {
		t.Fatalf("catalog path = %q, want %q", got, want)
	}
	if got, want := plan.PathsTouched[1], filepath.Join(project, ".managed", "plugins", "codex-smoke"); got != want {
		t.Fatalf("plugin root = %q, want %q", got, want)
	}
}

func TestInspectDetectsInstalledStateFromCodexConfig(t *testing.T) {
	withFakeCodexBinary(t)
	home := t.TempDir()
	project := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(project, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	pluginRoot := filepath.Join(project, ".agents", "plugins", "plugins", "codex-smoke")
	if err := os.MkdirAll(filepath.Join(pluginRoot, ".codex-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, ".codex-plugin", "plugin.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalogPath := filepath.Join(project, ".agents", "plugins", "marketplace.json")
	if err := os.MkdirAll(filepath.Dir(catalogPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalogPath, []byte("{\n  \"name\": \"local-repo\",\n  \"plugins\": [\n    {\"name\":\"codex-smoke\",\"source\":{\"source\":\"local\",\"path\":\"./plugins/codex-smoke\"},\"policy\":{\"installation\":\"AVAILABLE\",\"authentication\":\"ON_INSTALL\"},\"category\":\"Productivity\"}\n  ]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("[plugins.\"codex-smoke@local-repo\"]\nenabled = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	createCodexCacheBundle(t, home, "local-repo", "codex-smoke")
	record := domain.InstallationRecord{
		IntegrationID: "codex-smoke",
		Policy:        domain.InstallPolicy{Scope: "project"},
		Targets: map[domain.TargetID]domain.TargetInstallation{
			domain.TargetCodex: {
				TargetID:     domain.TargetCodex,
				DeliveryKind: domain.DeliveryCodexMarketplace,
				OwnedNativeObjects: []domain.NativeObjectRef{
					{Kind: "marketplace_catalog", Path: catalogPath},
					{Kind: "plugin_root", Path: pluginRoot},
				},
				AdapterMetadata: map[string]any{"catalog_name": "local-repo"},
			},
		},
	}
	adapter := Adapter{ProjectRoot: project, UserHome: home}
	inspect, err := adapter.Inspect(context.Background(), ports.InspectInput{Record: &record, Scope: "project"})
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if inspect.State != domain.InstallInstalled {
		t.Fatalf("state = %s", inspect.State)
	}
	if inspect.ActivationState != domain.ActivationComplete {
		t.Fatalf("activation = %s", inspect.ActivationState)
	}
	if len(inspect.EnvironmentRestrictions) != 0 {
		t.Fatalf("restrictions = %+v", inspect.EnvironmentRestrictions)
	}
	if !hasObservedKind(inspect.ObservedNativeObjects, "installed_cache_bundle") {
		t.Fatalf("observed native objects = %+v", inspect.ObservedNativeObjects)
	}
}

func TestInspectDetectsDisabledStateFromCodexConfig(t *testing.T) {
	withFakeCodexBinary(t)
	home := t.TempDir()
	project := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(project, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	pluginRoot := filepath.Join(project, ".agents", "plugins", "plugins", "codex-smoke")
	if err := os.MkdirAll(filepath.Join(pluginRoot, ".codex-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, ".codex-plugin", "plugin.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalogPath := filepath.Join(project, ".agents", "plugins", "marketplace.json")
	if err := os.MkdirAll(filepath.Dir(catalogPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalogPath, []byte("{\n  \"name\": \"local-repo\",\n  \"plugins\": [\n    {\"name\":\"codex-smoke\",\"source\":{\"source\":\"local\",\"path\":\"./plugins/codex-smoke\"},\"policy\":{\"installation\":\"AVAILABLE\",\"authentication\":\"ON_INSTALL\"},\"category\":\"Productivity\"}\n  ]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("[plugins.\"codex-smoke@local-repo\"]\nenabled = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	createCodexCacheBundle(t, home, "local-repo", "codex-smoke")
	record := domain.InstallationRecord{
		IntegrationID: "codex-smoke",
		Policy:        domain.InstallPolicy{Scope: "project"},
		Targets: map[domain.TargetID]domain.TargetInstallation{
			domain.TargetCodex: {
				TargetID:     domain.TargetCodex,
				DeliveryKind: domain.DeliveryCodexMarketplace,
				OwnedNativeObjects: []domain.NativeObjectRef{
					{Kind: "marketplace_catalog", Path: catalogPath},
					{Kind: "plugin_root", Path: pluginRoot},
				},
				AdapterMetadata: map[string]any{"catalog_name": "local-repo"},
			},
		},
	}
	adapter := Adapter{ProjectRoot: project, UserHome: home}
	inspect, err := adapter.Inspect(context.Background(), ports.InspectInput{Record: &record, Scope: "project"})
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if inspect.State != domain.InstallDisabled {
		t.Fatalf("state = %s", inspect.State)
	}
	if inspect.ActivationState != domain.ActivationComplete {
		t.Fatalf("activation = %s", inspect.ActivationState)
	}
}

func TestInspectWarnsOnInvalidCodexConfigToml(t *testing.T) {
	t.Parallel()
	withFakeCodexBinary(t)
	home := t.TempDir()
	project := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(project, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	catalogPath := filepath.Join(project, ".agents", "plugins", "marketplace.json")
	pluginRoot := filepath.Join(project, ".agents", "plugins", "plugins", "codex-smoke")
	if err := os.MkdirAll(pluginRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(catalogPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalogPath, []byte("{\n  \"name\": \"local-repo\",\n  \"plugins\": [\n    {\"name\":\"codex-smoke\",\"source\":{\"source\":\"local\",\"path\":\"./plugins/codex-smoke\"},\"policy\":{\"installation\":\"AVAILABLE\",\"authentication\":\"ON_INSTALL\"},\"category\":\"Productivity\"}\n  ]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("[plugins.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	adapter := Adapter{ProjectRoot: project, UserHome: home}
	record := domain.InstallationRecord{
		IntegrationID: "codex-smoke",
		Policy:        domain.InstallPolicy{Scope: "project"},
		WorkspaceRoot: project,
		Targets: map[domain.TargetID]domain.TargetInstallation{
			domain.TargetCodex: {
				OwnedNativeObjects: []domain.NativeObjectRef{
					{Kind: "marketplace_catalog", Path: catalogPath},
					{Kind: "plugin_root", Path: pluginRoot},
				},
				AdapterMetadata: map[string]any{"catalog_name": "local-repo"},
			},
		},
	}

	inspect, err := adapter.Inspect(context.Background(), ports.InspectInput{Record: &record, Scope: "project"})
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	found := false
	for _, warning := range inspect.Warnings {
		if strings.Contains(warning, "parse Codex config.toml:") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("warnings = %#v", inspect.Warnings)
	}
}

func TestInspectPreparedWithoutCacheRemainsActivationPending(t *testing.T) {
	withFakeCodexBinary(t)
	home := t.TempDir()
	project := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(project, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	pluginRoot := filepath.Join(project, ".agents", "plugins", "plugins", "codex-smoke")
	if err := os.MkdirAll(filepath.Join(pluginRoot, ".codex-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, ".codex-plugin", "plugin.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalogPath := filepath.Join(project, ".agents", "plugins", "marketplace.json")
	if err := os.MkdirAll(filepath.Dir(catalogPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalogPath, []byte("{\n  \"name\": \"local-repo\",\n  \"plugins\": [\n    {\"name\":\"codex-smoke\",\"source\":{\"source\":\"local\",\"path\":\"./plugins/codex-smoke\"},\"policy\":{\"installation\":\"AVAILABLE\",\"authentication\":\"ON_INSTALL\"},\"category\":\"Productivity\"}\n  ]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	record := domain.InstallationRecord{
		IntegrationID: "codex-smoke",
		Policy:        domain.InstallPolicy{Scope: "project"},
		Targets: map[domain.TargetID]domain.TargetInstallation{
			domain.TargetCodex: {
				TargetID:     domain.TargetCodex,
				DeliveryKind: domain.DeliveryCodexMarketplace,
				OwnedNativeObjects: []domain.NativeObjectRef{
					{Kind: "marketplace_catalog", Path: catalogPath},
					{Kind: "plugin_root", Path: pluginRoot},
				},
				AdapterMetadata: map[string]any{"catalog_name": "local-repo"},
			},
		},
	}
	adapter := Adapter{ProjectRoot: project, UserHome: home}
	inspect, err := adapter.Inspect(context.Background(), ports.InspectInput{Record: &record, Scope: "project"})
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if inspect.State != domain.InstallActivationPending {
		t.Fatalf("state = %s", inspect.State)
	}
	if inspect.ActivationState != domain.ActivationNativePending {
		t.Fatalf("activation = %s", inspect.ActivationState)
	}
	if !containsRestriction(inspect.EnvironmentRestrictions, domain.RestrictionNewThreadRequired) {
		t.Fatalf("restrictions = %+v", inspect.EnvironmentRestrictions)
	}
}

func TestInspectReadsCatalogPolicyAndObservedCache(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	project := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(project, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	catalogPath := filepath.Join(project, ".agents", "plugins", "marketplace.json")
	if err := os.MkdirAll(filepath.Dir(catalogPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalogPath, []byte("{\n  \"name\": \"local-repo\",\n  \"plugins\": [\n    {\"name\":\"codex-smoke\",\"source\":{\"source\":\"local\",\"path\":\"./plugins/codex-smoke\"},\"policy\":{\"installation\":\"AVAILABLE\",\"authentication\":\"ON_INSTALL\"},\"category\":\"Productivity\"}\n  ]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	createCodexCacheBundle(t, home, "local-repo", "codex-smoke")

	adapter := Adapter{ProjectRoot: project, UserHome: home}
	inspect, err := adapter.Inspect(context.Background(), ports.InspectInput{
		Scope: "project",
		Record: &domain.InstallationRecord{
			IntegrationID: "codex-smoke",
			Policy:        domain.InstallPolicy{Scope: "project"},
			WorkspaceRoot: project,
		},
	})
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if inspect.CatalogPolicy == nil || inspect.CatalogPolicy.Authentication != "ON_INSTALL" {
		t.Fatalf("catalog policy = %+v", inspect.CatalogPolicy)
	}
	if !hasObservedKind(inspect.ObservedNativeObjects, "installed_cache_bundle") {
		t.Fatalf("observed = %+v", inspect.ObservedNativeObjects)
	}
}

func TestPathsForRecordPrefersOwnedNativePaths(t *testing.T) {
	t.Parallel()
	adapter := Adapter{ProjectRoot: "/repo", UserHome: "/home/demo"}
	record := domain.InstallationRecord{
		IntegrationID: "codex-demo",
		Policy:        domain.InstallPolicy{Scope: "project"},
		WorkspaceRoot: "/repo/worktree",
		Targets: map[domain.TargetID]domain.TargetInstallation{
			domain.TargetCodex: {
				OwnedNativeObjects: []domain.NativeObjectRef{
					{Kind: "marketplace_catalog", Path: "/managed/catalog.json"},
					{Kind: "plugin_root", Path: "/managed/plugin-root"},
				},
			},
		},
	}

	paths := adapter.pathsForRecord(record)
	if paths.CatalogPath != "/managed/catalog.json" {
		t.Fatalf("catalog path = %q", paths.CatalogPath)
	}
	if paths.PluginRoot != "/managed/plugin-root" {
		t.Fatalf("plugin root = %q", paths.PluginRoot)
	}
	if paths.ConfigPath != "/home/demo/.codex/config.toml" {
		t.Fatalf("config path = %q", paths.ConfigPath)
	}
}

func writeAuthoredCodexSource(t *testing.T, version, prompt string) string {
	t.Helper()
	root := t.TempDir()
	writeCodexFile(t, filepath.Join(root, "plugin", "plugin.yaml"), "name: codex-smoke\nversion: "+version+"\ndescription: codex smoke\n")
	writeCodexFile(t, filepath.Join(root, "plugin", "skills", "codex-smoke", "SKILL.md"), "# skill\n")
	writeCodexFile(t, filepath.Join(root, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  smoke:\n    type: stdio\n    targets:\n      - codex-package\n    stdio:\n      command: ${package.root}/bin/smoke.js\n")
	writeCodexFile(t, filepath.Join(root, "plugin", "targets", "codex-package", "package.yaml"), "homepage: https://example.com/codex-smoke\nauthor:\n  name: Example\nkeywords:\n  - codex\n")
	writeCodexFile(t, filepath.Join(root, "plugin", "targets", "codex-package", "interface.json"), "{\n  \"defaultPrompt\": [\n    \""+prompt+"\"\n  ]\n}\n")
	return root
}

func writeCodexFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func withFakeCodexBinary(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "codex")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	original := os.Getenv("PATH")
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+original); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Setenv("PATH", original)
	})
}

func createCodexCacheBundle(t *testing.T, home, marketplace, plugin string) {
	t.Helper()
	root := filepath.Join(home, ".codex", "plugins", "cache", marketplace, plugin, "local", ".codex-plugin")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasObservedKind(items []domain.NativeObjectRef, kind string) bool {
	for _, item := range items {
		if item.Kind == kind {
			return true
		}
	}
	return false
}

func containsRestriction(items []domain.EnvironmentRestrictionCode, want domain.EnvironmentRestrictionCode) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
