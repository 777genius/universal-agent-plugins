package main

import (
	"context"
	"encoding/json"
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

type projectGrokDetector struct{ user string }

func (d projectGrokDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return []domain.DetectedClient{{ClientID: domain.ClientGrok, ConfigRoot: d.user, ExecutablePath: "/inert/grok-not-executed", Version: "1.0.67", Status: domain.DetectionDetected}}, nil
}

type projectGrokRunner struct {
	t     *testing.T
	calls int
}

func (r *projectGrokRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	r.calls++
	if !reflect.DeepEqual(command.Argv, []string{"/inert/grok-not-executed", "inspect", "--json"}) || command.Dir == "" {
		r.t.Fatal("unexpected global/native command", command)
	}
	plugins := []any{}
	root := filepath.Join(command.Dir, ".grok", "plugins")
	entries, err := os.ReadDir(root)
	if err != nil && !os.IsNotExist(err) {
		return legacyports.CommandResult{}, err
	}
	for _, entry := range entries {
		manifest := filepath.Join(root, entry.Name(), ".grok-plugin", "plugin.json")
		body, err := os.ReadFile(manifest)
		if err != nil {
			return legacyports.CommandResult{}, err
		}
		var object map[string]any
		if err := json.Unmarshal(body, &object); err != nil {
			return legacyports.CommandResult{}, err
		}
		plugins = append(plugins, map[string]any{"name": object["name"], "scope": "project", "path": filepath.Join(root, entry.Name()), "enabled": false})
	}
	body, _ := json.Marshal(map[string]any{"grokVersion": "1.0.67", "cwd": command.Dir, "projectRoot": command.Dir + string(filepath.Separator), "projectTrusted": false, "skills": []any{}, "plugins": plugins})
	return legacyports.CommandResult{Stdout: body}, nil
}
func configureProjectGrokJourney(app agentpluginscli.App, user string, runner *projectGrokRunner) agentpluginscli.App {
	executable, err := os.Executable()
	if err != nil {
		runner.t.Fatal(err)
	}
	engine, err := installer.New(installer.Config{StateRoot: filepath.Dir(app.ManagedRoot), ManagedRoot: app.ManagedRoot, ProjectRoot: app.ProjectRoot, Registry: app.ClientRegistry, Runner: runner, EnableNativeObserver: true, HelperExecutable: executable})
	if err != nil {
		runner.t.Fatal(err)
	}
	app.Installer = engine
	app.Detector = projectGrokDetector{user: user}
	app.Lifecycle.Activator = providers.Activator{Registry: app.ClientRegistry, Runner: runner}
	app.Lifecycle.NativeObserver = providers.NativeIdentityObserver{Registry: app.ClientRegistry, Runner: runner}
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return configureProjectGrokJourney(selected, user, runner), nil
	}
	return app
}

// Real TLS Git acquisition, production saved CLI/planner/staging/store/state
// and exact directory ownership, with inert inspect-only client transport.
func TestProjectGrokComposedSavedLifecycle(t *testing.T) {
	transport := newProjectJourneyTransport(t, "git")
	journey := newProjectJourney(t, transport)
	user := filepath.Join(journey.home, ".grok")
	projectJourneyWrite(t, filepath.Join(user, "keep"), "original user\n")
	runner := &projectGrokRunner{t: t}
	journey.app = configureProjectGrokJourney(journey.app, user, runner)
	userBefore := securityFixtureTree(t, user)
	for _, project := range []string{journey.a, journey.b} {
		journey.run(t, project, "add", transport.endpoint, "--alias", "docs", "--target", "grok")
		projectGrokOwned(t, journey, project, projectJourneySkill)
	}
	bBefore := securityFixtureTree(t, journey.b)
	installation, binding := journey.owned(t, journey.a)
	if len(installation.DataReceipts) != 0 {
		t.Fatal("skills-only projection invented MCP persistent data")
	}
	journey.run(t, journey.a, "deps", "sync", "--locked", "--yes")
	beforeAttest := securityFixtureTree(t, filepath.Join(journey.a, ".grok"))
	if _, err := journey.execute(journey.a, "add", transport.endpoint, "--alias", "docs", "--target", "grok", "--activation-complete"); err == nil {
		t.Fatal("persisted prepared Grok binding accepted activation attestation")
	}
	if !reflect.DeepEqual(beforeAttest, securityFixtureTree(t, filepath.Join(journey.a, ".grok"))) {
		t.Fatal("refused attestation changed native project bytes")
	}
	_, refused := journey.owned(t, journey.a)
	if refused.Activation == domain.ActivationActive || refused.Verification == domain.VerificationInstalled || refused.TargetLocator != binding.TargetLocator {
		t.Fatal("refused attestation promoted loading or changed artifact ownership")
	}
	// A rejected attestation records a failed attempt in existing core state.
	// An explicit ordinary add retries that attempt without native trust or loading.
	journey.run(t, journey.a, "add", transport.endpoint, "--alias", "docs", "--target", "grok")
	projectGrokOwned(t, journey, journey.a, projectJourneySkill)
	projectJourneyWrite(t, filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md"), "TEST immutable drift\n")
	projectJourneyWrite(t, filepath.Join(transport.repo, "skills", "docs", "SKILL.md"), projectJourneyAdvancedSkill)
	projectJourneyGit(t, transport.repo, "add", ".")
	projectJourneyGit(t, transport.repo, "commit", "-m", "advance disposable Grok source")
	projectJourneyGit(t, transport.repo, "push", "--quiet", transport.remote, "main")
	journey.run(t, journey.a, "repair", installation.InstallationID, "--target", "grok")
	projectGrokOwned(t, journey, journey.a, projectJourneySkill)
	journey.run(t, journey.a, "update", "docs", "--target", "grok")
	updated, current := projectGrokOwned(t, journey, journey.a, projectJourneyAdvancedSkill)
	if updated.Source.TreeDigest == installation.Source.TreeDigest || current.ClientBindingID != binding.ClientBindingID || current.TargetLocator != binding.TargetLocator {
		t.Fatal("Grok update lost ownership or exact pin")
	}
	status := journey.run(t, journey.a, "deps", "status")
	if !strings.Contains(status, `"activation_prepared"`) || strings.Contains(status, `"activation_active"`) {
		t.Fatal("status promoted unqualified native loading", status)
	}
	journey.run(t, journey.a, "remove", "docs", "--target", "grok")
	projectJourneyAbsent(t, current.TargetLocator)
	if !reflect.DeepEqual(userBefore, securityFixtureTree(t, user)) || !reflect.DeepEqual(bBefore, securityFixtureTree(t, journey.b)) {
		t.Fatal("Grok A lifecycle changed original user or B")
	}
	if runner.calls == 0 {
		t.Fatal("composed contract did not exercise read-only source discovery")
	}
}
func projectGrokOwned(t *testing.T, journey projectJourney, project, expected string) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	installation, binding := journey.owned(t, project)
	if binding.ClientID != "grok" || binding.Scope != "project" || binding.NativeProfileRoot != filepath.Join(project, ".grok") || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationPackageValid {
		t.Fatalf("Grok binding lost conservative project authority %+v", binding)
	}
	if projectJourneyRead(t, filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md")) != expected {
		t.Fatal("Grok exact immutable skill differs")
	}
	if _, err := os.Stat(filepath.Join(binding.TargetLocator, "mcp.json")); !os.IsNotExist(err) {
		t.Fatal("Grok bundled MCP projected", err)
	}
	return installation, binding
}
