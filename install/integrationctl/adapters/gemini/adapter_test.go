package gemini

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

func (s *stubRunner) Run(ctx context.Context, cmd ports.Command) (ports.CommandResult, error) {
	s.commands = append(s.commands, cmd)
	if s.run != nil {
		return s.run(cmd)
	}
	return ports.CommandResult{ExitCode: 0}, nil
}

func TestApplyInstallLocalUsesManagedGeminiInstall(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	writeGeminiFile(t, filepath.Join(root, "plugin", "targets", "gemini", "package.yaml"), "context_file_name: \"GEMINI.md\"\nexclude_tools:\n  - \"run_shell_command(rm -rf)\"\nplan_directory: \".gemini/plans\"\n")
	writeGeminiFile(t, filepath.Join(root, "plugin", "targets", "gemini", "manifest.extra.json"), "{\n  \"plan\": {\"retentionDays\": 7},\n  \"x_galleryTopic\": \"gemini-cli-extension\"\n}\n")
	writeGeminiFile(t, filepath.Join(root, "plugin", "targets", "gemini", "contexts", "GEMINI.md"), "# Primary\n")
	writeGeminiFile(t, filepath.Join(root, "plugin", "targets", "gemini", "contexts", "RELEASE.md"), "# Extra\n")
	writeGeminiFile(t, filepath.Join(root, "plugin", "targets", "gemini", "settings", "release-profile.yaml"), "name: \"release-profile\"\ndescription: \"Release profile\"\nenv_var: \"RELEASE_PROFILE\"\nsensitive: false\n")
	writeGeminiFile(t, filepath.Join(root, "plugin", "targets", "gemini", "themes", "release-dawn.yaml"), "name: \"release-dawn\"\nbackground:\n  primary: \"#fff9f2\"\n")
	writeGeminiFile(t, filepath.Join(root, "plugin", "targets", "gemini", "commands", "deploy.toml"), "description = \"Deploy\"\n")
	writeGeminiFile(t, filepath.Join(root, "plugin", "targets", "gemini", "policies", "release-review.toml"), "[[rule]]\ntoolName = \"dangerous\"\ndecision = \"ask_user\"\n")
	writeGeminiFile(t, filepath.Join(root, "plugin", "targets", "gemini", "hooks", "hooks.json"), "{\n  \"hooks\": {}\n}\n")
	writeGeminiFile(t, filepath.Join(root, "plugin", "targets", "gemini", "agents", "reviewer.md"), "# reviewer\n")
	writeGeminiFile(t, filepath.Join(root, "plugin", "skills", "security-audit", "SKILL.md"), "# Skill\n")
	writeGeminiFile(t, filepath.Join(root, "plugin", "mcp", "servers.yaml"), "api_version: v1\nservers:\n  docs:\n    type: remote\n    remote:\n      protocol: streamable_http\n      url: \"https://example.com/mcp\"\n    targets:\n      - gemini\n  release-checks:\n    type: stdio\n    stdio:\n      command: ${package.root}/bin/release-checks.mjs\n      args:\n        - --config=${package.root}/config.json\n    targets:\n      - gemini\n")
	runner := &stubRunner{}
	adapter := Adapter{Runner: runner, FS: fsadapter.OS{}, UserHome: home}

	result, err := adapter.ApplyInstall(context.Background(), ports.ApplyInput{
		Manifest: domain.IntegrationManifest{
			IntegrationID: "gemini-demo",
			Version:       "0.1.0",
			Description:   "Gemini demo extension",
			RequestedRef:  domain.RequestedSourceRef{Kind: "local_path", Value: root},
			ResolvedRef:   domain.ResolvedSourceRef{Kind: "local_path", Value: root},
		},
		ResolvedSource: &ports.ResolvedSource{
			Kind:      "local_path",
			LocalPath: root,
		},
		Policy: domain.InstallPolicy{Scope: "user", AutoUpdate: true},
	})
	if err != nil {
		t.Fatalf("apply install: %v", err)
	}
	managedRoot := filepath.Join(home, ".plugin-kit-ai", "materialized", "gemini", "gemini-demo")
	if len(runner.commands) != 0 {
		t.Fatalf("runner commands = %#v, want none for local projection install", runner.commands)
	}
	manifestBody, err := os.ReadFile(filepath.Join(managedRoot, "gemini-extension.json"))
	if err != nil {
		t.Fatalf("read materialized manifest: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(manifestBody, &doc); err != nil {
		t.Fatalf("parse materialized manifest: %v", err)
	}
	if doc["name"] != "gemini-demo" || doc["version"] != "0.1.0" || doc["description"] != "Gemini demo extension" {
		t.Fatalf("materialized manifest identity = %#v", doc)
	}
	if doc["contextFileName"] != "GEMINI.md" {
		t.Fatalf("contextFileName = %#v", doc["contextFileName"])
	}
	if _, err := os.Stat(filepath.Join(managedRoot, "GEMINI.md")); err != nil {
		t.Fatalf("stat primary context: %v", err)
	}
	if _, err := os.Stat(filepath.Join(managedRoot, "contexts", "RELEASE.md")); err != nil {
		t.Fatalf("stat extra context: %v", err)
	}
	if _, err := os.Stat(filepath.Join(managedRoot, "commands", "deploy.toml")); err != nil {
		t.Fatalf("stat commands: %v", err)
	}
	if _, err := os.Stat(filepath.Join(managedRoot, "policies", "release-review.toml")); err != nil {
		t.Fatalf("stat policies: %v", err)
	}
	if _, err := os.Stat(filepath.Join(managedRoot, "hooks", "hooks.json")); err != nil {
		t.Fatalf("stat hooks: %v", err)
	}
	if _, err := os.Stat(filepath.Join(managedRoot, "skills", "security-audit", "SKILL.md")); err != nil {
		t.Fatalf("stat skills: %v", err)
	}
	if _, err := os.Stat(filepath.Join(managedRoot, "agents", "reviewer.md")); err != nil {
		t.Fatalf("stat agents: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".gemini", "extensions", "gemini-demo", "gemini-extension.json")); err != nil {
		t.Fatalf("stat projected extension config: %v", err)
	}
	installMetaBody, err := os.ReadFile(filepath.Join(home, ".gemini", "extensions", "gemini-demo", ".gemini-extension-install.json"))
	if err != nil {
		t.Fatalf("read projected install metadata: %v", err)
	}
	var installMeta map[string]any
	if err := json.Unmarshal(installMetaBody, &installMeta); err != nil {
		t.Fatalf("parse projected install metadata: %v", err)
	}
	if installMeta["type"] != "link" || installMeta["source"] != managedRoot {
		t.Fatalf("install metadata = %#v", installMeta)
	}
	servers, _ := doc["mcpServers"].(map[string]any)
	if docs, _ := servers["docs"].(map[string]any); docs["httpUrl"] != "https://example.com/mcp" {
		t.Fatalf("docs projection = %#v", servers["docs"])
	}
	if checks, _ := servers["release-checks"].(map[string]any); checks["command"] != filepath.Join(root, "bin", "release-checks.mjs") {
		t.Fatalf("checks projection = %#v", servers["release-checks"])
	} else {
		args, _ := checks["args"].([]any)
		if len(args) != 1 || args[0] != "--config="+filepath.Join(root, "config.json") {
			t.Fatalf("checks args = %#v", checks["args"])
		}
	}
	if result.State != domain.InstallInstalled || result.ActivationState != domain.ActivationRestartPending {
		t.Fatalf("result = %+v", result)
	}
	if got := result.AdapterMetadata["materialized_source_root"]; got != managedRoot {
		t.Fatalf("materialized_source_root = %#v, want %q", got, managedRoot)
	}
	if got := result.AdapterMetadata["install_mode"]; got != "local_projection" {
		t.Fatalf("install_mode = %#v, want %q", got, "local_projection")
	}
}

func TestApplyInstallRemoteUsesGeminiInstallFlags(t *testing.T) {
	t.Parallel()
	runner := &stubRunner{}
	adapter := Adapter{Runner: runner, UserHome: t.TempDir()}

	_, err := adapter.ApplyInstall(context.Background(), ports.ApplyInput{
		Manifest: domain.IntegrationManifest{
			IntegrationID: "gemini-demo",
			RequestedRef:  domain.RequestedSourceRef{Kind: "git_url", Value: "https://github.com/acme/demo.git"},
			ResolvedRef:   domain.ResolvedSourceRef{Kind: "git_commit", Value: "https://github.com/acme/demo.git@abc123"},
		},
		ResolvedSource: &ports.ResolvedSource{
			Kind:      "git_url",
			LocalPath: t.TempDir(),
		},
		Policy: domain.InstallPolicy{Scope: "user", AutoUpdate: true, AllowPrerelease: true},
	})
	if err != nil {
		t.Fatalf("apply install: %v", err)
	}
	want := []string{"gemini", "extensions", "install", "https://github.com/acme/demo.git", "--auto-update", "--pre-release"}
	if got := runner.commands[0].Argv; !equalStrings(got, want) {
		t.Fatalf("argv = %#v, want %#v", got, want)
	}
}

func TestApplyInstallGitHubRepoPathUsesManagedProjection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	sourceRoot := filepath.Join(root, "source")
	writeGeminiFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: gemini-demo\nversion: 0.1.0\ndescription: Gemini demo extension\ntargets:\n  - gemini\n")
	writeGeminiFile(t, filepath.Join(sourceRoot, "plugin", "targets", "gemini", "package.yaml"), "context_file_name: \"GEMINI.md\"\n")
	writeGeminiFile(t, filepath.Join(sourceRoot, "plugin", "targets", "gemini", "contexts", "GEMINI.md"), "# Primary\n")
	runner := &stubRunner{}
	adapter := Adapter{Runner: runner, FS: fsadapter.OS{}, UserHome: home}

	result, err := adapter.ApplyInstall(context.Background(), ports.ApplyInput{
		Manifest: domain.IntegrationManifest{
			IntegrationID: "gemini-demo",
			Version:       "0.1.0",
			Description:   "Gemini demo extension",
			RequestedRef:  domain.RequestedSourceRef{Kind: "github_repo_path", Value: "github:acme/demo//plugins/gemini-demo"},
			ResolvedRef:   domain.ResolvedSourceRef{Kind: "git_commit", Value: "https://github.com/acme/demo@abc123"},
		},
		ResolvedSource: &ports.ResolvedSource{
			Kind:      "github_repo_path",
			LocalPath: sourceRoot,
		},
		Policy: domain.InstallPolicy{Scope: "user", AutoUpdate: true},
	})
	if err != nil {
		t.Fatalf("apply install: %v", err)
	}
	if len(runner.commands) != 0 {
		t.Fatalf("runner commands = %#v, want none for github repo-path projection install", runner.commands)
	}
	if got := result.AdapterMetadata["install_mode"]; got != "local_projection" {
		t.Fatalf("install_mode = %#v, want %q", got, "local_projection")
	}
	if _, err := os.Stat(filepath.Join(home, ".gemini", "extensions", "gemini-demo", "gemini-extension.json")); err != nil {
		t.Fatalf("stat projected extension config: %v", err)
	}
}

func TestApplyUpdateUsesGeminiUpdate(t *testing.T) {
	t.Parallel()
	runner := &stubRunner{}
	root := t.TempDir()
	home := filepath.Join(root, "home")
	sourceRoot := filepath.Join(root, "source")
	writeGeminiFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: gemini-demo\nversion: 0.2.0\ndescription: Gemini demo extension\ntargets:\n  - gemini\n")
	writeGeminiFile(t, filepath.Join(sourceRoot, "plugin", "targets", "gemini", "package.yaml"), "context_file_name: \"GEMINI.md\"\n")
	writeGeminiFile(t, filepath.Join(sourceRoot, "plugin", "targets", "gemini", "contexts", "GEMINI.md"), "# Updated\n")
	adapter := Adapter{Runner: runner, FS: fsadapter.OS{}, UserHome: home}
	writeGeminiFile(t, filepath.Join(home, ".gemini", "extensions", "gemini-demo", ".env"), "RELEASE_PROFILE=prod\n")

	result, err := adapter.ApplyUpdate(context.Background(), ports.ApplyInput{
		Manifest: domain.IntegrationManifest{
			IntegrationID: "gemini-demo",
			Version:       "0.2.0",
			Description:   "Gemini demo extension",
			ResolvedRef:   domain.ResolvedSourceRef{Kind: "git_commit", Value: "https://github.com/acme/demo.git@def456"},
		},
		ResolvedSource: &ports.ResolvedSource{Kind: "local_path", LocalPath: sourceRoot},
		Record: &domain.InstallationRecord{
			IntegrationID:      "gemini-demo",
			RequestedSourceRef: domain.RequestedSourceRef{Kind: "local_path", Value: sourceRoot},
			Targets: map[domain.TargetID]domain.TargetInstallation{
				domain.TargetGemini: {
					TargetID: domain.TargetGemini,
					AdapterMetadata: map[string]any{
						"materialized_source_root": filepath.Join(home, ".plugin-kit-ai", "materialized", "gemini", "gemini-demo"),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if len(runner.commands) != 0 {
		t.Fatalf("runner commands = %#v, want none for local projection update", runner.commands)
	}
	body, err := os.ReadFile(filepath.Join(home, ".plugin-kit-ai", "materialized", "gemini", "gemini-demo", "GEMINI.md"))
	if err != nil || !strings.Contains(string(body), "Updated") {
		t.Fatalf("materialized root not refreshed: %v %q", err, body)
	}
	projected, err := os.ReadFile(filepath.Join(home, ".gemini", "extensions", "gemini-demo", "GEMINI.md"))
	if err != nil || !strings.Contains(string(projected), "Updated") {
		t.Fatalf("projected extension dir not refreshed: %v %q", err, projected)
	}
	installMetaBody, err := os.ReadFile(filepath.Join(home, ".gemini", "extensions", "gemini-demo", ".gemini-extension-install.json"))
	if err != nil {
		t.Fatalf("read projected install metadata: %v", err)
	}
	var installMeta map[string]any
	if err := json.Unmarshal(installMetaBody, &installMeta); err != nil {
		t.Fatalf("parse projected install metadata: %v", err)
	}
	if installMeta["type"] != "link" || installMeta["source"] != filepath.Join(home, ".plugin-kit-ai", "materialized", "gemini", "gemini-demo") {
		t.Fatalf("install metadata = %#v", installMeta)
	}
	dotenv, err := os.ReadFile(filepath.Join(home, ".gemini", "extensions", "gemini-demo", ".env"))
	if err != nil || !strings.Contains(string(dotenv), "RELEASE_PROFILE=prod") {
		t.Fatalf("projected .env not preserved: %v %q", err, dotenv)
	}
	if result.State != domain.InstallInstalled {
		t.Fatalf("result = %+v", result)
	}
}

func TestApplyUpdateGitHubRepoPathUsesManagedProjection(t *testing.T) {
	t.Parallel()
	runner := &stubRunner{}
	root := t.TempDir()
	home := filepath.Join(root, "home")
	sourceRoot := filepath.Join(root, "source")
	writeGeminiFile(t, filepath.Join(sourceRoot, "plugin", "plugin.yaml"), "api_version: v1\nname: gemini-demo\nversion: 0.2.0\ndescription: Gemini demo extension\ntargets:\n  - gemini\n")
	writeGeminiFile(t, filepath.Join(sourceRoot, "plugin", "targets", "gemini", "package.yaml"), "context_file_name: \"GEMINI.md\"\n")
	writeGeminiFile(t, filepath.Join(sourceRoot, "plugin", "targets", "gemini", "contexts", "GEMINI.md"), "# Updated from repo path\n")
	adapter := Adapter{Runner: runner, FS: fsadapter.OS{}, UserHome: home}
	writeGeminiFile(t, filepath.Join(home, ".gemini", "extensions", "gemini-demo", ".env"), "RELEASE_PROFILE=prod\n")

	result, err := adapter.ApplyUpdate(context.Background(), ports.ApplyInput{
		Manifest: domain.IntegrationManifest{
			IntegrationID: "gemini-demo",
			Version:       "0.2.0",
			Description:   "Gemini demo extension",
			RequestedRef:  domain.RequestedSourceRef{Kind: "github_repo_path", Value: "github:acme/demo//plugins/gemini-demo"},
			ResolvedRef:   domain.ResolvedSourceRef{Kind: "git_commit", Value: "https://github.com/acme/demo@def456"},
		},
		ResolvedSource: &ports.ResolvedSource{Kind: "github_repo_path", LocalPath: sourceRoot},
		Record: &domain.InstallationRecord{
			IntegrationID:      "gemini-demo",
			RequestedSourceRef: domain.RequestedSourceRef{Kind: "github_repo_path", Value: "github:acme/demo//plugins/gemini-demo"},
			Targets: map[domain.TargetID]domain.TargetInstallation{
				domain.TargetGemini: {
					TargetID: domain.TargetGemini,
					AdapterMetadata: map[string]any{
						"install_mode":             "local_projection",
						"materialized_source_root": filepath.Join(home, ".plugin-kit-ai", "materialized", "gemini", "gemini-demo"),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if len(runner.commands) != 0 {
		t.Fatalf("runner commands = %#v, want none for github repo-path projection update", runner.commands)
	}
	body, err := os.ReadFile(filepath.Join(home, ".gemini", "extensions", "gemini-demo", "GEMINI.md"))
	if err != nil || !strings.Contains(string(body), "Updated from repo path") {
		t.Fatalf("projected extension dir not refreshed: %v %q", err, body)
	}
	if got := result.AdapterMetadata["update_mode"]; got != "local_projection" {
		t.Fatalf("update_mode = %#v, want %q", got, "local_projection")
	}
}

func TestApplyRemoveUsesGeminiLocalProjectionCleanup(t *testing.T) {
	t.Parallel()
	runner := &stubRunner{}
	root := t.TempDir()
	managedRoot := filepath.Join(root, ".plugin-kit-ai", "materialized", "gemini", "gemini-demo")
	writeGeminiFile(t, filepath.Join(managedRoot, "gemini-extension.json"), "{}\n")
	writeGeminiFile(t, filepath.Join(root, ".gemini", "extensions", "gemini-demo", "gemini-extension.json"), "{}\n")
	adapter := Adapter{Runner: runner, UserHome: root}

	result, err := adapter.ApplyRemove(context.Background(), ports.ApplyInput{
		Record: &domain.InstallationRecord{
			IntegrationID: "gemini-demo",
			Targets: map[domain.TargetID]domain.TargetInstallation{
				domain.TargetGemini: {
					TargetID: domain.TargetGemini,
					AdapterMetadata: map[string]any{
						"install_mode":             "local_projection",
						"materialized_source_root": managedRoot,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("apply remove: %v", err)
	}
	if len(runner.commands) != 0 {
		t.Fatalf("runner commands = %#v, want none for local projection remove", runner.commands)
	}
	if result.State != domain.InstallRemoved {
		t.Fatalf("result = %+v", result)
	}
	if _, err := os.Stat(managedRoot); !os.IsNotExist(err) {
		t.Fatalf("managed root still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".gemini", "extensions", "gemini-demo")); !os.IsNotExist(err) {
		t.Fatalf("extension dir still exists: %v", err)
	}
}

func TestInspectReturnsDisabledWhenSettingsDisableExtension(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	writeGeminiFile(t, filepath.Join(home, ".gemini", "extensions", "gemini-demo", "gemini-extension.json"), "{}\n")
	writeGeminiFile(t, filepath.Join(home, ".gemini", "settings.json"), "{\n  \"extensions\": {\n    \"disabled\": [\"gemini-demo\"]\n  }\n}\n")
	adapter := Adapter{FS: fsadapter.OS{}, UserHome: home}

	inspect, err := adapter.Inspect(context.Background(), ports.InspectInput{
		Record: &domain.InstallationRecord{
			IntegrationID: "gemini-demo",
			Policy:        domain.InstallPolicy{Scope: "user"},
		},
		Scope: "user",
	})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if inspect.State != domain.InstallDisabled {
		t.Fatalf("state = %s, want disabled", inspect.State)
	}
}

func TestInspectProjectScopeUsesPersistedWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	workspaceA := filepath.Join(root, "workspace-a")
	workspaceB := filepath.Join(root, "workspace-b")
	writeGeminiFile(t, filepath.Join(home, ".gemini", "extensions", "gemini-demo", "gemini-extension.json"), "{}\n")
	writeGeminiFile(t, filepath.Join(home, ".gemini", "extensions", "extension-enablement.json"), "{\n  \"gemini-demo\": {\n    \"overrides\": [\n      \"!"+strings.ReplaceAll(filepath.Clean(workspaceA), "\\", "\\\\")+"/*\"\n    ]\n  }\n}\n")
	writeGeminiFile(t, filepath.Join(workspaceB, ".gemini", "settings.json"), "{\n  \"extensions\": {\n    \"disabled\": []\n  }\n}\n")
	adapter := Adapter{FS: fsadapter.OS{}, UserHome: home}

	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(prevWD)
	}()
	if err := os.Chdir(workspaceB); err != nil {
		t.Fatalf("chdir workspace-b: %v", err)
	}

	inspect, err := adapter.Inspect(context.Background(), ports.InspectInput{
		Record: &domain.InstallationRecord{
			IntegrationID: "gemini-demo",
			Policy:        domain.InstallPolicy{Scope: "project"},
			WorkspaceRoot: workspaceA,
		},
		Scope: "project",
	})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if inspect.State != domain.InstallDisabled {
		t.Fatalf("state = %s, want disabled from persisted workspace root", inspect.State)
	}
}

func TestInspectProjectScopeEnablementOverridesWorkspaceSettingsFallback(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	workspace := filepath.Join(root, "workspace-a")
	writeGeminiFile(t, filepath.Join(home, ".gemini", "extensions", "gemini-demo", "gemini-extension.json"), "{}\n")
	writeGeminiFile(t, filepath.Join(home, ".gemini", "extensions", "extension-enablement.json"), "{\n  \"gemini-demo\": {\n    \"overrides\": [\n      \""+strings.ReplaceAll(filepath.Clean(workspace), "\\", "\\\\")+"/*\"\n    ]\n  }\n}\n")
	writeGeminiFile(t, filepath.Join(workspace, ".gemini", "settings.json"), "{\n  \"extensions\": {\n    \"disabled\": [\"gemini-demo\"]\n  }\n}\n")
	adapter := Adapter{FS: fsadapter.OS{}, UserHome: home}

	inspect, err := adapter.Inspect(context.Background(), ports.InspectInput{
		Record: &domain.InstallationRecord{
			IntegrationID: "gemini-demo",
			Policy:        domain.InstallPolicy{Scope: "project"},
			WorkspaceRoot: workspace,
		},
		Scope: "project",
	})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if inspect.State != domain.InstallInstalled {
		t.Fatalf("state = %s, want installed from enablement override", inspect.State)
	}
}

func TestDisabledByScopeOverrideUserScopeWildcardDisable(t *testing.T) {
	t.Parallel()

	disabled, handled, err := disabledByScopeOverride("user", "", []string{"!*"})
	if err != nil {
		t.Fatalf("disabledByScopeOverride: %v", err)
	}
	if !handled || !disabled {
		t.Fatalf("handled=%t disabled=%t, want true/true", handled, disabled)
	}
}

func TestDisabledByScopeOverrideProjectScopeMatchesWorkspaceRoot(t *testing.T) {
	t.Parallel()

	workspace := filepath.Join(t.TempDir(), "workspace")
	disabled, handled, err := disabledByScopeOverride("project", workspace, []string{filepath.Clean(workspace) + string(os.PathSeparator) + "*"})
	if err != nil {
		t.Fatalf("disabledByScopeOverride: %v", err)
	}
	if !handled || disabled {
		t.Fatalf("handled=%t disabled=%t, want true/false", handled, disabled)
	}
}

func TestPlanEnableProjectScopeUsesPersistedWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	workspaceA := filepath.Join(root, "workspace-a")
	workspaceB := filepath.Join(root, "workspace-b")
	if err := os.MkdirAll(workspaceA, 0o755); err != nil {
		t.Fatalf("mkdir workspace-a: %v", err)
	}
	if err := os.MkdirAll(workspaceB, 0o755); err != nil {
		t.Fatalf("mkdir workspace-b: %v", err)
	}
	adapter := Adapter{UserHome: home}

	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(prevWD)
	}()
	if err := os.Chdir(workspaceB); err != nil {
		t.Fatalf("chdir workspace-b: %v", err)
	}

	plan, err := adapter.PlanEnable(context.Background(), ports.PlanToggleInput{
		Record: domain.InstallationRecord{
			IntegrationID: "gemini-demo",
			Policy:        domain.InstallPolicy{Scope: "project"},
			WorkspaceRoot: workspaceA,
		},
	})
	if err != nil {
		t.Fatalf("plan enable: %v", err)
	}
	want := []string{
		filepath.Join(workspaceA, ".gemini", "settings.json"),
		filepath.Join(home, ".gemini", "extensions", "extension-enablement.json"),
	}
	if got := plan.PathsTouched; !equalStrings(got, want) {
		t.Fatalf("paths touched = %#v, want %#v", got, want)
	}
}

func TestApplyDisableUsesNativeGeminiDisable(t *testing.T) {
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
	adapter := Adapter{Runner: runner, UserHome: t.TempDir()}
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(prevWD)
	}()
	if err := os.Chdir(workspaceB); err != nil {
		t.Fatalf("chdir workspace-b: %v", err)
	}

	result, err := adapter.ApplyDisable(context.Background(), ports.ApplyInput{
		Record: &domain.InstallationRecord{
			IntegrationID: "gemini-demo",
			Policy:        domain.InstallPolicy{Scope: "project"},
			WorkspaceRoot: workspaceA,
			Targets: map[domain.TargetID]domain.TargetInstallation{
				domain.TargetGemini: {TargetID: domain.TargetGemini},
			},
		},
	})
	if err != nil {
		t.Fatalf("apply disable: %v", err)
	}
	want := []string{"gemini", "extensions", "disable", "gemini-demo", "--scope", "workspace"}
	if got := runner.commands[0].Argv; !equalStrings(got, want) {
		t.Fatalf("argv = %#v, want %#v", got, want)
	}
	if got := runner.commands[0].Dir; got != workspaceA {
		t.Fatalf("dir = %q, want %q", got, workspaceA)
	}
	if result.State != domain.InstallDisabled {
		t.Fatalf("result = %+v", result)
	}
}

func TestPlanInstallBlocksGitSourceWhenSecurityBlocksGitExtensions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	writeGeminiFile(t, filepath.Join(home, ".gemini", "settings.json"), "{\n  \"security\": {\n    \"blockGitExtensions\": true\n  }\n}\n")
	adapter := Adapter{FS: fsadapter.OS{}, UserHome: home}

	plan, err := adapter.PlanInstall(context.Background(), ports.PlanInstallInput{
		Manifest: domain.IntegrationManifest{
			IntegrationID: "gemini-demo",
			RequestedRef:  domain.RequestedSourceRef{Kind: "git_url", Value: "https://github.com/acme/demo.git"},
		},
		Policy: domain.InstallPolicy{Scope: "user"},
	})
	if err != nil {
		t.Fatalf("plan install: %v", err)
	}
	if !plan.Blocking {
		t.Fatal("expected blocking plan")
	}
	if !strings.Contains(strings.Join(plan.ManualSteps, "\n"), "blockGitExtensions") {
		t.Fatalf("manual steps = %#v", plan.ManualSteps)
	}
}

func TestPlanInstallAllowedExtensionsOverridesBlockGitExtensions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	writeGeminiFile(t, filepath.Join(home, ".gemini", "settings.json"), "{\n  \"security\": {\n    \"blockGitExtensions\": true,\n    \"allowedExtensions\": [\"^https://github\\\\.com/acme/demo\\\\.git$\"]\n  }\n}\n")
	adapter := Adapter{FS: fsadapter.OS{}, UserHome: home}

	plan, err := adapter.PlanInstall(context.Background(), ports.PlanInstallInput{
		Manifest: domain.IntegrationManifest{
			IntegrationID: "gemini-demo",
			RequestedRef:  domain.RequestedSourceRef{Kind: "git_url", Value: "https://github.com/acme/demo.git"},
		},
		Policy: domain.InstallPolicy{Scope: "user"},
	})
	if err != nil {
		t.Fatalf("plan install: %v", err)
	}
	if plan.Blocking {
		t.Fatalf("plan unexpectedly blocking: %#v", plan.ManualSteps)
	}
}

func TestPlanUpdateProjectScopeReadsPersistedWorkspaceSettingsForSecurity(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	workspaceA := filepath.Join(root, "workspace-a")
	workspaceB := filepath.Join(root, "workspace-b")
	writeGeminiFile(t, filepath.Join(workspaceA, ".gemini", "settings.json"), "{\n  \"security\": {\n    \"blockGitExtensions\": true\n  }\n}\n")
	if err := os.MkdirAll(workspaceB, 0o755); err != nil {
		t.Fatalf("mkdir workspace-b: %v", err)
	}
	adapter := Adapter{FS: fsadapter.OS{}, UserHome: home}

	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(prevWD)
	}()
	if err := os.Chdir(workspaceB); err != nil {
		t.Fatalf("chdir workspace-b: %v", err)
	}

	plan, err := adapter.PlanUpdate(context.Background(), ports.PlanUpdateInput{
		CurrentRecord: domain.InstallationRecord{
			IntegrationID: "gemini-demo",
			Policy:        domain.InstallPolicy{Scope: "project"},
			WorkspaceRoot: workspaceA,
		},
		NextManifest: domain.IntegrationManifest{
			IntegrationID: "gemini-demo",
			RequestedRef:  domain.RequestedSourceRef{Kind: "git_url", Value: "https://github.com/acme/demo.git"},
		},
	})
	if err != nil {
		t.Fatalf("plan update: %v", err)
	}
	if !plan.Blocking {
		t.Fatal("expected blocking plan")
	}
	if !strings.Contains(strings.Join(plan.ManualSteps, "\n"), "blockGitExtensions") {
		t.Fatalf("manual steps = %#v", plan.ManualSteps)
	}
}

func TestRepairUsesGeminiUpdateSemantics(t *testing.T) {
	t.Parallel()
	runner := &stubRunner{}
	adapter := Adapter{Runner: runner, UserHome: t.TempDir()}

	result, err := adapter.Repair(context.Background(), ports.RepairInput{
		Record: domain.InstallationRecord{IntegrationID: "gemini-demo", Policy: domain.InstallPolicy{Scope: "user"}},
		Manifest: &domain.IntegrationManifest{
			IntegrationID: "gemini-demo",
			ResolvedRef:   domain.ResolvedSourceRef{Kind: "git_commit", Value: "https://github.com/acme/demo.git@def456"},
		},
	})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	want := []string{"gemini", "extensions", "update", "gemini-demo"}
	if got := runner.commands[0].Argv; !equalStrings(got, want) {
		t.Fatalf("argv = %#v, want %#v", got, want)
	}
	if result.State != domain.InstallInstalled {
		t.Fatalf("result = %+v", result)
	}
}

func TestApplyInstallLocalBuildsFromGeneratedGeminiRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	writeGeminiFile(t, filepath.Join(root, "gemini-extension.json"), "{\n  \"name\": \"gemini-demo\",\n  \"version\": \"0.1.0\",\n  \"description\": \"demo\",\n  \"contextFileName\": \"GEMINI.md\"\n}\n")
	writeGeminiFile(t, filepath.Join(root, "GEMINI.md"), "# Root\n")
	runner := &stubRunner{}
	adapter := Adapter{Runner: runner, FS: fsadapter.OS{}, UserHome: home}

	_, err := adapter.ApplyInstall(context.Background(), ports.ApplyInput{
		Manifest: domain.IntegrationManifest{
			IntegrationID: "gemini-demo",
			Version:       "0.1.0",
			Description:   "demo",
			RequestedRef:  domain.RequestedSourceRef{Kind: "local_path", Value: root},
		},
		ResolvedSource: &ports.ResolvedSource{
			Kind:      "local_path",
			LocalPath: root,
		},
	})
	if err != nil {
		t.Fatalf("apply install: %v", err)
	}
	if len(runner.commands) != 0 {
		t.Fatalf("runner commands = %#v, want none for local projection install", runner.commands)
	}
	if _, err := os.Stat(filepath.Join(home, ".plugin-kit-ai", "materialized", "gemini", "gemini-demo", "GEMINI.md")); err != nil {
		t.Fatalf("stat copied root context: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".gemini", "extensions", "gemini-demo", "gemini-extension.json")); err != nil {
		t.Fatalf("stat projected extension config: %v", err)
	}
}

func TestRunGeminiReturnsMutationErrorOnFailure(t *testing.T) {
	t.Parallel()
	runner := &stubRunner{
		run: func(cmd ports.Command) (ports.CommandResult, error) {
			return ports.CommandResult{ExitCode: 1, Stderr: []byte("boom")}, nil
		},
	}
	adapter := Adapter{Runner: runner, FS: fsadapter.OS{}, UserHome: t.TempDir()}

	_, err := adapter.ApplyInstall(context.Background(), ports.ApplyInput{
		Manifest: domain.IntegrationManifest{
			IntegrationID: "gemini-demo",
			Version:       "0.1.0",
			Description:   "demo",
			RequestedRef:  domain.RequestedSourceRef{Kind: "git_url", Value: "https://github.com/acme/demo.git"},
			ResolvedRef:   domain.ResolvedSourceRef{Kind: "git_commit", Value: "https://github.com/acme/demo.git@abc123"},
		},
		ResolvedSource: &ports.ResolvedSource{Kind: "git_url", LocalPath: t.TempDir()},
	})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.ErrMutationApply {
		t.Fatalf("err = %#v, want mutation_apply", err)
	}
}

func TestSelectPrimaryContextRejectsAmbiguousConfiguredName(t *testing.T) {
	t.Parallel()

	_, ok, err := selectPrimaryContext([]string{
		"release/GEMINI.md",
		"handoff/GEMINI.md",
	}, "GEMINI.md")
	if ok {
		t.Fatal("expected ambiguous configured context to be rejected")
	}
	if err == nil || !strings.Contains(err.Error(), `context_file_name "GEMINI.md" is ambiguous`) {
		t.Fatalf("selectPrimaryContext error = %v", err)
	}
}

func writeGeminiFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
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

func stringsContain(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
