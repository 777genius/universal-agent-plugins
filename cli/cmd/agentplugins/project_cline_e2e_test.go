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
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type projectClineDetector struct{ root string }

func (d projectClineDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return []domain.DetectedClient{{ClientID: domain.ClientCline, ConfigRoot: d.root, Status: domain.DetectionDetected}}, nil
}

type projectClineNoNative struct{ calls int }

func (r *projectClineNoNative) Run(_ context.Context, c legacyports.Command) (legacyports.CommandResult, error) {
	r.calls++
	return legacyports.CommandResult{}, fmt.Errorf("unexpected Cline native command %v", c.Argv)
}

func configureProjectCline(t *testing.T, app agentpluginscli.App, user string, runner *projectClineNoNative) agentpluginscli.App {
	t.Helper()
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	engine, err := installer.New(installer.Config{StateRoot: filepath.Dir(app.ManagedRoot), ManagedRoot: app.ManagedRoot, ProjectRoot: app.ProjectRoot, Registry: app.ClientRegistry, Runner: runner, EnableNativeObserver: true, HelperExecutable: helper})
	if err != nil {
		t.Fatal(err)
	}
	app.Installer = engine
	app.Detector = projectClineDetector{user}
	app.Lifecycle.Activator = providers.Activator{Registry: app.ClientRegistry, Runner: runner}
	app.Lifecycle.NativeObserver = providers.NativeIdentityObserver{Registry: app.ClientRegistry, Runner: runner}
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return configureProjectCline(t, selected, user, runner), nil
	}
	return app
}

// Real saved CLI/TLS Git/immutable pin/state and native file transactions.
// The explicit process boundary refuses every vendor or MCP command.
func TestProjectClineSavedLifecycleSeparatesUserAndProjects(t *testing.T) {
	t.Setenv("CLINE_MCP_SETTINGS_PATH", "")
	t.Setenv("CLINE_DATA_DIR", "")
	transport := newProjectJourneyTransport(t, "git")
	journey := newProjectJourney(t, transport)
	user := filepath.Join(journey.home, ".cline")
	projectJourneyWrite(t, filepath.Join(user, "keep"), "original user")
	runner := &projectClineNoNative{}
	journey.app = configureProjectCline(t, journey.app, user, runner)
	journey.run(t, "", "add", transport.endpoint, "--alias", "docs", "--target", "cline")
	userBefore := securityFixtureTree(t, user)
	userStateBefore := projectJourneyRead(t, filepath.Join(journey.cache, "state-v2.json"))
	for _, project := range []string{journey.a, journey.b} {
		journey.run(t, project, "add", transport.endpoint, "--alias", "docs", "--target", "cline")
		projectClineOwned(t, journey, project, projectJourneySkill)
	}
	bBefore := securityFixtureTree(t, journey.b)
	installation, binding := journey.owned(t, journey.a)
	beforeState := projectJourneyRead(t, filepath.Join(journey.a, ".uap", "state-v2.json"))
	nativeBefore := securityFixtureTree(t, filepath.Join(journey.a, ".cline"))
	configBefore := projectJourneyRead(t, filepath.Join(journey.a, ".uap", "clients", "cline", "mcp.json"))
	if _, err := journey.execute(journey.a, "add", transport.endpoint, "--alias", "docs", "--target", "cline", "--activation-complete"); err == nil {
		t.Fatal("saved replay accepted activation attestation")
	}
	if beforeState != projectJourneyRead(t, filepath.Join(journey.a, ".uap", "state-v2.json")) || !reflect.DeepEqual(nativeBefore, securityFixtureTree(t, filepath.Join(journey.a, ".cline"))) || configBefore != projectJourneyRead(t, filepath.Join(journey.a, ".uap", "clients", "cline", "mcp.json")) {
		t.Fatal("denied saved attestation changed state/native files")
	}
	journey.run(t, journey.a, "deps", "sync", "--locked", "--yes")
	projectJourneyWrite(t, filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md"), "managed package drift")
	projectJourneyWrite(t, filepath.Join(transport.repo, "skills", "docs", "SKILL.md"), projectJourneyAdvancedSkill)
	projectJourneyGit(t, transport.repo, "add", ".")
	projectJourneyGit(t, transport.repo, "commit", "-m", "advance Cline fixture")
	projectJourneyGit(t, transport.repo, "push", "--quiet", transport.remote, "main")
	journey.run(t, journey.a, "repair", installation.InstallationID, "--target", "cline")
	projectClineOwned(t, journey, journey.a, projectJourneySkill)
	if err := os.RemoveAll(filepath.Join(journey.a, ".cline", "skills", "docs")); err != nil {
		t.Fatal(err)
	}
	journey.run(t, journey.a, "repair", installation.InstallationID, "--target", "cline")
	projectClineOwned(t, journey, journey.a, projectJourneySkill)
	journey.run(t, journey.a, "update", "docs", "--target", "cline")
	updated, current := projectClineOwned(t, journey, journey.a, projectJourneyAdvancedSkill)
	if updated.Source.TreeDigest == installation.Source.TreeDigest || current.ClientBindingID != binding.ClientBindingID || current.TargetLocator != binding.TargetLocator {
		t.Fatal("saved update lost exact authority")
	}
	beforeCalls := runner.calls
	status := journey.run(t, journey.a, "deps", "status")
	if runner.calls != beforeCalls || !strings.Contains(status, "activation_prepared") || strings.Contains(status, "activation_active") {
		t.Fatal("saved status called native or promoted activation", status)
	}
	journey.run(t, journey.a, "remove", "docs", "--target", "cline")
	projectJourneyAbsent(t, current.TargetLocator, filepath.Join(journey.a, ".cline", "skills", "docs"))
	if !reflect.DeepEqual(userBefore, securityFixtureTree(t, user)) || !reflect.DeepEqual(bBefore, securityFixtureTree(t, journey.b)) || userStateBefore != projectJourneyRead(t, filepath.Join(journey.cache, "state-v2.json")) {
		t.Fatal("A lifecycle changed original user native/state authority or B")
	}
	if runner.calls != 0 {
		t.Fatal("file preparation invoked native Cline or MCP")
	}
}

func projectClineOwned(t *testing.T, j projectJourney, project, expected string) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	installation, b := j.owned(t, project)
	if b.ClientID != "cline" || b.Scope != "project" || b.NativeProfileRoot != project || b.Activation != domain.ActivationPrepared || b.Verification != domain.VerificationInstalled {
		t.Fatalf("Cline lost scoped Prepared authority: %+v", b)
	}
	if projectJourneyRead(t, filepath.Join(project, ".cline", "skills", "docs", "SKILL.md")) != expected {
		t.Fatal("actual project skill differs")
	}
	body := projectJourneyRead(t, filepath.Join(project, ".uap", "clients", "cline", "mcp.json"))
	if !strings.Contains(body, `"docs"`) {
		t.Fatal("prepared MCP member missing")
	}
	return installation, b
}
