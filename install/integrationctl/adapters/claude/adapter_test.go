package claude

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	fsadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/fs"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type stubRunner struct {
	commands []ports.Command
	run      func(ports.Command) (ports.CommandResult, error)
}

func (s *stubRunner) Run(_ context.Context, cmd ports.Command) (ports.CommandResult, error) {
	s.commands = append(s.commands, cmd)
	if s.run != nil {
		return s.run(cmd)
	}
	return ports.CommandResult{ExitCode: 0}, nil
}

func TestApplyInstallLocalUsesManagedMarketplaceAndPluginInstall(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	source := filepath.Join(root, "source")
	writeClaudeFile(t, filepath.Join(source, "plugin", "plugin.yaml"), "api_version: v1\nname: claude-demo\nversion: 0.1.0\ndescription: Claude demo plugin\ntargets:\n  - claude\n")
	writeClaudeFile(t, filepath.Join(source, "plugin", "skills", "demo", "SKILL.md"), "# Demo\n")
	writeClaudeFile(t, filepath.Join(source, "plugin", "targets", "claude", "settings.json"), "{\n  \"agent\": \"reviewer\"\n}\n")
	writeClaudeFile(t, filepath.Join(source, "plugin", "targets", "claude", "user-config.json"), "{\n  \"mode\": \"strict\"\n}\n")
	writeClaudeFile(t, filepath.Join(source, "plugin", "targets", "claude", "commands", "review.md"), "# review\n")
	writeClaudeFile(t, filepath.Join(source, "plugin", "targets", "claude", "agents", "reviewer.md"), "# reviewer\n")
	writeClaudeFile(t, filepath.Join(source, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  docs:\n    type: remote\n    remote:\n      protocol: streamable_http\n      url: \"https://example.com/mcp\"\n    targets:\n      - claude\n  checks:\n    type: stdio\n    stdio:\n      command: ${package.root}/bin/checks\n      args:\n        - --script=${package.root}/run.mjs\n    targets:\n      - claude\n")

	runner := &stubRunner{}
	adapter := Adapter{Runner: runner, FS: fsadapter.OS{}, ProjectRoot: filepath.Join(root, "project"), UserHome: home}

	result, err := adapter.ApplyInstall(context.Background(), ports.ApplyInput{
		Manifest: domain.IntegrationManifest{
			IntegrationID: "claude-demo",
			Version:       "0.1.0",
			Description:   "Claude demo plugin",
		},
		ResolvedSource: &ports.ResolvedSource{Kind: "local_path", LocalPath: source},
		Policy:         domain.InstallPolicy{Scope: "project"},
	})
	if err != nil {
		t.Fatalf("apply install: %v", err)
	}
	if len(runner.commands) != 2 {
		t.Fatalf("commands = %#v", runner.commands)
	}
	managedRoot := filepath.Join(home, ".plugin-kit-ai", "materialized", "claude", "claude-demo")
	marketplaceName := "integrationctl-claude-demo"
	if got := runner.commands[0].Argv; !equalStrings(got, []string{"claude", "plugin", "marketplace", "add", managedRoot}) {
		t.Fatalf("marketplace add argv = %#v", got)
	}
	if got := runner.commands[1].Argv; !equalStrings(got, []string{"claude", "plugin", "install", "claude-demo@" + marketplaceName, "--scope", "project"}) {
		t.Fatalf("plugin install argv = %#v", got)
	}
	catalogBody, err := os.ReadFile(filepath.Join(managedRoot, ".claude-plugin", "marketplace.json"))
	if err != nil {
		t.Fatalf("read marketplace.json: %v", err)
	}
	var catalog map[string]any
	if err := json.Unmarshal(catalogBody, &catalog); err != nil {
		t.Fatalf("parse marketplace.json: %v", err)
	}
	if catalog["name"] != marketplaceName {
		t.Fatalf("marketplace name = %#v", catalog["name"])
	}
	pluginManifestBody, err := os.ReadFile(filepath.Join(managedRoot, "plugins", "claude-demo", ".claude-plugin", "plugin.json"))
	if err != nil {
		t.Fatalf("read plugin manifest: %v", err)
	}
	var pluginDoc map[string]any
	if err := json.Unmarshal(pluginManifestBody, &pluginDoc); err != nil {
		t.Fatalf("parse plugin manifest: %v", err)
	}
	if pluginDoc["name"] != "claude-demo" || pluginDoc["skills"] != "./skills/" || pluginDoc["agents"] != "./agents/" || pluginDoc["mcpServers"] != "./.mcp.json" {
		t.Fatalf("plugin manifest = %#v", pluginDoc)
	}
	if _, err := os.Stat(filepath.Join(managedRoot, "plugins", "claude-demo", "skills", "demo", "SKILL.md")); err != nil {
		t.Fatalf("stat skill: %v", err)
	}
	if _, err := os.Stat(filepath.Join(managedRoot, "plugins", "claude-demo", "commands", "review.md")); err != nil {
		t.Fatalf("stat command: %v", err)
	}
	mcpBody, err := os.ReadFile(filepath.Join(managedRoot, "plugins", "claude-demo", ".mcp.json"))
	if err != nil {
		t.Fatalf("read .mcp.json: %v", err)
	}
	var mcp map[string]map[string]any
	if err := json.Unmarshal(mcpBody, &mcp); err != nil {
		t.Fatalf("parse .mcp.json: %v", err)
	}
	if mcp["docs"]["type"] != "http" || mcp["checks"]["command"] != filepath.Join(source, "bin", "checks") {
		t.Fatalf("mcp projection = %#v", mcp)
	}
	args, _ := mcp["checks"]["args"].([]any)
	if len(args) != 1 || args[0] != "--script="+filepath.Join(source, "run.mjs") {
		t.Fatalf("mcp args = %#v", mcp["checks"]["args"])
	}
	if result.State != domain.InstallInstalled || !result.ReloadRequired {
		t.Fatalf("result = %+v", result)
	}
}

func TestApplyInstallRollbackRemovesMarketplaceWhenPluginInstallFails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	source := filepath.Join(root, "source")
	writeClaudeFile(t, filepath.Join(source, ".claude-plugin", "plugin.json"), "{\n  \"name\": \"claude-demo\",\n  \"version\": \"0.1.0\",\n  \"description\": \"demo\"\n}\n")
	runner := &stubRunner{
		run: func(cmd ports.Command) (ports.CommandResult, error) {
			if len(cmd.Argv) >= 3 && cmd.Argv[0] == "claude" && cmd.Argv[1] == "plugin" && cmd.Argv[2] == "install" {
				return ports.CommandResult{ExitCode: 1, Stderr: []byte("install failed")}, nil
			}
			return ports.CommandResult{ExitCode: 0}, nil
		},
	}
	adapter := Adapter{Runner: runner, FS: fsadapter.OS{}, UserHome: home}
	_, err := adapter.ApplyInstall(context.Background(), ports.ApplyInput{
		Manifest:       domain.IntegrationManifest{IntegrationID: "claude-demo", Version: "0.1.0", Description: "demo"},
		ResolvedSource: &ports.ResolvedSource{Kind: "local_path", LocalPath: source},
		Policy:         domain.InstallPolicy{Scope: "user"},
	})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.ErrMutationApply {
		t.Fatalf("err = %#v, want mutation apply", err)
	}
	if len(runner.commands) != 3 {
		t.Fatalf("commands = %#v", runner.commands)
	}
	if got := runner.commands[2].Argv; !equalStrings(got, []string{"claude", "plugin", "marketplace", "remove", "integrationctl-claude-demo"}) {
		t.Fatalf("rollback argv = %#v", got)
	}
}

