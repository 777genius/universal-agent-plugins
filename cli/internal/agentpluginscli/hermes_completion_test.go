package agentpluginscli

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/hermes"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Only the native protocol is inert: acquisition, envelope loading, ownership,
// review, stale checks and the installer all use production implementations.
type hermesCompletionRunner struct {
	project, profile string
	trusted          []string
	grants           int
}

func (runner *hermesCompletionRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	args := command.Argv[1:]
	var output string
	switch strings.Join(args, " ") {
	case "-C " + runner.project + " rev-parse --show-toplevel":
		output = filepath.ToSlash(runner.project) + "\n"
	case "config path":
		output = filepath.Join(runner.profile, "config.yaml") + "\n"
	case "config get --json skills.trusted_project_dirs":
		body, _ := json.Marshal(runner.trusted)
		output = string(body)
	case "skills trust " + runner.project:
		runner.grants++
		runner.trusted = []string{runner.project}
		output = "Trusted: " + runner.project + "\n"
	case "config get --json skills.project_discovery":
		output = "true"
	default:
		return legacyports.CommandResult{}, fmt.Errorf("unexpected inert native command: %q", args)
	}
	return legacyports.CommandResult{Stdout: []byte(output), CleanupComplete: true}, nil
}

func setHermesCompletionHost(t *testing.T, journey *savedJourney) *hermesCompletionRunner {
	t.Helper()
	base, err := filepath.EvalSymlinks(journey.cli.root)
	if err != nil {
		t.Fatal(err)
	}
	project, home := filepath.Join(base, "project"), filepath.Join(base, "home")
	profile := filepath.Join(home, ".hermes")
	hermesCompletionWrite(t, filepath.Join(project, ".git", "HEAD"), "inert Git marker\n")
	hermesCompletionWrite(t, filepath.Join(profile, "config.yaml"), "model: retained\n")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HERMES_HOME", profile)
	client := domain.DetectedClient{ClientID: domain.ClientHermes, ConfigRoot: profile, Status: domain.DetectionDetected,
		ExecutablePath: hermesCompletionBundle(t, base), Version: "0.23.0"}
	stateRoot := filepath.Join(project, ".uap")
	journey.cli.store = statev2.Store{Path: filepath.Join(stateRoot, "state-v2.json"), ProjectRoot: project}
	journey.cli.operations = filepath.Join(stateRoot, "operations")
	app := &journey.cli.app
	app.Scope, app.ProjectRoot, app.UserHome = domain.ScopeProject, project, home
	app.ManagedRoot, app.DependenciesRoot = filepath.Join(stateRoot, "managed"), stateRoot
	app.EnvironmentLockPath, app.EnvironmentStoreRoot = filepath.Join(stateRoot, "environment.lock"), filepath.Join(stateRoot, "store")
	app.StateStore, app.Detector = journey.cli.store, staticDetector{clients: []domain.DetectedClient{client}}
	app.ClientRegistry, err = clients.NewRegistry(hermes.New())
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	runner := &hermesCompletionRunner{project: project, profile: profile}
	app.Installer, err = installer.New(installer.Config{ProjectRoot: project, StateRoot: stateRoot, Registry: app.ClientRegistry,
		Runner: runner, HelperExecutable: helper, EnableNativeObserver: true,
		LookupExecutable: func(string) (string, error) { return filepath.Join(base, "git"), nil }})
	if err != nil {
		t.Fatal(err)
	}
	return runner
}

func hermesCompletionBundle(t *testing.T, base string) string {
	t.Helper()
	root, launcher := filepath.Join(base, "inert-bundle"), "hermes"
	if runtime.GOOS == "windows" {
		launcher += ".exe"
	}
	for _, name := range []string{"hermes-agent", "venv/site", "tools/python"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(name)), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"bin/" + launcher, "tools/python/python"} {
		hermesCompletionWrite(t, filepath.Join(root, filepath.FromSlash(name)), "inert fixture; never executed")
	}
	target := map[string]string{"linux/amd64": "linux-x64", "darwin/arm64": "darwin-arm64", "windows/amd64": "win32-x64"}[runtime.GOOS+"/"+runtime.GOARCH]
	manifest := fmt.Sprintf(`{"schema":1,"ref":"2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19","target":%q,"repo":"hermes-agent","venv":"venv","store":"tools","launchers":["hermes"],"runtime":{"repoDir":"hermes-agent","toolsDir":"tools","storePython":"tools/python/python","sitePackages":"venv/site","commands":{"hermes":%q}}}`, target, "bin/"+launcher)
	hermesCompletionWrite(t, filepath.Join(root, "manifest.json"), manifest)
	return filepath.Join(root, "bin", launcher)
}

