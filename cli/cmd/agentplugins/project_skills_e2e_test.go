package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

type projectSkillsSource struct {
	input     string
	transport *projectJourneyTransport
	advance   func()
}

func newProjectSkillsSource(t *testing.T, kind string) projectSkillsSource {
	t.Helper()
	if kind == "git" {
		transport := newProjectJourneyTransport(t, "git")
		return projectSkillsSource{input: "uap-fixture/skills@main", transport: transport, advance: func() {
			projectJourneyWrite(t, filepath.Join(transport.repo, "skills", "docs", "SKILL.md"), projectJourneyAdvancedSkill)
			projectJourneyGit(t, transport.repo, "add", ".")
			projectJourneyGit(t, transport.repo, "commit", "-m", "advance raw project skill")
			projectJourneyGit(t, transport.repo, "push", "--quiet", transport.remote, "main")
		}}
	}
	var mu sync.Mutex
	body := projectJourneySkill
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/.well-known/agent-skills/index.json":
			_, _ = fmt.Fprint(w, `{"skills":[{"name":"docs","files":["SKILL.md"]}]}`)
		case "/.well-known/agent-skills/docs/SKILL.md":
			_, _ = fmt.Fprint(w, body)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return projectSkillsSource{input: server.URL, transport: &projectJourneyTransport{server: server}, advance: func() {
		mu.Lock()
		defer mu.Unlock()
		body = projectJourneyAdvancedSkill
	}}
}

// The raw GitHub adapter intentionally strips user Git config. Its explicit
// URL fixture seam maps one public coordinate to a disposable bare repository;
// real native Git still resolves and fetches every immutable source object.
func projectSkillsGitAcquirer(app agentpluginscli.App, remote string) agentpluginscli.App {
	acquirer := app.SourceAcquirer.(sourceacquisition.Acquirer)
	acquirer.URLForRepo = func(string) string { return remote }
	app.SourceAcquirer = acquirer
	app.SourceAcquirerForDryRun = func(root string) agentpluginscli.SourceAcquirer {
		preview := acquirer
		preview.TempRoot = root
		return preview
	}
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return projectSkillsGitAcquirer(selected, remote), nil
	}
	return app
}

func projectSkillPin(t *testing.T, journey projectJourney, project string) environment.LockedPackage {
	t.Helper()
	root := project
	if root == "" {
		root = journey.cache
	}
	var lock environment.Lock
	if err := json.Unmarshal([]byte(projectJourneyRead(t, filepath.Join(root, "uap.lock"))), &lock); err != nil {
		t.Fatal(err)
	}
	if len(lock.Packages) != 1 {
		t.Fatalf("expected exact raw Skill pin: %+v", lock)
	}
	return lock.Packages[0]
}

func assertProjectSkill(t *testing.T, journey projectJourney, project, expected string) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	installation, binding := journey.owned(t, project)
	scope, environmentID := domain.ScopeUser, environment.EnvironmentUserDefault
	if project != "" {
		scope, environmentID = domain.ScopeProject, environment.EnvironmentProjectDefault
	}
	if binding.Scope != string(scope) || binding.NativeProfileRoot != journey.nativeRoot(project) || binding.Verification != domain.VerificationInstalled {
		t.Fatalf("raw Skill lost selected scope: %+v", binding)
	}
	if project != "" && binding.Activation != domain.ActivationPrepared {
		t.Fatalf("fixture claimed native project activation: %+v", binding)
	}
	if journey.state(t, project).EnvironmentReceipts[0].EnvironmentID != environmentID {
		t.Fatal("raw Skill receipt uses wrong environment")
	}
	if err := pathpolicy.RequireContainedChild(journey.namespace(project), binding.TargetLocator); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{binding.TargetLocator, journey.nativeRoot(project)} {
		if got := projectJourneyRead(t, filepath.Join(root, "skills", "docs", "SKILL.md")); got != expected {
			t.Fatalf("wrong raw Skill bytes in %s: %q", root, got)
		}
	}
	if got := projectJourneyRead(t, filepath.Join(journey.nativeRoot(project), "skills", "foreign", "SKILL.md")); got != "foreign skill\n" {
		t.Fatal("raw Skill changed foreign skill")
	}
	return installation, binding
}