func TestApplyInstallRejectsManagedManifestExtraOverride(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	source := filepath.Join(root, "source")
	writeClaudeFile(t, filepath.Join(source, "plugin", "plugin.yaml"), "api_version: v1\nname: claude-demo\nversion: 0.1.0\ndescription: Claude demo plugin\ntargets:\n  - claude\n")
	writeClaudeFile(t, filepath.Join(source, "plugin", "targets", "claude", "manifest.extra.json"), "{\n  \"name\": \"override\"\n}\n")

	adapter := Adapter{Runner: &stubRunner{}, FS: fsadapter.OS{}, ProjectRoot: filepath.Join(root, "project"), UserHome: home}
	_, err := adapter.ApplyInstall(context.Background(), ports.ApplyInput{
		Manifest: domain.IntegrationManifest{
			IntegrationID: "claude-demo",
			Version:       "0.1.0",
			Description:   "Claude demo plugin",
		},
		ResolvedSource: &ports.ResolvedSource{Kind: "local_path", LocalPath: source},
		Policy:         domain.InstallPolicy{Scope: "project"},
	})
	if err == nil {
		t.Fatal("expected manifest.extra override error")
	}
	if !strings.Contains(err.Error(), "Claude manifest.extra.json may not override managed key name") {
		t.Fatalf("error = %v", err)
	}
}

