package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func (journey projectJourney) execute(project string, args ...string) (string, error) {
	var output, stderr bytes.Buffer
	app := journey.app
	app.Input, app.Output, app.ErrorOutput, app.Terminal = strings.NewReader(""), &output, &stderr, false
	root := agentpluginscli.NewRoot(app)
	root.SetOut(&output)
	root.SetErr(&stderr)
	args = append(args, "--format", "json")
	if project != "" {
		args = append(args, "--scope", "project", "--project-root", project)
	}
	root.SetArgs(args)
	err := root.Execute()
	return output.String() + stderr.String(), err
}

func (journey projectJourney) run(t *testing.T, project string, args ...string) string {
	t.Helper()
	var sequence uint64
	if journey.transport != nil && journey.transport.diagnostic != nil {
		sequence = journey.transport.diagnostic.failureSequence()
	}
	output, err := journey.execute(project, args...)
	if err != nil {
		if journey.transport != nil && journey.transport.diagnostic != nil {
			journey.transport.diagnostic.outsideInvocations(sequence)
		}
		t.Fatalf("%v in project %q: %v\n%s", args, project, err, output)
	}
	return output
}

func (journey projectJourney) state(t *testing.T, project string) domain.StateFileV2 {
	t.Helper()
	scope := domain.ScopeUser
	if project != "" {
		scope = domain.ScopeProject
	}
	app, err := journey.app.ForScope(scope, project)
	if err != nil {
		t.Fatal(err)
	}
	state, err := app.StateStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func (journey projectJourney) owned(t *testing.T, project string) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	state := journey.state(t, project)
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 || len(state.EnvironmentReceipts) != 1 {
		t.Fatalf("expected one owned package, binding and environment receipt: %+v", state)
	}
	installation := state.Installations[0]
	for _, binding := range installation.Clients {
		return installation, binding
	}
	t.Fatal("binding missing")
	return domain.Installation{}, domain.ClientBinding{}
}

func (journey projectJourney) namespace(project string) string {
	if project == "" {
		return journey.cache
	}
	return filepath.Join(project, ".uap")
}

func (journey projectJourney) nativeRoot(project string) string {
	if project == "" {
		return filepath.Join(journey.home, ".gemini")
	}
	return filepath.Join(project, ".gemini")
}

func (journey projectJourney) assertOwnedContext(t *testing.T, project string) domain.Installation {
	t.Helper()
	installation, binding := journey.owned(t, project)
	scope, environmentID := domain.ScopeUser, environment.EnvironmentUserDefault
	activation := domain.ActivationActive
	if project != "" {
		scope, environmentID, activation = domain.ScopeProject, environment.EnvironmentProjectDefault, domain.ActivationPrepared
	}
	state := journey.state(t, project)
	if binding.Scope != string(scope) || binding.NativeProfileRoot != journey.nativeRoot(project) || binding.Activation != activation || binding.Verification != domain.VerificationInstalled || state.EnvironmentReceipts[0].EnvironmentID != environmentID {
		t.Fatalf("scope/lifecycle/owner context mismatch: %+v, %+v", binding, state.EnvironmentReceipts)
	}
	if err := pathpolicy.RequireContainedChild(journey.namespace(project), binding.TargetLocator); err != nil {
		t.Fatal(err)
	}
	if got := projectJourneyRead(t, filepath.Join(journey.nativeRoot(project), "skills", "docs", "SKILL.md")); got != projectJourneySkill {
		t.Fatalf("native skill bytes = %q", got)
	}
	settings := projectJourneyRead(t, filepath.Join(journey.nativeRoot(project), "settings.json"))
	for _, retained := range []string{"retained foreign settings", "foreign.test", `"docs"`} {
		if !strings.Contains(settings, retained) {
			t.Fatalf("native settings missing %q: %s", retained, settings)
		}
	}
	return installation
}

func (journey projectJourney) assertOtherScopeUnchanged(t *testing.T, project string, native map[string]string, state string) {
	t.Helper()
	if !reflect.DeepEqual(native, securityFixtureTree(t, journey.nativeRoot(project))) || state != projectJourneyRead(t, filepath.Join(journey.namespace(project), "state-v2.json")) {
		t.Fatalf("operation changed another environment %q", project)
	}
	installation, binding := journey.owned(t, project)
	if projectJourneyRead(t, filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md")) != projectJourneySkill {
		t.Fatal("operation changed another environment's managed bytes")
	}
	for _, receipt := range installation.DataReceipts {
		if projectJourneyRead(t, filepath.Join(receipt.Locator, "keep.txt")) != "retained environment data\n" {
			t.Fatal("operation changed another environment's persistent data")
		}
	}
}

func (journey projectJourney) assertPortablePair(t *testing.T, root string) [2]string {
	t.Helper()
	pair := [2]string{projectJourneyRead(t, filepath.Join(root, "uap.toml")), projectJourneyRead(t, filepath.Join(root, "uap.lock"))}
	for _, body := range pair {
		for _, path := range []string{journey.home, journey.cache, journey.a, journey.b, journey.transport.repo, journey.transport.remote} {
			if strings.Contains(body, path) || strings.Contains(body, filepath.ToSlash(path)) || strings.Contains(body, strings.ReplaceAll(path, `\`, `\\`)) {
				t.Fatalf("portable agreement contains machine path %q", path)
			}
		}
	}
	return pair
}

func (journey projectJourney) inspectNoRecovery(t *testing.T, project string) {
	t.Helper()
	app, err := journey.app.ForScope(domain.ScopeProject, project)
	if err != nil {
		t.Fatal(err)
	}
	view, err := app.Installer.Inspect(context.Background())
	if err != nil || view.Recovery.Required {
		t.Fatalf("project inspection requires recovery: %+v, %v", view.Recovery, err)
	}
}

func projectJourneyAbsent(t *testing.T, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected absent path %s: %v", path, err)
		}
	}
}
