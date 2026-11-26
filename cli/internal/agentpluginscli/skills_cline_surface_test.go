package agentpluginscli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/cline"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type rawClineSurfaceRunner struct {
	t                   *testing.T
	project, executable string
	calls               int
}

func (runner *rawClineSurfaceRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.calls++
	expected := []string{runner.executable, "--auto-approve", "false", "--cwd", runner.project, "config", "skills", "--json"}
	if command.Dir != runner.project || !reflect.DeepEqual(command.Argv, expected) || command.StdoutLimitBytes != 1<<20 {
		runner.t.Fatalf("Cline catalog escaped selected authority: %+v", command)
	}
	if _, bounded := ctx.Deadline(); !bounded {
		runner.t.Fatal("Cline catalog has no deadline")
	}
	body, err := json.Marshal([]any{map[string]any{"name": "pdf", "description": "Read PDFs", "instructions": "First reviewed content.",
		"frontmatter": map[string]any{"name": "pdf", "description": "Read PDFs"}, "path": filepath.Join(runner.project, ".cline", "skills", "pdf", "SKILL.md")}})
	return legacyports.CommandResult{Stdout: body, CleanupComplete: true}, err
}

func rawClineSurfaceFixture(t *testing.T, surfaces []domain.ClientSurface) (savedJourney, *rawClineSurfaceRunner, string) {
	t.Helper()
	_, journey, source, _ := managementMutableWebsite(t)
	base, err := filepath.EvalSymlinks(journey.cli.root)
	if err != nil {
		t.Fatal(err)
	}
	project, user := filepath.Join(base, "project"), filepath.Join(base, "home", ".cline")
	for _, root := range []string{project, user} {
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
		dependencyWrite(t, filepath.Join(root, "foreign.txt"), "foreign original bytes")
	}
	client := domain.DetectedClient{ClientID: domain.ClientCline, Status: domain.DetectionDetected, ConfigRoot: user, Version: "3.0.73", Surfaces: surfaces}
	stateRoot := filepath.Join(project, ".uap")
	journey.cli.store = statev2.Store{Path: filepath.Join(stateRoot, "state-v2.json"), ProjectRoot: project}
	journey.cli.operations = filepath.Join(stateRoot, "operations-v2")
	app := &journey.cli.app
	app.Scope, app.ProjectRoot, app.ManagedRoot = domain.ScopeProject, project, filepath.Join(stateRoot, "managed")
	app.DependenciesRoot, app.StateStore = project, journey.cli.store
	app.EnvironmentLockPath, app.EnvironmentStoreRoot = filepath.Join(stateRoot, "environment.lock"), filepath.Join(stateRoot, "store")
	app.Detector = staticDetector{clients: []domain.DetectedClient{client}}
	app.ClientRegistry, err = clients.NewRegistry(cline.New())
	if err != nil {
		t.Fatal(err)
	}
	runner := &rawClineSurfaceRunner{t: t, project: project, executable: filepath.Join(base, "inert-cline")}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	app.Installer, err = installer.New(installer.Config{ProjectRoot: project, StateRoot: stateRoot, StateFile: journey.cli.store.Path,
		Registry: app.ClientRegistry, Runner: runner, HelperExecutable: helper, EnableNativeObserver: true,
		LookupExecutable: func(name string) (string, error) {
			if name != "cline" {
				t.Fatalf("unexpected lookup %s", name)
			}
			return runner.executable, nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	return journey, runner, source
}

func TestRawProjectSkillsPreserveActualClineSurface(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		surfaces []domain.ClientSurface
		observed bool
	}{
		{"CLI", []domain.ClientSurface{{ID: "cline_cli", Detected: true, Evidence: "actual host observation"}}, true},
		{"absent", nil, false},
		{"editor only", []domain.ClientSurface{{ID: "cline_vscode_extension", Detected: true}}, false},
		{"CLI unavailable", []domain.ClientSurface{{ID: "cline_cli", Detected: false}}, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			journey, runner, source := rawClineSurfaceFixture(t, scenario.surfaces)
			before := savedJourneyTree(t, filepath.Join(filepath.Dir(runner.project), "home", ".cline"))
			output := journey.run(t, "skills", "add", source, "--skill", "pdf", "--target", "cline", "--scope", "project", "--project-root", runner.project, "--no-ui", "--yes", "--format", "json")
			var document map[string]any
			if err := json.Unmarshal([]byte(output), &document); err != nil {
				t.Fatal(err)
			}
			binding := savedJourneyBinding(t, journey.state(t), "cline")
			if binding.Verification != domain.VerificationInstalled || binding.NativeProfileRoot != runner.project {
				t.Fatalf("wrong materialization: %+v", binding)
			}
			if scenario.observed {
				if runner.calls != 1 || document["readiness"] != "ready" || binding.Activation != domain.ActivationActive || binding.Authentication != domain.AuthenticationNotRequired {
					t.Fatalf("lost CLI surface: calls=%d binding=%+v output=%s", runner.calls, binding, output)
				}
			} else if runner.calls != 0 || document["readiness"] == "ready" || binding.Activation != domain.ActivationPrepared {
				t.Fatalf("invented CLI authority: calls=%d binding=%+v output=%s", runner.calls, binding, output)
			}
			if !reflect.DeepEqual(before, savedJourneyTree(t, filepath.Join(filepath.Dir(runner.project), "home", ".cline"))) {
				t.Fatal("Project install changed original User profile")
			}
		})
	}
}

func TestRawProjectSkillsClineRepeatQueriesFreshCatalogWithoutRewriting(t *testing.T) {
	journey, runner, source := rawClineSurfaceFixture(t, []domain.ClientSurface{{ID: "cline_cli", Detected: true, Evidence: "actual host observation"}})
	argv := []string{"skills", "add", source, "--skill", "pdf", "--target", "cline", "--scope", "project", "--project-root", runner.project, "--no-ui", "--yes", "--format", "json"}
	first := journey.run(t, argv...)
	var firstDocument map[string]any
	if err := json.Unmarshal([]byte(first), &firstDocument); err != nil || firstDocument["readiness"] != "ready" || runner.calls == 0 {
		t.Fatalf("first add did not observe the selected CLI catalog: %s %v", first, err)
	}
	state := journey.state(t)
	binding := savedJourneyBinding(t, state, "cline")
	if len(state.Installations) != 1 || state.Installations[0].InstallationID == "" || binding.Activation != domain.ActivationActive {
		t.Fatalf("first add lacks its exact Active installation: %+v", state)
	}
	installationID := state.Installations[0].InstallationID
	managed := savedJourneyTree(t, binding.TargetLocator)
	native := savedJourneyTree(t, filepath.Join(runner.project, ".cline"))
	userRoot := filepath.Join(filepath.Dir(runner.project), "home", ".cline")
	user := savedJourneyTree(t, userRoot)
	held := map[string][]byte{}
	for _, name := range []string{".uap/state-v2.json", "uap.toml", "uap.lock", "foreign.txt"} {
		body, err := os.ReadFile(filepath.Join(runner.project, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		held[name] = body
	}
	calls := runner.calls
	repeated := journey.run(t, argv...)
	var document map[string]any
	if err := json.Unmarshal([]byte(repeated), &document); err != nil {
		t.Fatal(err)
	}
	items, ok := document["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("repeat lost its selected item: %s", repeated)
	}
	item, ok := items[0].(map[string]any)
	if !ok || item["alias"] != "pdf" || item["targetId"] != "cline" || item["status"] != "unchanged" || document["readiness"] != "ready" || runner.calls <= calls {
		t.Fatalf("repeat reused readiness without a fresh selected catalog: %s calls=%d before=%d", repeated, runner.calls, calls)
	}
	after := journey.state(t)
	if len(after.Installations) != 1 || after.Installations[0].InstallationID != installationID || !reflect.DeepEqual(binding, savedJourneyBinding(t, after, "cline")) {
		t.Fatal("read-only repeat changed installation or binding identity")
	}
	if !reflect.DeepEqual(managed, savedJourneyTree(t, binding.TargetLocator)) || !reflect.DeepEqual(native, savedJourneyTree(t, filepath.Join(runner.project, ".cline"))) || !reflect.DeepEqual(user, savedJourneyTree(t, userRoot)) {
		t.Fatal("fresh repeat catalog changed managed, native or original User bytes")
	}
	for name, body := range held {
		current, err := os.ReadFile(filepath.Join(runner.project, filepath.FromSlash(name)))
		if err != nil || string(current) != string(body) {
			t.Fatalf("read-only repeat changed Project authority %s: %v", name, err)
		}
	}
}

func TestRawProjectSkillsAbsoluteClineExecutableDoesNotInventSurface(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		surfaces []domain.ClientSurface
	}{
		{"absent", nil},
		{"editor only", []domain.ClientSurface{{ID: "cline_vscode_extension", Detected: true}}},
		{"CLI unavailable", []domain.ClientSurface{{ID: "cline_cli", Detected: false}}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			journey, runner, source := rawClineSurfaceFixture(t, scenario.surfaces)
			observed, err := journey.cli.app.Detector.Detect(t.Context())
			if err != nil || len(observed) != 1 {
				t.Fatalf("fixture detector: %+v %v", observed, err)
			}
			dependencyWrite(t, runner.executable, "inert executable is not observed CLI authority")
			if err := os.Chmod(runner.executable, 0700); err != nil {
				t.Fatal(err)
			}
			observed[0].ExecutablePath = runner.executable
			journey.cli.app.Detector = staticDetector{clients: observed}
			output := journey.run(t, "skills", "add", source, "--skill", "pdf", "--target", "cline", "--scope", "project", "--project-root", runner.project, "--no-ui", "--yes", "--format", "json")
			var document map[string]any
			if err := json.Unmarshal([]byte(output), &document); err != nil {
				t.Fatal(err)
			}
			binding := savedJourneyBinding(t, journey.state(t), "cline")
			if runner.calls != 0 || document["readiness"] == "ready" || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled {
				t.Fatalf("absolute executable invented CLI surface authority: calls=%d binding=%+v output=%s", runner.calls, binding, output)
			}
		})
	}
}