func TestPlanInstallBlocksWhenManagedSettingsDisallowMarketplaceAdd(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	writeClaudeFile(t, filepath.Join(home, ".claude", "managed-settings.json"), "{\n  \"strictKnownMarketplaces\": []\n}\n")
	runner := &stubRunner{
		run: func(cmd ports.Command) (ports.CommandResult, error) {
			if equalStrings(cmd.Argv, []string{"claude", "plugin", "list", "--json"}) {
				return ports.CommandResult{ExitCode: 0, Stdout: []byte("[]")}, nil
			}
			return ports.CommandResult{ExitCode: 0}, nil
		},
	}
	adapter := Adapter{Runner: runner, FS: fsadapter.OS{}, UserHome: home}

	inspect, err := adapter.Inspect(context.Background(), ports.InspectInput{Scope: "user"})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if !containsRestriction(inspect.EnvironmentRestrictions, domain.RestrictionManagedPolicyBlock) {
		t.Fatalf("expected managed policy block, got %#v", inspect.EnvironmentRestrictions)
	}
	plan, err := adapter.PlanInstall(context.Background(), ports.PlanInstallInput{
		Manifest: domain.IntegrationManifest{IntegrationID: "claude-demo"},
		Policy:   domain.InstallPolicy{Scope: "user"},
		Inspect:  inspect,
	})
	if err != nil {
		t.Fatalf("plan install: %v", err)
	}
	if !plan.Blocking {
		t.Fatal("expected plan to be blocking")
	}
	if len(plan.ManualSteps) == 0 || !strings.Contains(strings.Join(plan.ManualSteps, " "), "strictKnownMarketplaces") {
		t.Fatalf("manual steps = %#v", plan.ManualSteps)
	}
}

func TestInspectFlagsManagedPolicyBlockWhenPathPatternDisallowsMarketplace(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	writeClaudeFile(t, filepath.Join(home, ".claude", "managed-settings.json"), "{\n  \"strictKnownMarketplaces\": [\n    {\"source\": \"pathPattern\", \"pathPattern\": \"^/tmp/other$\"}\n  ]\n}\n")
	adapter := Adapter{FS: fsadapter.OS{}, UserHome: home}

	inspect, err := adapter.Inspect(context.Background(), ports.InspectInput{Scope: "user", IntegrationID: "claude-demo"})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if !containsRestriction(inspect.EnvironmentRestrictions, domain.RestrictionManagedPolicyBlock) {
		t.Fatalf("expected managed policy block, got %#v", inspect.EnvironmentRestrictions)
	}
}

func TestInspectProjectScopeUsesPersistedWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	workspaceA := filepath.Join(root, "workspace-a")
	workspaceB := filepath.Join(root, "workspace-b")
	binDir := filepath.Join(root, "bin")
	settingsPath := filepath.Join(workspaceA, ".claude", "settings.json")
	writeClaudeFile(t, settingsPath, "{\n  \"plugins\": []\n}\n")
	adapter := Adapter{FS: fsadapter.OS{}, UserHome: t.TempDir()}

	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(prevWD) }()
	for _, dir := range []string{workspaceB, binDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %q: %v", dir, err)
		}
	}
	t.Setenv("PATH", binDir)
	if err := os.Chdir(workspaceB); err != nil {
		t.Fatalf("chdir workspace-b: %v", err)
	}

	inspect, err := adapter.Inspect(context.Background(), ports.InspectInput{
		Scope: "project",
		Record: &domain.InstallationRecord{
			IntegrationID: "claude-demo",
			Policy:        domain.InstallPolicy{Scope: "project"},
			WorkspaceRoot: workspaceA,
		},
	})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if inspect.State != domain.InstallInstalled {
		t.Fatalf("state = %s, want installed", inspect.State)
	}
	if len(inspect.SettingsFiles) == 0 || inspect.SettingsFiles[0] != settingsPath {
		t.Fatalf("settings files = %#v, want %q first", inspect.SettingsFiles, settingsPath)
	}
}

func TestPlanUpdateUsesPersistedWorkspaceRootSettingsPath(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	workspaceA := filepath.Join(root, "workspace-a")
	workspaceB := filepath.Join(root, "workspace-b")
	settingsPath := filepath.Join(workspaceA, ".claude", "settings.json")
	adapter := Adapter{UserHome: home}

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
			IntegrationID: "claude-demo",
			Policy:        domain.InstallPolicy{Scope: "project"},
			WorkspaceRoot: workspaceA,
		},
		NextManifest: domain.IntegrationManifest{IntegrationID: "claude-demo", Version: "0.2.0"},
	})
	if err != nil {
		t.Fatalf("plan update: %v", err)
	}
	if len(plan.PathsTouched) != 2 {
		t.Fatalf("paths touched = %#v", plan.PathsTouched)
	}
	if plan.PathsTouched[1] != settingsPath {
		t.Fatalf("settings path = %q, want %q", plan.PathsTouched[1], settingsPath)
	}
}

func TestInspectInstalledClaudeStillFlagsMissingCLIAndBlocksMutation(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	settingsPath := filepath.Join(workspace, ".claude", "settings.json")
	binDir := filepath.Join(root, "bin")
	writeClaudeFile(t, settingsPath, "{\n  \"plugins\": []\n}\n")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	t.Setenv("PATH", binDir)

	adapter := Adapter{FS: fsadapter.OS{}, UserHome: t.TempDir()}
	record := domain.InstallationRecord{
		IntegrationID: "claude-demo",
		Policy:        domain.InstallPolicy{Scope: "project"},
		WorkspaceRoot: workspace,
	}

	inspect, err := adapter.Inspect(context.Background(), ports.InspectInput{
		Scope:         "project",
		IntegrationID: "claude-demo",
		Record:        &record,
	})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if inspect.State != domain.InstallInstalled {
		t.Fatalf("state = %s, want installed", inspect.State)
	}
	if !containsRestriction(inspect.EnvironmentRestrictions, domain.RestrictionSourceToolMissing) {
		t.Fatalf("expected source tool missing restriction, got %#v", inspect.EnvironmentRestrictions)
	}

	plan, err := adapter.PlanUpdate(context.Background(), ports.PlanUpdateInput{
		CurrentRecord: record,
		NextManifest:  domain.IntegrationManifest{IntegrationID: "claude-demo", Version: "0.2.0"},
		Inspect:       inspect,
	})
	if err != nil {
		t.Fatalf("plan update: %v", err)
	}
	if !plan.Blocking {
		t.Fatal("expected update plan to be blocking when Claude CLI is missing")
	}
	if len(plan.ManualSteps) == 0 || !strings.Contains(strings.Join(plan.ManualSteps, " "), "PATH") {
		t.Fatalf("manual steps = %#v", plan.ManualSteps)
	}
	if len(inspect.SettingsFiles) == 0 || inspect.SettingsFiles[0] != settingsPath {
		t.Fatalf("settings files = %#v, want %q first", inspect.SettingsFiles, settingsPath)
	}
}