// Exercise production CLI composition, source acquisition, saved environment and
// filesystem transaction. Fixture detection is not native client discovery.
func TestProjectSkillsComposedGitAndHTTPSLifecycle(t *testing.T) {
	for _, kind := range []string{"git", "https"} {
		t.Run(kind, func(t *testing.T) {
			source := newProjectSkillsSource(t, kind)
			journey := newProjectJourney(t, source.transport)
			if kind == "git" {
				journey.app = projectSkillsGitAcquirer(journey.app, source.transport.remote)
			}
			initial := securityFixtureTree(t, journey.a)
			journey.run(t, journey.a, "skills", "add", source.input, "--skill", "docs", "--target", "gemini", "--no-ui", "--dry-run")
			if !reflect.DeepEqual(initial, securityFixtureTree(t, journey.a)) {
				t.Fatal("raw Skill preview wrote project files")
			}
			projectJourneyAbsent(t, filepath.Join(journey.cache, "state-v2.json"), filepath.Join(journey.cache, "uap.toml"), filepath.Join(journey.cache, "store"), filepath.Join(journey.b, ".uap"))
			for _, root := range []string{"", journey.a, journey.b} {
				journey.run(t, root, "skills", "add", source.input, "--skill", "docs", "--target", "gemini", "--no-ui", "--yes")
				assertProjectSkill(t, journey, root, projectJourneySkill)
			}
			pin := projectSkillPin(t, journey, journey.a)
			if (kind == "git" && (pin.Source.Git == nil || pin.Source.Git.Kind != environment.SkillKind)) || (kind == "https" && pin.Source.Web == nil) {
				t.Fatalf("lost raw Skill provenance: %+v", pin)
			}
			for _, root := range []string{journey.cache, journey.b} {
				for _, name := range []string{"uap.toml", "uap.lock"} {
					if projectJourneyRead(t, filepath.Join(root, name)) != projectJourneyRead(t, filepath.Join(journey.a, name)) {
						t.Fatal("scope leaked into portable raw Skill agreement")
					}
				}
			}
			userNative, bNative := securityFixtureTree(t, journey.nativeRoot("")), securityFixtureTree(t, journey.nativeRoot(journey.b))
			userState := projectJourneyRead(t, filepath.Join(journey.cache, "state-v2.json"))
			bState := projectJourneyRead(t, filepath.Join(journey.namespace(journey.b), "state-v2.json"))
			statePath := filepath.Join(journey.namespace(journey.a), "state-v2.json")
			before := projectJourneyRead(t, statePath)
			journey.run(t, journey.a, "deps", "sync", "--locked", "--yes")
			if before != projectJourneyRead(t, statePath) {
				t.Fatal("stable raw Skill replay rewrote project receipt")
			}
			installation, binding := journey.owned(t, journey.a)
			projectJourneyWrite(t, filepath.Join(binding.TargetLocator, "drift.txt"), "managed projection drift\n")
			source.advance()
			journey.run(t, journey.a, "repair", installation.InstallationID, "--target", "gemini")
			assertProjectSkill(t, journey, journey.a, projectJourneySkill)
			projectJourneyAbsent(t, filepath.Join(binding.TargetLocator, "drift.txt"))
			journey.run(t, journey.a, "update", "docs", "--target", "gemini")
			updated, selected := assertProjectSkill(t, journey, journey.a, projectJourneyAdvancedSkill)
			if updated.InstallationID != installation.InstallationID || selected.ClientBindingID != binding.ClientBindingID || selected.TargetLocator != binding.TargetLocator || updated.Source.TreeDigest == installation.Source.TreeDigest {
				t.Fatal("raw Skill update replaced ownership or failed to advance bytes")
			}
			journey.run(t, journey.a, "remove", "docs", "--target", "gemini")
			projectJourneyAbsent(t, binding.TargetLocator, filepath.Join(journey.nativeRoot(journey.a), "skills", "docs"))
			if len(journey.state(t, journey.a).EnvironmentReceipts) != 0 {
				t.Fatal("removed raw Skill retained ownership receipt")
			}
			journey.assertOtherScopeUnchanged(t, "", userNative, userState)
			journey.assertOtherScopeUnchanged(t, journey.b, bNative, bState)
		})
	}
}

func TestProjectSkillsRejectUnsupportedBeforeAcquisition(t *testing.T) {
	app, user, a, _ := projectCompositionFixture(t)
	journey := projectJourney{app: app}
	if _, err := journey.execute(a, "skills", "add", "https://invalid.example/never-acquire", "--skill", "docs", "--target", "chatgpt", "--yes"); err == nil || !strings.Contains(err.Error(), "does not support project scope") {
		t.Fatalf("unsupported raw Skills boundary: %v", err)
	}
	selected, err := app.ForScope(domain.ScopeProject, a)
	if err != nil {
		t.Fatal(err)
	}
	_, err = selected.PrepareSkills(context.Background(), agentpluginscli.SkillsBatchRequest{Selections: []agentpluginscli.SkillsSelection{{Website: "https://invalid.example/never-acquire", Name: "docs"}}, TargetIDs: []string{"chatgpt"}})
	if err == nil || !strings.Contains(err.Error(), "does not support project scope") {
		t.Fatalf("direct batch bypassed project qualification: %v", err)
	}
	projectJourneyAbsent(t, user, filepath.Join(a, ".uap"), filepath.Join(a, "uap.toml"))
}

func TestProjectSkillsPreparedPlanRejectsReplacedRootBeforeLock(t *testing.T) {
	source := newProjectSkillsSource(t, "https")
	journey := newProjectJourney(t, source.transport)
	app, err := journey.app.ForScope(domain.ScopeProject, journey.a)
	if err != nil {
		t.Fatal(err)
	}
	request := agentpluginscli.SkillsBatchRequest{Selections: []agentpluginscli.SkillsSelection{{Website: source.input, Name: "docs"}}, TargetIDs: []string{"gemini"}}
	prepared, err := app.PrepareSkills(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Close()
	original := journey.a + "-original"
	if err = os.Rename(journey.a, original); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(original) })
	if err = os.Mkdir(journey.a, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err = prepared.Apply(context.Background()); !errors.Is(err, agentpluginscli.ErrSkillsPlanStale) {
		t.Fatalf("replaced project accepted by prepared batch: %v", err)
	}
	if _, err = app.PrepareSkills(context.Background(), request); err == nil {
		t.Fatal("direct prepare accepted replaced project namespace")
	}
	if _, err = app.StartSkillsPicker(context.Background(), agentpluginscli.SkillsPickerRequest{Input: "invalid/never-acquire"}); err == nil {
		t.Fatal("public picker accepted replaced project namespace")
	}
	entries, err := os.ReadDir(journey.a)
	if err != nil || len(entries) != 0 {
		t.Fatalf("replaced project acquired lock or state: %v %v", entries, err)
	}
	projectJourneyAbsent(t, filepath.Join(original, ".uap"), filepath.Join(journey.cache, "store"))
}
