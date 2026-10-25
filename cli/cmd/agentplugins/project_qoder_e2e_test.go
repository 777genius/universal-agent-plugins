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

type projectQoderDetector struct{ root string }

func (d projectQoderDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return []domain.DetectedClient{{ClientID: domain.ClientQoder, ConfigRoot: d.root, Status: domain.DetectionDetected}}, nil
}

type projectQoderNoNative struct{ calls int }

func (r *projectQoderNoNative) Run(_ context.Context, c legacyports.Command) (legacyports.CommandResult, error) {
	r.calls++
	return legacyports.CommandResult{}, fmt.Errorf("unexpected Qoder native command %v", c.Argv)
}

func configureProjectQoder(t *testing.T, app agentpluginscli.App, user string, runner *projectQoderNoNative) agentpluginscli.App {
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
	app.Detector = projectQoderDetector{user}
	app.Lifecycle.Activator = providers.Activator{Registry: app.ClientRegistry, Runner: runner}
	app.Lifecycle.NativeObserver = providers.NativeIdentityObserver{Registry: app.ClientRegistry, Runner: runner}
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return configureProjectQoder(t, selected, user, runner), nil
	}
	return app
}

// Actual saved CLI/TLS Git, immutable pin, typed state and directory
// transactions. Every native-client command is forbidden by the injected runner.
func TestProjectQoderSavedLifecycleSeparatesUserAndProjects(t *testing.T) {
	transport := newProjectJourneyTransport(t, "git")
	journey := newProjectJourney(t, transport)
	user := filepath.Join(journey.home, ".qoder")
	projectJourneyWrite(t, filepath.Join(user, "plugins", "installed_plugins_v2.json"), "original unrelated global registry")
	// Keep an actual original user installation/state as an independent sentinel.
	// Qoder user registry mutation is intentionally outside this project slice.
	journey.run(t, "", "add", transport.endpoint, "--alias", "user-docs", "--target", "gemini")
	userGemini := securityFixtureTree(t, filepath.Join(journey.home, ".gemini"))
	runner := &projectQoderNoNative{}
	journey.app = configureProjectQoder(t, journey.app, user, runner)
	userBefore := securityFixtureTree(t, user)
	userStateBefore := projectJourneyRead(t, filepath.Join(journey.cache, "state-v2.json"))
	for _, project := range []string{journey.a, journey.b} {
		journey.run(t, project, "add", transport.endpoint, "--alias", "docs", "--target", "qoder")
		projectQoderOwned(t, journey, project, projectJourneySkill)
	}
	bBefore := securityFixtureTree(t, journey.b)
	installation, binding := journey.owned(t, journey.a)
	for _, args := range [][]string{
		{"add", transport.endpoint, "--alias", "docs", "--target", "qoder"},
		{"update", "docs", "--target", "qoder"},
		{"repair", installation.InstallationID, "--target", "qoder"},
	} {
		state := projectJourneyRead(t, filepath.Join(journey.a, ".uap", "state-v2.json"))
		if _, err := journey.execute(journey.a, append(append([]string(nil), args...), "--activation-complete")...); err == nil {
			t.Fatal("saved Qoder request accepted attestation")
		}
		if state != projectJourneyRead(t, filepath.Join(journey.a, ".uap", "state-v2.json")) {
			t.Fatal("refused attestation changed state")
		}
		output := journey.run(t, journey.a, args...)
		if strings.Contains(output, `"mutated":true`) || state != projectJourneyRead(t, filepath.Join(journey.a, ".uap", "state-v2.json")) {
			t.Fatal("intact saved Prepared operation changed state", output)
		}
	}
	journey.run(t, journey.a, "deps", "sync", "--locked", "--yes")
	view := filepath.Join(binding.TargetLocator, ".agentplugins-qoder-native")
	projectJourneyWrite(t, filepath.Join(view, "skills", "docs", "SKILL.md"), "managed view drift")
	projectJourneyWrite(t, filepath.Join(transport.repo, "skills", "docs", "SKILL.md"), projectJourneyAdvancedSkill)
	projectJourneyGit(t, transport.repo, "add", ".")
	projectJourneyGit(t, transport.repo, "commit", "-m", "advance disposable Qoder source")
	projectJourneyGit(t, transport.repo, "push", "--quiet", transport.remote, "main")
	journey.run(t, journey.a, "repair", installation.InstallationID, "--target", "qoder")
	projectQoderOwned(t, journey, journey.a, projectJourneySkill)
	if err := os.RemoveAll(binding.TargetLocator); err != nil {
		t.Fatal(err)
	}
	journey.run(t, journey.a, "repair", installation.InstallationID, "--target", "qoder")
	projectQoderOwned(t, journey, journey.a, projectJourneySkill)
	journey.run(t, journey.a, "update", "docs", "--target", "qoder")
	updated, current := projectQoderOwned(t, journey, journey.a, projectJourneyAdvancedSkill)
	if updated.Source.TreeDigest == installation.Source.TreeDigest || current.ClientBindingID != binding.ClientBindingID || current.TargetLocator != binding.TargetLocator {
		t.Fatal("saved update lost immutable source or binding authority")
	}
	calls := runner.calls
	status := journey.run(t, journey.a, "deps", "status")
	if runner.calls != calls || !strings.Contains(status, "activation_prepared") || strings.Contains(status, "activation_active") {
		t.Fatal("status called native or promoted activation", status)
	}
	journey.run(t, journey.a, "remove", "docs", "--target", "qoder")
	projectJourneyAbsent(t, current.TargetLocator)
	if runner.calls != 0 || !reflect.DeepEqual(userGemini, securityFixtureTree(t, filepath.Join(journey.home, ".gemini"))) || !reflect.DeepEqual(userBefore, securityFixtureTree(t, user)) || !reflect.DeepEqual(bBefore, securityFixtureTree(t, journey.b)) || userStateBefore != projectJourneyRead(t, filepath.Join(journey.cache, "state-v2.json")) {
		t.Fatal("A lifecycle changed original user state/native registry or B")
	}
}

func projectQoderOwned(t *testing.T, j projectJourney, project, expected string) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	installation, b := j.owned(t, project)
	if b.ClientID != "qoder" || b.Scope != "project" || b.NativeProfileRoot != filepath.Join(project, ".uap", "managed", "clients", "qoder") || b.Activation != domain.ActivationPrepared || b.Verification != domain.VerificationInstalled || len(b.NativeObjects) != 1 {
		t.Fatalf("Qoder lost exact Prepared authority %+v", b)
	}
	if projectJourneyRead(t, filepath.Join(b.TargetLocator, ".agentplugins-qoder-native", "skills", "docs", "SKILL.md")) != expected {
		t.Fatal("native import view differs from selected immutable skill")
	}
	for _, surface := range []string{"mcp.json", ".mcp.json", "hooks", "agents"} {
		if _, err := os.Lstat(filepath.Join(b.TargetLocator, ".agentplugins-qoder-native", surface)); !os.IsNotExist(err) {
			t.Fatal("unqualified native resource projected", surface, err)
		}
	}
	return installation, b
}