func TestInspectUsesNativePluginListForKnownPluginState(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace-a")
	settingsPath := filepath.Join(workspace, ".claude", "settings.json")
	writeClaudeFile(t, settingsPath, "{\n  \"enabledPlugins\": {\n    \"claude-demo@integrationctl-claude-demo\": true\n  }\n}\n")
	binDir := filepath.Join(root, "bin")
	writeClaudeFile(t, filepath.Join(binDir, "claude"), "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(filepath.Join(binDir, "claude"), 0o755); err != nil {
		t.Fatalf("chmod claude shim: %v", err)
	}
	prevPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+prevPath)
	runner := &stubRunner{
		run: func(cmd ports.Command) (ports.CommandResult, error) {
			if !equalStrings(cmd.Argv, []string{"claude", "plugin", "list", "--json"}) {
				t.Fatalf("argv = %#v", cmd.Argv)
			}
			return ports.CommandResult{ExitCode: 0, Stdout: []byte("[]")}, nil
		},
	}
	adapter := Adapter{Runner: runner, FS: fsadapter.OS{}, UserHome: t.TempDir()}

	inspect, err := adapter.Inspect(context.Background(), ports.InspectInput{
		Scope:         "project",
		IntegrationID: "claude-demo",
		Record: &domain.InstallationRecord{
			IntegrationID: "claude-demo",
			Policy:        domain.InstallPolicy{Scope: "project"},
			WorkspaceRoot: workspace,
			Targets: map[domain.TargetID]domain.TargetInstallation{
				domain.TargetClaude: {
					TargetID: domain.TargetClaude,
					AdapterMetadata: map[string]any{
						"plugin_ref": "claude-demo@integrationctl-claude-demo",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if inspect.State != domain.InstallRemoved {
		t.Fatalf("state = %s, want removed from native plugin list", inspect.State)
	}
	if len(runner.commands) != 1 || runner.commands[0].Dir != workspace {
		t.Fatalf("commands = %#v", runner.commands)
	}
}

func TestResolveInspectIntegrationIDPrefersExplicitInput(t *testing.T) {
	t.Parallel()

	got := resolveInspectIntegrationID(ports.InspectInput{
		IntegrationID: "explicit",
		Record:        &domain.InstallationRecord{IntegrationID: "record"},
	})
	if got != "explicit" {
		t.Fatalf("integration id = %q", got)
	}
}

func TestPlanUpdateBlocksWhenMarketplaceIsSeedManaged(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	seed := filepath.Join(root, "seed")
	binDir := filepath.Join(root, "bin")
	writeClaudeFile(t, filepath.Join(seed, "marketplaces", "integrationctl-claude-demo", ".claude-plugin", "marketplace.json"), "{\n  \"name\": \"integrationctl-claude-demo\"\n}\n")
	writeClaudeFile(t, filepath.Join(binDir, "claude"), "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(filepath.Join(binDir, "claude"), 0o755); err != nil {
		t.Fatalf("chmod claude shim: %v", err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("CLAUDE_CODE_PLUGIN_SEED_DIR", seed)
	runner := &stubRunner{
		run: func(cmd ports.Command) (ports.CommandResult, error) {
			if equalStrings(cmd.Argv, []string{"claude", "plugin", "list", "--json"}) {
				return ports.CommandResult{ExitCode: 0, Stdout: []byte("[]")}, nil
			}
			return ports.CommandResult{ExitCode: 0}, nil
		},
	}
	adapter := Adapter{Runner: runner, FS: fsadapter.OS{}, UserHome: home}

	inspect, err := adapter.Inspect(context.Background(), ports.InspectInput{
		Scope:         "user",
		IntegrationID: "claude-demo",
		Record: &domain.InstallationRecord{
			IntegrationID: "claude-demo",
			Policy:        domain.InstallPolicy{Scope: "user"},
			Targets: map[domain.TargetID]domain.TargetInstallation{
				domain.TargetClaude: {
					TargetID: domain.TargetClaude,
					AdapterMetadata: map[string]any{
						"marketplace_name": "integrationctl-claude-demo",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if !containsRestriction(inspect.EnvironmentRestrictions, domain.RestrictionReadOnlyNativeLayer) {
		t.Fatalf("expected read-only restriction, got %#v", inspect.EnvironmentRestrictions)
	}
	plan, err := adapter.PlanUpdate(context.Background(), ports.PlanUpdateInput{
		CurrentRecord: domain.InstallationRecord{
			IntegrationID: "claude-demo",
			Policy:        domain.InstallPolicy{Scope: "user"},
		},
		NextManifest: domain.IntegrationManifest{IntegrationID: "claude-demo", Version: "0.2.0"},
		Inspect:      inspect,
	})
	if err != nil {
		t.Fatalf("plan update: %v", err)
	}
	if !plan.Blocking {
		t.Fatal("expected seed-managed update to be blocking")
	}
	if len(plan.ManualSteps) == 0 || !strings.Contains(strings.Join(plan.ManualSteps, " "), "seed-managed") {
		t.Fatalf("manual steps = %#v", plan.ManualSteps)
	}
}

func TestApplyUpdateUsesMarketplaceRefreshAndReinstall(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	source := filepath.Join(root, "source")
	workspaceA := filepath.Join(root, "workspace-a")
	workspaceB := filepath.Join(root, "workspace-b")
	if err := os.MkdirAll(workspaceA, 0o755); err != nil {
		t.Fatalf("mkdir workspace-a: %v", err)
	}
	if err := os.MkdirAll(workspaceB, 0o755); err != nil {
		t.Fatalf("mkdir workspace-b: %v", err)
	}
	writeClaudeFile(t, filepath.Join(source, "plugin", "plugin.yaml"), "api_version: v1\nname: claude-demo\nversion: 0.2.0\ndescription: Claude demo plugin\ntargets:\n  - claude\n")
	writeClaudeFile(t, filepath.Join(source, "plugin", "skills", "demo", "SKILL.md"), "# Updated\n")
	runner := &stubRunner{}
	adapter := Adapter{Runner: runner, FS: fsadapter.OS{}, UserHome: home}
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(prevWD) }()
	if err := os.Chdir(workspaceB); err != nil {
		t.Fatalf("chdir workspace-b: %v", err)
	}

	result, err := adapter.ApplyUpdate(context.Background(), ports.ApplyInput{
		Manifest:       domain.IntegrationManifest{IntegrationID: "claude-demo", Version: "0.2.0", Description: "Claude demo plugin"},
		ResolvedSource: &ports.ResolvedSource{Kind: "local_path", LocalPath: source},
		Record: &domain.InstallationRecord{
			IntegrationID: "claude-demo",
			Policy:        domain.InstallPolicy{Scope: "project"},
			WorkspaceRoot: workspaceA,
			Targets: map[domain.TargetID]domain.TargetInstallation{
				domain.TargetClaude: {
					TargetID: domain.TargetClaude,
					AdapterMetadata: map[string]any{
						"marketplace_name":         "integrationctl-claude-demo",
						"plugin_ref":               "claude-demo@integrationctl-claude-demo",
						"materialized_source_root": filepath.Join(home, ".plugin-kit-ai", "materialized", "claude", "claude-demo"),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("apply update: %v", err)
	}
	want := [][]string{
		{"claude", "plugin", "marketplace", "update", "integrationctl-claude-demo"},
		{"claude", "plugin", "uninstall", "claude-demo@integrationctl-claude-demo", "--scope", "project"},
		{"claude", "plugin", "install", "claude-demo@integrationctl-claude-demo", "--scope", "project"},
	}
	if len(runner.commands) != len(want) {
		t.Fatalf("commands = %#v", runner.commands)
	}
	for i := range want {
		if !equalStrings(runner.commands[i].Argv, want[i]) {
			t.Fatalf("command %d = %#v want %#v", i, runner.commands[i].Argv, want[i])
		}
		if runner.commands[i].Dir != workspaceA {
			t.Fatalf("command %d dir = %q want %q", i, runner.commands[i].Dir, workspaceA)
		}
	}
	body, err := os.ReadFile(filepath.Join(home, ".plugin-kit-ai", "materialized", "claude", "claude-demo", "plugins", "claude-demo", "skills", "demo", "SKILL.md"))
	if err != nil || !strings.Contains(string(body), "Updated") {
		t.Fatalf("managed source not refreshed: %v %q", err, body)
	}
	if result.State != domain.InstallInstalled || !result.ReloadRequired {
		t.Fatalf("result = %+v", result)
	}
}

func TestApplyRemoveUsesUninstallThenMarketplaceRemove(t *testing.T) {
	root := t.TempDir()
	workspaceA := filepath.Join(root, "workspace-a")
	workspaceB := filepath.Join(root, "workspace-b")
	if err := os.MkdirAll(workspaceA, 0o755); err != nil {
		t.Fatalf("mkdir workspace-a: %v", err)
	}
	if err := os.MkdirAll(workspaceB, 0o755); err != nil {
		t.Fatalf("mkdir workspace-b: %v", err)
	}
	managedRoot := filepath.Join(root, ".plugin-kit-ai", "materialized", "claude", "claude-demo")
	writeClaudeFile(t, filepath.Join(managedRoot, ".claude-plugin", "marketplace.json"), "{}\n")
	runner := &stubRunner{}
	adapter := Adapter{Runner: runner, UserHome: root}
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(prevWD) }()
	if err := os.Chdir(workspaceB); err != nil {
		t.Fatalf("chdir workspace-b: %v", err)
	}

	result, err := adapter.ApplyRemove(context.Background(), ports.ApplyInput{
		Record: &domain.InstallationRecord{
			IntegrationID: "claude-demo",
			Policy:        domain.InstallPolicy{Scope: "project"},
			WorkspaceRoot: workspaceA,
			Targets: map[domain.TargetID]domain.TargetInstallation{
				domain.TargetClaude: {
					TargetID: domain.TargetClaude,
					AdapterMetadata: map[string]any{
						"marketplace_name":         "integrationctl-claude-demo",
						"plugin_ref":               "claude-demo@integrationctl-claude-demo",
						"materialized_source_root": managedRoot,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("apply remove: %v", err)
	}
	want := [][]string{
		{"claude", "plugin", "uninstall", "claude-demo@integrationctl-claude-demo", "--scope", "project"},
		{"claude", "plugin", "marketplace", "remove", "integrationctl-claude-demo"},
	}
	if len(runner.commands) != len(want) {
		t.Fatalf("commands = %#v", runner.commands)
	}
	for i := range want {
		if !equalStrings(runner.commands[i].Argv, want[i]) {
			t.Fatalf("command %d = %#v want %#v", i, runner.commands[i].Argv, want[i])
		}
		if runner.commands[i].Dir != workspaceA {
			t.Fatalf("command %d dir = %q want %q", i, runner.commands[i].Dir, workspaceA)
		}
	}
	if _, err := os.Stat(managedRoot); !os.IsNotExist(err) {
		t.Fatalf("managed root still exists: %v", err)
	}
	if result.State != domain.InstallRemoved {
		t.Fatalf("result = %+v", result)
	}
}

func TestRepairUsesMarketplaceRefreshAndBestEffortUninstall(t *testing.T) {
	root := t.TempDir()
	workspaceA := filepath.Join(root, "workspace-a")
	workspaceB := filepath.Join(root, "workspace-b")
	if err := os.MkdirAll(workspaceA, 0o755); err != nil {
		t.Fatalf("mkdir workspace-a: %v", err)
	}
	if err := os.MkdirAll(workspaceB, 0o755); err != nil {
		t.Fatalf("mkdir workspace-b: %v", err)
	}
	runner := &stubRunner{}
	adapter := Adapter{Runner: runner, FS: fsadapter.OS{}, UserHome: filepath.Join(root, "home")}
	source := filepath.Join(root, "source")
	writeClaudeFile(t, filepath.Join(source, "plugin", "skills", "demo", "SKILL.md"), "# Demo\n")
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(prevWD) }()
	if err := os.Chdir(workspaceB); err != nil {
		t.Fatalf("chdir workspace-b: %v", err)
	}
	_, err = adapter.Repair(context.Background(), ports.RepairInput{
		Record: domain.InstallationRecord{
			IntegrationID: "claude-demo",
			Policy:        domain.InstallPolicy{Scope: "project"},
			WorkspaceRoot: workspaceA,
			Targets: map[domain.TargetID]domain.TargetInstallation{
				domain.TargetClaude: {
					TargetID: domain.TargetClaude,
					AdapterMetadata: map[string]any{
						"marketplace_name": "integrationctl-claude-demo",
						"plugin_ref":       "claude-demo@integrationctl-claude-demo",
					},
				},
			},
		},
		Manifest:       &domain.IntegrationManifest{IntegrationID: "claude-demo", Version: "0.2.0", Description: "Claude demo plugin"},
		ResolvedSource: &ports.ResolvedSource{Kind: "local_path", LocalPath: source},
	})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if len(runner.commands) != 3 {
		t.Fatalf("commands = %#v", runner.commands)
	}
	for i := range runner.commands {
		if runner.commands[i].Dir != workspaceA {
			t.Fatalf("command %d dir = %q want %q", i, runner.commands[i].Dir, workspaceA)
		}
	}
}

func TestRepairFallsBackToMarketplaceAddWhenUpdateFails(t *testing.T) {
	t.Parallel()
	runner := &stubRunner{
		run: func(cmd ports.Command) (ports.CommandResult, error) {
			if equalStrings(cmd.Argv, []string{"claude", "plugin", "marketplace", "update", "integrationctl-claude-demo"}) {
				return ports.CommandResult{ExitCode: 1, Stderr: []byte("missing marketplace")}, nil
			}
			return ports.CommandResult{ExitCode: 0}, nil
		},
	}
	home := t.TempDir()
	adapter := Adapter{Runner: runner, FS: fsadapter.OS{}, UserHome: home}
	root := t.TempDir()
	writeClaudeFile(t, filepath.Join(root, "plugin", "plugin.yaml"), "api_version: v1\nname: claude-demo\nversion: 0.2.0\ndescription: Claude demo plugin\ntargets:\n  - claude\n")

	_, err := adapter.Repair(context.Background(), ports.RepairInput{
		Record: domain.InstallationRecord{
			IntegrationID: "claude-demo",
			Policy:        domain.InstallPolicy{Scope: "user"},
			Targets: map[domain.TargetID]domain.TargetInstallation{
				domain.TargetClaude: {
					TargetID: domain.TargetClaude,
					AdapterMetadata: map[string]any{
						"marketplace_name": "integrationctl-claude-demo",
						"plugin_ref":       "claude-demo@integrationctl-claude-demo",
					},
				},
			},
		},
		Manifest:       &domain.IntegrationManifest{IntegrationID: "claude-demo", Version: "0.2.0", Description: "Claude demo plugin"},
		ResolvedSource: &ports.ResolvedSource{Kind: "local_path", LocalPath: root},
	})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	want := [][]string{
		{"claude", "plugin", "marketplace", "update", "integrationctl-claude-demo"},
		{"claude", "plugin", "marketplace", "add", filepath.Join(home, ".plugin-kit-ai", "materialized", "claude", "claude-demo")},
		{"claude", "plugin", "uninstall", "claude-demo@integrationctl-claude-demo", "--scope", "user"},
		{"claude", "plugin", "install", "claude-demo@integrationctl-claude-demo", "--scope", "user"},
	}
	if len(runner.commands) != len(want) {
		t.Fatalf("commands = %#v", runner.commands)
	}
	for i := range want {
		if !equalStrings(runner.commands[i].Argv, want[i]) {
			t.Fatalf("command %d = %#v want %#v", i, runner.commands[i].Argv, want[i])
		}
	}
}

func writeClaudeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func containsRestriction(items []domain.EnvironmentRestrictionCode, want domain.EnvironmentRestrictionCode) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