func TestHermesCompletionRawReviewRepeatAndStaleDisclosure(t *testing.T) {
	_, journey, source, _ := managementMutableWebsite(t)
	runner := setHermesCompletionHost(t, &journey)
	request := SkillsBatchRequest{Selections: []SkillsSelection{{Website: source, Name: "pdf", Alias: "pdf"}}, TargetIDs: []string{"hermes"}}
	first, err := journey.cli.app.PrepareSkills(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	hermesCompletionDisclosure(t, strings.Join(first.Warnings(), "\n"))
	if err := first.Check(t.Context()); err != nil || runner.grants != 0 {
		t.Fatalf("review changed trust or failed: grants=%d err=%v", runner.grants, err)
	}
	first.Close() // Declining the first review must not create an installation.
	if runner.grants != 0 || len(journey.state(t).Installations) != 0 {
		t.Fatal("declined review changed installation or trust")
	}
	first, err = journey.cli.app.PrepareSkills(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	result, err := first.Apply(t.Context())
	if err != nil || result.Status != "succeeded" || runner.grants != 1 || result.Items[0].Client.Activation != string(domain.ActivationPrepared) {
		t.Fatalf("confirmed raw add: %+v grants=%d err=%v", result, runner.grants, err)
	}
	before := savedJourneyBinding(t, journey.state(t), "hermes")
	requirements, locked := journey.intent(t)
	runner.trusted = nil // Deliberate native user untrust, never installer rollback.
	repeat, err := journey.cli.app.PrepareSkills(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer repeat.Close()
	if repeat.Actions()[0].Action != "repair" || runner.grants != 1 {
		t.Fatalf("repeat was not an inert reviewed repair: %+v", repeat.Actions())
	}
	hermesCompletionDisclosure(t, strings.Join(repeat.Warnings(), "\n"))
	result, err = repeat.Apply(t.Context())
	if err != nil || result.Status != "succeeded" || runner.grants != 2 {
		t.Fatalf("confirmed repeat: %+v grants=%d err=%v", result, runner.grants, err)
	}
	after := savedJourneyBinding(t, journey.state(t), "hermes")
	newRequirements, newLock := journey.intent(t)
	if before.ClientBindingID != after.ClientBindingID || !reflect.DeepEqual(before.PackageRevision, after.PackageRevision) ||
		!reflect.DeepEqual(requirements, newRequirements) || !reflect.DeepEqual(locked, newLock) {
		t.Fatal("repeat changed exact binding, source pin or intent")
	}
	stale, err := journey.cli.app.PrepareSkills(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer stale.Close()
	stale.warnings = nil
	if _, err := stale.Apply(t.Context()); !errors.Is(err, ErrSkillsPlanStale) || runner.grants != 2 {
		t.Fatalf("changed review disclosure was applied: grants=%d err=%v", runner.grants, err)
	}
}

func hermesCompletionDisclosure(t *testing.T, text string) {
	t.Helper()
	for _, want := range []string{"whole Git repository root", ".hermes/skills", ".agents/skills", "including ones added later", "trust stays after UAP removal", "quarantines"} {
		if !strings.Contains(text, want) {
			t.Fatalf("review omitted %q: %s", want, text)
		}
	}
}

func TestHermesCompletionSavedRepeatAndMixedMissingProjection(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		t.Run(fmt.Sprintf("mixed=%v", mixed), func(t *testing.T) {
			origin := hermesCompletionArchive(t, mixed)
			journey := newSavedJourney(t, t.TempDir())
			journey.cli.app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: t.TempDir(), HTTPClient: origin.Client()}
			runner := setHermesCompletionHost(t, &journey)
			journey.run(t, "add", origin.URL+"/demo.zip", "--target", "hermes", "--scope", "project", "--project-root", journey.cli.app.ProjectRoot, "--yes", "--format", "json")
			before := savedJourneyBinding(t, journey.state(t), "hermes")
			user, _ := os.ReadFile(filepath.Join(runner.profile, "config.yaml"))
			runner.trusted, runner.grants = nil, 0
			journey.run(t, "deps", "sync", "--locked", "--scope", "project", "--project-root", journey.cli.app.ProjectRoot, "--yes", "--format", "json")
			if runner.grants != 0 {
				t.Fatal("ordinary sync regranted native trust")
			}
			if mixed {
				for _, object := range before.NativeObjects {
					if object.Kind == "hermes_project_skill" {
						if err := os.RemoveAll(object.Path); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			command := &cobra.Command{Use: "add"}
			command.SetContext(t.Context())
			var output bytes.Buffer
			command.SetOut(&output)
			opts := &options{scope: "project", target: "hermes", yes: true, format: "json"}
			snapshot, err := snapshotEnvironment(command, journey.cli.app)
			if err != nil {
				t.Fatal(err)
			}
			request, err := prepareSavedSync(command, journey.cli.app, opts, snapshot, snapshot.Requirements, snapshot.Lock, snapshot.ManifestBytes, []string{"demo"}, nil, []string{"hermes"})
			if err != nil {
				t.Fatal(err)
			}
			if len(request.prepared) != 1 || request.prepared[0].Action != "repair" || runner.grants != 0 {
				t.Fatalf("saved repeat lacks inert exact repair: %+v", request.prepared)
			}
			hermesCompletionDisclosure(t, request.prepared[0].Reason)
			adapter := &dependencyInstaller{app: journey.cli.app, opts: opts, detected: request.detected, trustAliases: request.aliases}
			prepared, err := adapter.Prepare(t.Context(), request.prepared[0])
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = prepared.Close() }()
			complete, err := journey.cli.app.Installer.VerifyPreparedProjection(t.Context(), prepared)
			if err != nil || !complete {
				t.Fatalf("immutable projection lost coherence: complete=%v err=%v", complete, err)
			}
			if mixed {
				observed, _ := journey.cli.app.Installer.VerifyPreparedBinding(t.Context(), prepared)
				if observed {
					t.Fatal("missing native Skill was reported coherent")
				}
			}
			if err := prepared.Close(); err != nil {
				t.Fatal(err)
			}
			if err := applyDependencySync(command, journey.cli.app, opts, request, true); err != nil {
				t.Fatal(err)
			}
			var decoded map[string]any
			if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
				t.Fatalf("saved JSON: %v", err)
			}
			wantGrants := 1
			if mixed {
				wantGrants = 0
			}
			after := savedJourneyBinding(t, journey.state(t), "hermes")
			currentUser, _ := os.ReadFile(filepath.Join(runner.profile, "config.yaml"))
			if runner.grants != wantGrants || string(user) != string(currentUser) || before.ClientBindingID != after.ClientBindingID ||
				!reflect.DeepEqual(before.PackageRevision, after.PackageRevision) || !reflect.DeepEqual(before.NativeObjects, after.NativeObjects) {
				t.Fatalf("repair changed authority or mixed repair granted trust: grants=%d want=%d", runner.grants, wantGrants)
			}
			runner.trusted = nil
			maintenance := &dependencyInstaller{app: journey.cli.app, opts: opts, detected: request.detected}
			ordinary, err := maintenance.Prepare(t.Context(), request.prepared[0])
			if err != nil {
				t.Fatal(err)
			}
			if _, err := maintenance.Apply(t.Context(), ordinary); err != nil {
				t.Fatal(err)
			}
			if err := ordinary.Close(); err != nil {
				t.Fatal(err)
			}
			if runner.grants != wantGrants || len(runner.trusted) != 0 {
				t.Fatal("ordinary repair regranted native trust")
			}
			for _, object := range before.NativeObjects {
				if object.Kind == "hermes_project_skill" {
					if _, err := os.Stat(filepath.Join(object.Path, "SKILL.md")); err != nil {
						t.Fatalf("incomplete native projection was not repaired: %v", err)
					}
				}
			}
		})
	}
}

func hermesCompletionWrite(t *testing.T, name, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func hermesCompletionArchive(t *testing.T, mixed bool) *httptest.Server {
	t.Helper()
	files := map[string]string{
		"plugin.json":          `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo","version":"1.0.0","description":"Hermes completion fixture"}`,
		"skills/docs/SKILL.md": "---\nname: docs\ndescription: Inert completion test\n---\nRetained content.\n",
	}
	if mixed {
		files["mcp.json"] = `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"https://example.test/mcp"}}}`
	}
	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	for name, content := range files {
		member, err := writer.Create("publisher/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := member.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/demo.zip" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body.Bytes())
	}))
	t.Cleanup(server.Close)
	return server
}
