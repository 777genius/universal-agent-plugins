package opencode

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	fsadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/fs"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestApplyInstallPatchesConfigAndCopiesAssets(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	sourceRoot := t.TempDir()
	mustWriteOpenCodeFile(t, filepath.Join(projectRoot, "opencode.json"), "{\n  // keep me\n  \"theme\": \"midnight\",\n  \"mcp\": {\"user-owned\":{\"type\":\"local\",\"command\":[\"node\",\"user.js\"]}},\n  \"plugin\": [\"@user/existing\"]\n}\n")
	mustWriteOpenCodeFile(t, filepath.Join(sourceRoot, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  context7:\n    type: stdio\n    stdio:\n      command: npx\n      args:\n        - -y\n        - '@upstash/context7-mcp'\n    targets:\n      - opencode\n")
	mustWriteOpenCodeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "package.yaml"), "plugins:\n  - '@acme/opencode-demo-plugin'\n")
	mustWriteOpenCodeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "default_agent.txt"), "reviewer\n")
	mustWriteOpenCodeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "instructions.yaml"), "- AGENTS.md\n")
	mustWriteOpenCodeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "permission.json"), "{\"bash\":\"ask\"}\n")
	mustWriteOpenCodeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "config.extra.json"), "{\"watcher\":{\"ignore\":[\"dist/**\"]}}\n")
	mustWriteOpenCodeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "plugins", "example.js"), "export const ExamplePlugin = async () => ({})\n")
	mustWriteOpenCodeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "package.json"), "{\n  \"name\": \"demo\",\n  \"private\": true\n}\n")
	mustWriteOpenCodeFile(t, filepath.Join(sourceRoot, "plugin", "skills", "demo", "SKILL.md"), "# Demo\n")

	adapter := Adapter{FS: fsadapter.OS{}, ProjectRoot: projectRoot, UserHome: t.TempDir()}
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
	body, err := os.ReadFile(filepath.Join(projectRoot, "opencode.json"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(body)
	for _, want := range []string{"\"theme\"", "midnight", "\"plugin\"", "\"mcp\"", "\"$schema\"", "\"default_agent\"", "reviewer", "\"instructions\"", "\"permission\"", "\"watcher\"", "\"@user/existing\"", "\"user-owned\""} {
		if !strings.Contains(text, want) {
			t.Fatalf("OpenCode config missing %q:\n%s", want, text)
		}
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".opencode", "plugins", "example.js")); err != nil {
		t.Fatalf("stat projected plugin: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".opencode", "package.json")); err != nil {
		t.Fatalf("stat projected package.json: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".opencode", "skills", "demo", "SKILL.md")); err != nil {
		t.Fatalf("stat projected skill: %v", err)
	}
}

func TestApplyUpdateRemovesStaleOwnedEntriesAndFiles(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	sourceRoot := t.TempDir()
	mustWriteOpenCodeFile(t, filepath.Join(projectRoot, "opencode.json"), "{\n  \"theme\": \"midnight\",\n  \"plugin\": [\"@user/existing\", \"@acme/opencode-demo-plugin\"],\n  \"mcp\": {\"user-owned\":{\"type\":\"local\",\"command\":[\"node\",\"user.js\"]},\"context7\":{\"type\":\"local\",\"command\":[\"npx\",\"-y\",\"@upstash/context7-mcp\"]}},\n  \"default_agent\": \"reviewer\"\n}\n")
	mustWriteOpenCodeFile(t, filepath.Join(projectRoot, ".opencode", "plugins", "example.js"), "old\n")
	mustWriteOpenCodeFile(t, filepath.Join(projectRoot, ".opencode", "plugins", "stale.js"), "stale\n")
	mustWriteOpenCodeFile(t, filepath.Join(projectRoot, ".opencode", "package.json"), "{\n  \"name\": \"demo-old\"\n}\n")
	mustWriteOpenCodeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "package.yaml"), "plugins:\n  - '@acme/opencode-next-plugin'\n")
	mustWriteOpenCodeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "plugins", "example.js"), "new\n")
	mustWriteOpenCodeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "package.json"), "{\n  \"name\": \"demo-next\"\n}\n")
	mustWriteOpenCodeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "permission.json"), "{\"edit\":\"allow\"}\n")

	adapter := Adapter{FS: fsadapter.OS{}, ProjectRoot: projectRoot, UserHome: t.TempDir()}
	result, err := adapter.ApplyUpdate(context.Background(), ports.ApplyInput{
		Record: &domain.InstallationRecord{
			Policy: domain.InstallPolicy{Scope: "project"},
			Targets: map[domain.TargetID]domain.TargetInstallation{
				domain.TargetOpenCode: {
					TargetID:           domain.TargetOpenCode,
					OwnedNativeObjects: ownedObjects(filepath.Join(projectRoot, "opencode.json"), []string{"default_agent"}, []string{"@acme/opencode-demo-plugin"}, []string{"context7"}, []string{filepath.Join(projectRoot, ".opencode", "plugins", "example.js"), filepath.Join(projectRoot, ".opencode", "plugins", "stale.js"), filepath.Join(projectRoot, ".opencode", "package.json")}, domain.ProtectionWorkspace),
				},
			},
		},
		ResolvedSource: &ports.ResolvedSource{Kind: "local_path", LocalPath: sourceRoot},
	})
	if err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if result.State != domain.InstallInstalled {
		t.Fatalf("state = %s, want installed", result.State)
	}
	body, err := os.ReadFile(filepath.Join(projectRoot, "opencode.json"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(body)
	for _, want := range []string{"\"@user/existing\"", "\"@acme/opencode-next-plugin\"", "\"user-owned\"", "\"permission\""} {
		if !strings.Contains(text, want) {
			t.Fatalf("updated config missing %q:\n%s", want, text)
		}
	}
	for _, notWant := range []string{"\"@acme/opencode-demo-plugin\"", "\"context7\"", "\"default_agent\""} {
		if strings.Contains(text, notWant) {
			t.Fatalf("updated config still contains %q:\n%s", notWant, text)
		}
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".opencode", "plugins", "stale.js")); !os.IsNotExist(err) {
		t.Fatalf("stale projected file still exists: %v", err)
	}
}

