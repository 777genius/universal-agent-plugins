package cursor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	fsadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/fs"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestApplyInstallPreservesUnmanagedEntries(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	sourceRoot := t.TempDir()
	mustWriteFile(t, filepath.Join(projectRoot, ".cursor", "mcp.json"), `{"mcpServers":{"user-owned":{"command":"node","args":["user.mjs"]}}}`)
	mustWriteFile(t, filepath.Join(sourceRoot, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  release-checks:\n    type: stdio\n    stdio:\n      command: node\n      args:\n        - ${package.root}/bin/release-checks.mjs\n")

	adapter := Adapter{
		FS:          fsadapter.OS{},
		ProjectRoot: projectRoot,
		UserHome:    t.TempDir(),
	}
	result, err := adapter.ApplyInstall(context.Background(), ports.ApplyInput{
		Policy: domain.InstallPolicy{Scope: "project"},
		ResolvedSource: &ports.ResolvedSource{
			Kind:      "local_path",
			LocalPath: sourceRoot,
		},
	})
	if err != nil {
		t.Fatalf("apply install: %v", err)
	}
	if result.State != domain.InstallInstalled {
		t.Fatalf("state = %s, want installed", result.State)
	}
	doc := readJSONFile(t, filepath.Join(projectRoot, ".cursor", "mcp.json"))
	servers := mustObject(t, doc["mcpServers"])
	if _, ok := servers["user-owned"]; !ok {
		t.Fatal("expected unmanaged Cursor MCP entry to remain")
	}
	releaseChecks := mustObject(t, servers["release-checks"])
	args := mustStringSlice(t, releaseChecks["args"])
	if len(args) != 1 || args[0] != filepath.Join(sourceRoot, "bin", "release-checks.mjs") {
		t.Fatalf("args = %#v, want interpolated package root", args)
	}
}

func TestApplyRemoveDeletesOnlyOwnedAliases(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	configPath := filepath.Join(projectRoot, ".cursor", "mcp.json")
	mustWriteFile(t, configPath, `{"mcpServers":{"user-owned":{"command":"node","args":["user.mjs"]},"release-checks":{"command":"node","args":["managed.mjs"]}}}`)

	adapter := Adapter{
		FS:          fsadapter.OS{},
		ProjectRoot: projectRoot,
		UserHome:    t.TempDir(),
	}
	record := domain.InstallationRecord{
		Policy: domain.InstallPolicy{Scope: "project"},
		Targets: map[domain.TargetID]domain.TargetInstallation{
			domain.TargetCursor: {
				TargetID: domain.TargetCursor,
				OwnedNativeObjects: []domain.NativeObjectRef{
					{Kind: "file", Path: configPath},
					{Kind: "cursor_mcp_server", Name: "release-checks", Path: configPath},
				},
			},
		},
	}
	_, err := adapter.ApplyRemove(context.Background(), ports.ApplyInput{Record: &record})
	if err != nil {
		t.Fatalf("apply remove: %v", err)
	}
	doc := readJSONFile(t, configPath)
	servers := mustObject(t, doc["mcpServers"])
	if _, ok := servers["release-checks"]; ok {
		t.Fatal("expected owned Cursor MCP entry to be removed")
	}
	if _, ok := servers["user-owned"]; !ok {
		t.Fatal("expected unmanaged Cursor MCP entry to remain")
	}
}

func TestApplyUpdateRemovesStaleOwnedAliasesAfterAliasRename(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	sourceRoot := t.TempDir()
	configPath := filepath.Join(projectRoot, ".cursor", "mcp.json")
	mustWriteFile(t, configPath, `{"mcpServers":{"user-owned":{"command":"node","args":["user.mjs"]},"release-checks":{"command":"node","args":["managed-old.mjs"]}}}`)
	mustWriteFile(t, filepath.Join(sourceRoot, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  release-checks-v2:\n    type: stdio\n    stdio:\n      command: node\n      args:\n        - ${package.root}/bin/release-checks-v2.mjs\n")

	adapter := Adapter{
		FS:          fsadapter.OS{},
		ProjectRoot: projectRoot,
		UserHome:    t.TempDir(),
	}
	_, err := adapter.ApplyUpdate(context.Background(), ports.ApplyInput{
		Policy:         domain.InstallPolicy{Scope: "project"},
		ResolvedSource: &ports.ResolvedSource{Kind: "local_path", LocalPath: sourceRoot},
		Record: &domain.InstallationRecord{
			Policy:        domain.InstallPolicy{Scope: "project"},
			WorkspaceRoot: projectRoot,
			Targets: map[domain.TargetID]domain.TargetInstallation{
				domain.TargetCursor: {
					TargetID: domain.TargetCursor,
					OwnedNativeObjects: []domain.NativeObjectRef{
						{Kind: "file", Path: configPath},
						{Kind: "cursor_mcp_server", Name: "release-checks", Path: configPath},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("apply update: %v", err)
	}
	doc := readJSONFile(t, configPath)
	servers := mustObject(t, doc["mcpServers"])
	if _, ok := servers["release-checks"]; ok {
		t.Fatal("expected stale owned Cursor MCP entry to be removed")
	}
	if _, ok := servers["release-checks-v2"]; !ok {
		t.Fatal("expected renamed managed Cursor MCP entry to be present")
	}
	if _, ok := servers["user-owned"]; !ok {
		t.Fatal("expected unmanaged Cursor MCP entry to remain")
	}
}

func TestApplyInstallMaterializesCursorPluginPackage(t *testing.T) {
	t.Parallel()
	userHome := t.TempDir()
	sourceRoot := t.TempDir()
	mustWriteFile(t, filepath.Join(sourceRoot, ".cursor-plugin", "plugin.json"), `{"name":"agent-code-navigator","version":"0.1.0","logo":"./icon.png","skills":"./skills/"}`)
	mustWriteFile(t, filepath.Join(sourceRoot, ".mcp.json"), `{"mcpServers":{"memory-platform":{"command":"./bin/memory-mcp","args":["--repo=${package.root}"]}}}`)
	mustWriteFile(t, filepath.Join(sourceRoot, "icon.png"), "fake icon")
	mustWriteFile(t, filepath.Join(sourceRoot, "skills", "code-tool-router", "SKILL.md"), "# Code tool router\n")

	adapter := Adapter{FS: fsadapter.OS{}, UserHome: userHome}
	result, err := adapter.ApplyInstall(context.Background(), ports.ApplyInput{
		Manifest: domain.IntegrationManifest{
			IntegrationID: "agent-code-navigator",
			Deliveries: []domain.Delivery{{
				TargetID:     domain.TargetCursor,
				DeliveryKind: domain.DeliveryCursorPlugin,
			}},
		},
		ResolvedSource: &ports.ResolvedSource{Kind: "local_path", LocalPath: sourceRoot},
	})
	if err != nil {
		t.Fatalf("apply install: %v", err)
	}
	pluginRoot := filepath.Join(userHome, ".cursor", "plugins", "local", "agent-code-navigator")
	if result.State != domain.InstallActivationPending {
		t.Fatalf("state = %s, want activation pending", result.State)
	}
	if !result.ReloadRequired || !result.NewThreadRequired {
		t.Fatalf("activation flags = reload:%v newThread:%v, want both true", result.ReloadRequired, result.NewThreadRequired)
	}
	if _, err := os.Stat(filepath.Join(pluginRoot, ".cursor-plugin", "plugin.json")); err != nil {
		t.Fatalf("stat cursor manifest: %v", err)
	}
	if _, err := os.Stat(filepath.Join(pluginRoot, "icon.png")); err != nil {
		t.Fatalf("stat manifest-referenced logo: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(pluginRoot, "skills", "code-tool-router", "SKILL.md"))
	if err != nil {
		t.Fatalf("read generated skill: %v", err)
	}
	if string(body) != "# Code tool router\n" {
		t.Fatalf("skill body = %q", body)
	}
	mcpBody, err := os.ReadFile(filepath.Join(pluginRoot, ".mcp.json"))
	if err != nil {
		t.Fatalf("read cursor package mcp: %v", err)
	}
	if !strings.Contains(string(mcpBody), filepath.Join(sourceRoot, "bin", "memory-mcp")) {
		t.Fatalf("mcp command was not source-root projected: %s", string(mcpBody))
	}
	if !strings.Contains(string(mcpBody), "--repo="+sourceRoot) {
		t.Fatalf("mcp args were not source-root projected: %s", string(mcpBody))
	}
	if _, err := os.Stat(filepath.Join(userHome, ".cursor", "mcp.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no Cursor MCP config for plugin package install, stat err = %v", err)
	}
	if len(result.OwnedNativeObjects) != 1 || result.OwnedNativeObjects[0].Kind != cursorPluginRootKind || result.OwnedNativeObjects[0].Path != pluginRoot {
		t.Fatalf("owned objects = %#v, want cursor plugin root", result.OwnedNativeObjects)
	}
}

func TestApplyRemoveDeletesCursorPluginRoot(t *testing.T) {
	t.Parallel()
	userHome := t.TempDir()
	pluginRoot := filepath.Join(userHome, ".cursor", "plugins", "local", "agent-code-navigator")
	mustWriteFile(t, filepath.Join(pluginRoot, ".cursor-plugin", "plugin.json"), `{"name":"agent-code-navigator"}`)

	adapter := Adapter{FS: fsadapter.OS{}, UserHome: userHome}
	record := domain.InstallationRecord{
		IntegrationID: "agent-code-navigator",
		Targets: map[domain.TargetID]domain.TargetInstallation{
			domain.TargetCursor: {
				TargetID: domain.TargetCursor,
				OwnedNativeObjects: []domain.NativeObjectRef{
					{Kind: cursorPluginRootKind, Name: "agent-code-navigator", Path: pluginRoot},
				},
			},
		},
	}
	result, err := adapter.ApplyRemove(context.Background(), ports.ApplyInput{Record: &record})
	if err != nil {
		t.Fatalf("apply remove: %v", err)
	}
	if result.State != domain.InstallRemoved {
		t.Fatalf("state = %s, want removed", result.State)
	}
	if !result.ReloadRequired {
		t.Fatal("expected Cursor reload to be required after plugin removal")
	}
	if _, err := os.Stat(pluginRoot); !os.IsNotExist(err) {
		t.Fatalf("expected plugin root to be removed, stat err = %v", err)
	}
}

func TestInspectCursorPluginRootFromRecord(t *testing.T) {
	t.Parallel()
	userHome := t.TempDir()
	pluginRoot := filepath.Join(userHome, ".cursor", "plugins", "local", "agent-code-navigator")
	mustWriteFile(t, filepath.Join(pluginRoot, ".cursor-plugin", "plugin.json"), `{"name":"agent-code-navigator"}`)

	adapter := Adapter{FS: fsadapter.OS{}, UserHome: userHome}
	inspect, err := adapter.Inspect(context.Background(), ports.InspectInput{
		Record: &domain.InstallationRecord{
			Targets: map[domain.TargetID]domain.TargetInstallation{
				domain.TargetCursor: {
					TargetID: domain.TargetCursor,
					OwnedNativeObjects: []domain.NativeObjectRef{
						{Kind: cursorPluginRootKind, Name: "agent-code-navigator", Path: pluginRoot},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if inspect.State != domain.InstallInstalled || !inspect.Installed {
		t.Fatalf("inspect state = %s installed = %v, want installed", inspect.State, inspect.Installed)
	}
	if len(inspect.ObservedNativeObjects) != 1 || inspect.ObservedNativeObjects[0].Kind != cursorPluginRootKind {
		t.Fatalf("observed native objects = %#v", inspect.ObservedNativeObjects)
	}
}

func TestInspectProjectScopeUsesPersistedWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	workspaceA := filepath.Join(root, "workspace-a")
	workspaceB := filepath.Join(root, "workspace-b")
	configPath := filepath.Join(workspaceA, ".cursor", "mcp.json")
	mustWriteFile(t, configPath, `{"mcpServers":{"release-checks":{"command":"node","args":["managed.mjs"]}}}`)

	adapter := Adapter{FS: fsadapter.OS{}, UserHome: t.TempDir()}
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(prevWD) }()
	if err := os.MkdirAll(workspaceB, 0o755); err != nil {
		t.Fatalf("mkdir workspace-b: %v", err)
	}
	if err := os.Chdir(workspaceB); err != nil {
		t.Fatalf("chdir workspace-b: %v", err)
	}

	inspect, err := adapter.Inspect(context.Background(), ports.InspectInput{
		Record: &domain.InstallationRecord{
			Policy:        domain.InstallPolicy{Scope: "project"},
			WorkspaceRoot: workspaceA,
			Targets: map[domain.TargetID]domain.TargetInstallation{
				domain.TargetCursor: {
					TargetID: domain.TargetCursor,
					OwnedNativeObjects: []domain.NativeObjectRef{
						{Kind: "cursor_mcp_server", Name: "release-checks", Path: configPath},
					},
				},
			},
		},
		Scope: "project",
	})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if inspect.State != domain.InstallInstalled {
		t.Fatalf("state = %s, want installed", inspect.State)
	}
	if len(inspect.SettingsFiles) == 0 || inspect.SettingsFiles[0] != configPath {
		t.Fatalf("settings files = %#v, want %q first", inspect.SettingsFiles, configPath)
	}
}

func TestPlanUpdateUsesPersistedWorkspaceRootConfigPath(t *testing.T) {
	root := t.TempDir()
	workspaceA := filepath.Join(root, "workspace-a")
	workspaceB := filepath.Join(root, "workspace-b")
	configPath := filepath.Join(workspaceA, ".cursor", "mcp.json")

	adapter := Adapter{UserHome: t.TempDir()}
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(prevWD) }()
	for _, dir := range []string{workspaceA, workspaceB} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %q: %v", dir, err)
		}
	}
	if err := os.Chdir(workspaceB); err != nil {
		t.Fatalf("chdir workspace-b: %v", err)
	}

	plan, err := adapter.PlanUpdate(context.Background(), ports.PlanUpdateInput{
		CurrentRecord: domain.InstallationRecord{
			Policy:        domain.InstallPolicy{Scope: "project"},
			WorkspaceRoot: workspaceA,
			Targets: map[domain.TargetID]domain.TargetInstallation{
				domain.TargetCursor: {
					TargetID: domain.TargetCursor,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("plan update: %v", err)
	}
	if len(plan.PathsTouched) != 1 || plan.PathsTouched[0] != configPath {
		t.Fatalf("paths touched = %#v, want %q", plan.PathsTouched, configPath)
	}
}

func mustWriteFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	return out
}

func mustObject(t *testing.T, value any) map[string]any {
	t.Helper()
	out, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("value is %T, want object", value)
	}
	return out
}

func mustStringSlice(t *testing.T, value any) []string {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("value is %T, want []any", value)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			t.Fatalf("item is %T, want string", item)
		}
		out = append(out, s)
	}
	return out
}
