package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type projectWorkspaceDetector struct{ user string }

func (d projectWorkspaceDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return []domain.DetectedClient{{ClientID: domain.ClientVSCode, ConfigRoot: d.user, Version: "1.143.1", Status: domain.DetectionDetected}}, nil
}

type projectWorkspaceNoNative struct{ calls int }

func (r *projectWorkspaceNoNative) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	r.calls++
	return legacyports.CommandResult{}, fmt.Errorf("unexpected VS Code/Copilot/MCP command: %v", command.Argv)
}

func configureWorkspaceJourney(t *testing.T, app agentpluginscli.App, user string, runner *projectWorkspaceNoNative) agentpluginscli.App {
	t.Helper()
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	engine, err := installer.New(installer.Config{ProjectRoot: app.ProjectRoot, StateRoot: filepath.Dir(app.ManagedRoot), ManagedRoot: app.ManagedRoot,
		Registry: app.ClientRegistry, Runner: runner, HelperExecutable: helper, EnableNativeObserver: true})
	if err != nil {
		t.Fatal(err)
	}
	app.Installer, app.Detector = engine, projectWorkspaceDetector{user: user}
	kernel := nativeconfig.New()
	app.Lifecycle.Activator = providers.Activator{Registry: app.ClientRegistry, Runner: runner, NativeConfig: &kernel}
	app.Lifecycle.NativeObserver = providers.NativeIdentityObserver{Registry: app.ClientRegistry, Runner: runner, NativeConfig: &kernel}
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return configureWorkspaceJourney(t, selected, user, runner), nil
	}
	return app
}