func TestApplyRemoveDeletesOnlyOwnedEntries(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	configPath := filepath.Join(projectRoot, "opencode.json")
	mustWriteOpenCodeFile(t, configPath, "{\n  \"theme\": \"midnight\",\n  \"plugin\": [\"@user/existing\", \"@acme/opencode-demo-plugin\"],\n  \"mcp\": {\"user-owned\":{\"type\":\"local\",\"command\":[\"node\",\"user.js\"]},\"context7\":{\"type\":\"local\",\"command\":[\"npx\",\"-y\",\"@upstash/context7-mcp\"]}},\n  \"permission\": {\"bash\":\"ask\"}\n}\n")
	mustWriteOpenCodeFile(t, filepath.Join(projectRoot, ".opencode", "plugins", "example.js"), "owned\n")

	adapter := Adapter{FS: fsadapter.OS{}, ProjectRoot: projectRoot, UserHome: t.TempDir()}
	_, err := adapter.ApplyRemove(context.Background(), ports.ApplyInput{
		Record: &domain.InstallationRecord{
			Policy: domain.InstallPolicy{Scope: "project"},
			Targets: map[domain.TargetID]domain.TargetInstallation{
				domain.TargetOpenCode: {
					TargetID:           domain.TargetOpenCode,
					OwnedNativeObjects: ownedObjects(configPath, []string{"permission"}, []string{"@acme/opencode-demo-plugin"}, []string{"context7"}, []string{filepath.Join(projectRoot, ".opencode", "plugins", "example.js")}, domain.ProtectionWorkspace),
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("apply remove: %v", err)
	}
	body, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(body)
	for _, want := range []string{"\"theme\"", "midnight", "\"@user/existing\"", "\"user-owned\""} {
		if !strings.Contains(text, want) {
			t.Fatalf("remove config missing %q:\n%s", want, text)
		}
	}
	for _, notWant := range []string{"\"@acme/opencode-demo-plugin\"", "\"context7\"", "\"permission\""} {
		if strings.Contains(text, notWant) {
			t.Fatalf("remove config still contains %q:\n%s", notWant, text)
		}
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".opencode", "plugins", "example.js")); !os.IsNotExist(err) {
		t.Fatalf("owned projected file still exists: %v", err)
	}
}

func TestProjectScopeUsesNearestGitRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	projectRoot := filepath.Join(root, "repo", "nested", "pkg")
	mustWriteOpenCodeFile(t, filepath.Join(root, "repo", ".git"), "")
	adapter := Adapter{FS: fsadapter.OS{}, ProjectRoot: projectRoot, UserHome: t.TempDir()}

	got := adapter.configPath("project", "")
	want := filepath.Join(root, "repo", "opencode.json")
	if got != want {
		t.Fatalf("config path = %s, want %s", got, want)
	}
}

func TestPlanUpdateUsesMetadataConfigPathAndPersistedWorkspaceRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	workspaceRoot := filepath.Join(root, "repo", "nested", "pkg")
	metadataConfig := filepath.Join(root, "repo", "configs", "opencode.custom.jsonc")
	mustWriteOpenCodeFile(t, filepath.Join(root, "repo", ".git"), "")

	adapter := Adapter{FS: fsadapter.OS{}, UserHome: t.TempDir()}
	plan, err := adapter.PlanUpdate(context.Background(), ports.PlanUpdateInput{
		CurrentRecord: domain.InstallationRecord{
			Policy:        domain.InstallPolicy{Scope: "project"},
			WorkspaceRoot: workspaceRoot,
			Targets: map[domain.TargetID]domain.TargetInstallation{
				domain.TargetOpenCode: {
					TargetID: domain.TargetOpenCode,
					AdapterMetadata: map[string]any{
						"config_path": metadataConfig,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("plan update: %v", err)
	}
	wantPaths := []string{
		metadataConfig,
		filepath.Join(root, "repo", ".opencode"),
	}
	if len(plan.PathsTouched) != len(wantPaths) {
		t.Fatalf("paths touched len = %d, want %d: %#v", len(plan.PathsTouched), len(wantPaths), plan.PathsTouched)
	}
	for i, want := range wantPaths {
		if plan.PathsTouched[i] != want {
			t.Fatalf("paths touched[%d] = %s, want %s", i, plan.PathsTouched[i], want)
		}
	}
}

func TestInspectProjectScopeUsesPersistedWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	workspaceA := filepath.Join(root, "workspace-a")
	workspaceB := filepath.Join(root, "workspace-b")
	configPath := filepath.Join(workspaceA, "opencode.json")
	mustWriteOpenCodeFile(t, configPath, "{\n  \"plugin\": [\"@acme/opencode-demo-plugin\"]\n}\n")

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
				domain.TargetOpenCode: {
					TargetID:           domain.TargetOpenCode,
					OwnedNativeObjects: ownedObjects(configPath, nil, []string{"@acme/opencode-demo-plugin"}, nil, nil, domain.ProtectionWorkspace),
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

func TestInspectUsesMetadataConfigPathFallback(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	workspaceRoot := filepath.Join(root, "workspace")
	configPath := filepath.Join(root, "custom", "opencode.jsonc")
	mustWriteOpenCodeFile(t, configPath, "{\n  \"plugin\": [\"@acme/opencode-demo-plugin\"]\n}\n")

	adapter := Adapter{FS: fsadapter.OS{}, UserHome: t.TempDir()}
	inspect, err := adapter.Inspect(context.Background(), ports.InspectInput{
		Record: &domain.InstallationRecord{
			Policy:        domain.InstallPolicy{Scope: "project"},
			WorkspaceRoot: workspaceRoot,
			Targets: map[domain.TargetID]domain.TargetInstallation{
				domain.TargetOpenCode: {
					TargetID: domain.TargetOpenCode,
					AdapterMetadata: map[string]any{
						"config_path":       configPath,
						"owned_plugin_refs": []string{"@acme/opencode-demo-plugin"},
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
	if len(inspect.ObservedNativeObjects) == 0 {
		t.Fatalf("observed objects = %#v, want managed plugin ref", inspect.ObservedNativeObjects)
	}
	if inspect.ObservedNativeObjects[0].Path != configPath {
		t.Fatalf("observed path = %s, want %s", inspect.ObservedNativeObjects[0].Path, configPath)
	}
}

func TestInspectManagedSurfaceLayerMarksReadOnlySourceAccess(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	managedPath := filepath.Join(root, "managed-opencode.json")
	mustWriteOpenCodeFile(t, managedPath, "{}\n")

	adapter := Adapter{
		FS:                   fsadapter.OS{},
		ProjectRoot:          t.TempDir(),
		UserHome:             t.TempDir(),
		managedConfigPathsFn: func(string) []string { return []string{managedPath} },
	}
	restrictions, sourceAccess, managedPaths := adapter.inspectManagedSurfaceLayer()
	if len(restrictions) != 1 || restrictions[0] != domain.RestrictionReadOnlyNativeLayer {
		t.Fatalf("restrictions = %#v", restrictions)
	}
	if sourceAccess != "managed_config_layer" {
		t.Fatalf("source access = %q", sourceAccess)
	}
	if len(managedPaths) != 1 || managedPaths[0] != managedPath {
		t.Fatalf("managed paths = %#v", managedPaths)
	}
}

func TestPlanBlockingManualStepsIncludesManagedLayerGuidance(t *testing.T) {
	t.Parallel()
	steps, blocking := planBlockingManualSteps(ports.InspectResult{
		EnvironmentRestrictions: []domain.EnvironmentRestrictionCode{domain.RestrictionReadOnlyNativeLayer},
	})
	if !blocking {
		t.Fatal("expected blocking for read-only native layer")
	}
	if len(steps) == 0 || !strings.Contains(strings.Join(steps, "\n"), "administrator") {
		t.Fatalf("steps = %#v", steps)
	}
}

func mustWriteOpenCodeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}
