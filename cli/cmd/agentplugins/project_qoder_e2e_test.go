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
	app.Lifecycle.NativeObserver = providers.NativeIdentityObserver{Stager: app.Lifecycle.Stager, Registry: app.ClientRegistry, Runner: runner}
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

// Exercise the same direct local, explicit-target CLI group route as the native
// cycle. No Qoder process is permitted; repair and retention use production APIs.
func TestProjectQoderDirectGroupRepairAndDefaultRemovalKeepExactAuthority(t *testing.T) {
	transport := newProjectJourneyTransport(t, "git")
	j := newProjectJourney(t, transport)
	user := filepath.Join(j.home, ".qoder")
	projectJourneyWrite(t, filepath.Join(user, "plugins", "installed_plugins_v2.json"), "original registry\n")
	runner := &projectQoderNoNative{}
	j.app = configureProjectQoder(t, j.app, user, runner)
	beforeUser := securityFixtureTree(t, user)
	for _, project := range []string{j.a, j.b} {
		j.run(t, project, "add", transport.repo, "--target", "qoder", "--no-save")
	}
	installed, binding := projectQoderDirectOwned(t, j, projectJourneySkill)
	beforeB := securityFixtureTree(t, j.b)
	if len(installed.DataReceipts) != 0 || installed.DataRetained || binding.DataReceiptID != "" {
		t.Fatal("skills-only install invented a data allocation")
	}
	stateBefore := projectJourneyRead(t, filepath.Join(j.a, ".uap", "state-v2.json"))
	for _, args := range [][]string{{"add", transport.repo, "--target", "qoder", "--no-save"}, {"update", installed.DeclaredName, "--target", "qoder"}} {
		output := j.run(t, j.a, args...)
		if !strings.Contains(output, `"mutated":false`) || stateBefore != projectJourneyRead(t, filepath.Join(j.a, ".uap", "state-v2.json")) {
			t.Fatal("direct unchanged group changed state", output)
		}
	}
	output := j.run(t, j.a, "repair", installed.InstallationID, "--target", "qoder")
	intact, current := projectQoderDirectOwned(t, j, projectJourneySkill)
	if !strings.Contains(output, `"mutated":true`) || intact.InstallationID != installed.InstallationID || current.ClientBindingID != binding.ClientBindingID || intact.Source.TreeDigest != installed.Source.TreeDigest {
		t.Fatal("explicit direct CLI group repair lost exact restaging authority", output)
	}
	projectJourneyWrite(t, filepath.Join(transport.repo, "skills", "docs", "SKILL.md"), projectJourneyAdvancedSkill)
	j.run(t, j.a, "update", transport.repo, "--target", "qoder")
	updated, current := projectQoderDirectOwned(t, j, projectJourneyAdvancedSkill)
	if updated.Source.TreeDigest == installed.Source.TreeDigest || current.TargetLocator != binding.TargetLocator {
		t.Fatal("same-version direct update lost source or physical authority")
	}
	// This inert CLI contract owns the complete selected package. The hosted
	// cycle separately authorizes each leaf before nonrecursive fault removal.
	if err := os.RemoveAll(current.TargetLocator); err != nil {
		t.Fatal(err)
	}
	j.run(t, j.a, "repair", installed.InstallationID, "--target", "qoder")
	repaired, _ := projectQoderDirectOwned(t, j, projectJourneyAdvancedSkill)
	if repaired.Source.TreeDigest != updated.Source.TreeDigest {
		t.Fatal("frozen local repair did not restore its exact recorded digest")
	}
	j.run(t, j.a, "remove", installed.DeclaredName, "--target", "qoder")
	projectJourneyAbsent(t, current.TargetLocator)
	state := j.state(t, j.a)
	if len(state.Installations) != 1 || !state.Installations[0].DataRetained || len(state.Installations[0].Clients) != 0 || len(state.Installations[0].DataReceipts) != 1 {
		t.Fatal("default skills-only removal did not retain its actual owner allocation")
	}
	if runner.calls != 0 || !reflect.DeepEqual(beforeUser, securityFixtureTree(t, user)) || !reflect.DeepEqual(beforeB, securityFixtureTree(t, j.b)) {
		t.Fatal("direct A lifecycle invoked native or changed original user/B authority")
	}
}

func projectQoderDirectOwned(t *testing.T, j projectJourney, expected string) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	state := j.state(t, j.a)
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 || len(state.EnvironmentReceipts) != 0 {
		t.Fatal("direct --no-save route lost exact unsaved state")
	}
	installation := state.Installations[0]
	for _, binding := range installation.Clients {
		if binding.ClientID != "qoder" || binding.Scope != "project" || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled || binding.NativeProfileRoot != filepath.Join(j.a, ".uap", "managed", "clients", "qoder") || len(binding.NativeObjects) != 1 || binding.NativeObjects[0].Path != binding.TargetLocator {
			t.Fatal("direct Prepared receipt ownership differs")
		}
		if projectJourneyRead(t, filepath.Join(binding.TargetLocator, ".agentplugins-qoder-native", "skills", "docs", "SKILL.md")) != expected {
			t.Fatal("direct Qoder native view does not match frozen source")
		}
		return installation, binding
	}
	t.Fatal("direct binding missing")
	return domain.Installation{}, domain.ClientBinding{}
}