// Public Cobra, TLS Git acquisition/pins, SDK and native file transactions run.
// The vendor boundary refuses every process; workbench metadata has its separate
// genuine pinned observation and is never fabricated by this fixture.
func TestProjectVSCodeSavedLifecyclePreservesUserBAndCopilotFiles(t *testing.T) {
	transport := newProjectJourneyTransport(t, "git")
	journey := newProjectJourney(t, transport)
	journey.run(t, "", "add", transport.endpoint, "--alias", "original-user", "--target", "gemini")
	userState := projectJourneyRead(t, filepath.Join(journey.cache, "state-v2.json"))
	geminiUser := securityFixtureTree(t, filepath.Join(journey.home, ".gemini"))
	user := filepath.Join(journey.home, "Code")
	projectJourneyWrite(t, filepath.Join(user, "User", "mcp.json"), `{"servers":{"original-user":{"type":"http","url":"https://foreign.test/user"}}}`)
	userBefore := securityFixtureTree(t, user)
	runner := &projectWorkspaceNoNative{}
	journey.app = configureWorkspaceJourney(t, journey.app, user, runner)
	for _, project := range []string{journey.a, journey.b} {
		projectJourneyWrite(t, filepath.Join(project, ".vscode", "mcp.json"), "{\n// retained workspace comment\n\"inputs\":[],\"servers\":{\"foreign\":{\"type\":\"stdio\",\"command\":\"inert\",\"env\":{\"PORT\":8123},},},\n}\n")
		projectJourneyWrite(t, filepath.Join(project, ".github", "mcp.json"), "Copilot control bytes")
		projectJourneyWrite(t, filepath.Join(project, ".github", "skills", "foreign", "SKILL.md"), "foreign skill")
		journey.run(t, project, "add", transport.endpoint, "--alias", "review", "--target", "vscode")
		workspaceJourneyOwned(t, journey, project, projectJourneySkill)
	}
	bBefore := securityFixtureTree(t, journey.b)
	installation, binding := workspaceJourneyOwned(t, journey, journey.a, projectJourneySkill)
	beforeState := projectJourneyRead(t, filepath.Join(journey.a, ".uap", "state-v2.json"))
	config := projectJourneyRead(t, filepath.Join(journey.a, ".vscode", "mcp.json"))
	for _, args := range [][]string{
		{"add", transport.endpoint, "--alias", "review", "--target", "vscode"},
		{"deps", "sync", "--locked", "--yes"},
	} {
		journey.run(t, journey.a, args...)
		if projectJourneyRead(t, filepath.Join(journey.a, ".uap", "state-v2.json")) != beforeState || projectJourneyRead(t, filepath.Join(journey.a, ".vscode", "mcp.json")) != config {
			t.Fatal("unchanged saved workspace replay changed persisted bytes")
		}
	}
	if _, err := journey.execute(journey.a, "add", transport.endpoint, "--alias", "review", "--target", "vscode", "--activation-complete"); err == nil {
		t.Fatal("workspace accepted operator attestation as discovery")
	}
	projectJourneyWrite(t, filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md"), "managed package drift")
	projectJourneyWrite(t, filepath.Join(transport.repo, "skills", "docs", "SKILL.md"), projectJourneyAdvancedSkill)
	projectJourneyGit(t, transport.repo, "add", ".")
	projectJourneyGit(t, transport.repo, "commit", "-m", "advance workspace source")
	projectJourneyGit(t, transport.repo, "push", "--quiet", transport.remote, "main")
	journey.run(t, journey.a, "repair", installation.InstallationID, "--target", "vscode")
	workspaceJourneyOwned(t, journey, journey.a, projectJourneySkill)
	if err := os.RemoveAll(filepath.Join(journey.a, ".github", "skills", "docs")); err != nil {
		t.Fatal(err)
	}
	journey.run(t, journey.a, "repair", installation.InstallationID, "--target", "vscode")
	workspaceJourneyOwned(t, journey, journey.a, projectJourneySkill)
	journey.run(t, journey.a, "update", "review", "--target", "vscode")
	updated, current := workspaceJourneyOwned(t, journey, journey.a, projectJourneyAdvancedSkill)
	if updated.Source.TreeDigest == installation.Source.TreeDigest || current.ClientBindingID != binding.ClientBindingID || current.TargetLocator != binding.TargetLocator {
		t.Fatal("workspace update changed authority or ignored moving source")
	}
	status := journey.run(t, journey.a, "deps", "status")
	if !strings.Contains(status, "activation_prepared") || strings.Contains(status, "activation_active") {
		t.Fatal("workspace status promoted file preparation", status)
	}
	journey.run(t, journey.a, "remove", "review", "--target", "vscode")
	projectJourneyAbsent(t, current.TargetLocator, filepath.Join(journey.a, ".github", "skills", "docs"))
	if projectJourneyRead(t, filepath.Join(journey.a, ".github", "mcp.json")) != "Copilot control bytes" || !strings.Contains(projectJourneyRead(t, filepath.Join(journey.a, ".vscode", "mcp.json")), "retained workspace comment") {
		t.Fatal("workspace removal changed Copilot or foreign JSONC authority")
	}
	if !reflect.DeepEqual(userBefore, securityFixtureTree(t, user)) || !reflect.DeepEqual(geminiUser, securityFixtureTree(t, filepath.Join(journey.home, ".gemini"))) || !reflect.DeepEqual(bBefore, securityFixtureTree(t, journey.b)) || userState != projectJourneyRead(t, filepath.Join(journey.cache, "state-v2.json")) || runner.calls != 0 {
		t.Fatal("workspace A lifecycle changed original user/B or invoked a vendor")
	}
}

func workspaceJourneyOwned(t *testing.T, journey projectJourney, project, expected string) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	installation, binding := journey.owned(t, project)
	if binding.ClientID != "vscode" || binding.Scope != "project" || binding.NativeProfileRoot != filepath.Join(project, ".vscode") || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled {
		t.Fatalf("workspace binding lost file-only Prepared authority: %+v", binding)
	}
	if projectJourneyRead(t, filepath.Join(project, ".github", "skills", "docs", "SKILL.md")) != expected {
		t.Fatal("workspace skill differs from pinned package")
	}
	body := projectJourneyRead(t, filepath.Join(project, ".vscode", "mcp.json"))
	if !strings.Contains(body, `"servers"`) || !strings.Contains(body, `"docs"`) || !strings.Contains(body, `"PORT":8123`) {
		t.Fatal("workspace codec lost owned registration or foreign numeric environment")
	}
	return installation, binding
}
