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
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/windsurf"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type rawDevinProfileRunner struct {
	t                           *testing.T
	project, executable, global string
	calls                       int
}

func (runner *rawDevinProfileRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.calls++
	if command.Dir != runner.project || !reflect.DeepEqual(command.Argv, []string{runner.executable, "skills", "list", "--json"}) || command.StdoutLimitBytes != 1<<20 {
		runner.t.Fatalf("raw Project Skill query escaped selected authority: %+v", command)
	}
	if _, bounded := ctx.Deadline(); !bounded {
		runner.t.Fatal("raw Project Skill query has no deadline")
	}
	if body, err := os.ReadFile(filepath.Join(runner.global, "config.json")); err != nil || string(body) != `{"version":1,"auto_update":false}` {
		runner.t.Fatal("native query did not retain the known original global policy", string(body), err)
	}
	body, err := json.Marshal([]any{map[string]any{"name": "pdf", "description": "Read PDFs", "provider": "Devin",
		"base_dir": filepath.Join(runner.project, ".devin", "skills", "pdf"), "display_name": "pdf", "triggers": []string{"user", "model"}, "warnings": []string{}, "errors": []string{}}})
	return legacyports.CommandResult{Stdout: body, CleanupComplete: true}, err
}

func rawDevinProjectProfileFixture(t *testing.T, authority string) (savedJourney, *rawDevinProfileRunner, []string, string) {
	t.Helper()
	_, journey, source, _ := managementMutableWebsite(t)
	base, err := filepath.EvalSymlinks(journey.cli.root)
	if err != nil {
		t.Fatal(err)
	}
	project, global, legacy := filepath.Join(base, "project"), filepath.Join(base, "original-xdg", "devin"), filepath.Join(base, "home", ".codeium", "windsurf")
	for _, root := range []string{project, global, legacy, filepath.Join(base, "bin")} {
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
		dependencyWrite(t, filepath.Join(root, "foreign.txt"), "foreign original bytes")
	}
	policy := `{"version":1,"auto_update":false}`
	if authority == "unknown-policy" {
		policy = `{"version":1,"auto_update":false,"unknown":true}`
	}
	dependencyWrite(t, filepath.Join(global, "config.json"), policy)
	dependencyWrite(t, filepath.Join(legacy, "mcp_config.json"), `{"mcpServers":{"foreign":{"disabled":true}},"keep":"Cascade"}`)
	client := domain.DetectedClient{ClientID: domain.ClientWindsurf, Status: domain.DetectionDetected, Version: "3000.12.57",
		ExecutablePath: filepath.Join(base, "bin", "devin"), ConfigRoot: legacy, NativeConfigRoot: global}
	dependencyWrite(t, client.ExecutablePath, "inert selected executable; only the controlled runner observes it")
	if err := os.Chmod(client.ExecutablePath, 0700); err != nil {
		t.Fatal(err)
	}
	switch authority {
	case "missing":
		client.NativeConfigRoot = ""
	case "legacy-alias":
		client.NativeConfigRoot = legacy
	}
	// Ambient paths must never repair a missing explicit host observation.
	t.Setenv("XDG_CONFIG_HOME", filepath.Dir(global))
	stateRoot := filepath.Join(project, ".uap")
	journey.cli.store = statev2.Store{Path: filepath.Join(stateRoot, "state-v2.json"), ProjectRoot: project}
	journey.cli.operations = filepath.Join(stateRoot, "operations-v2")
	app := &journey.cli.app
	app.Scope, app.ProjectRoot, app.ManagedRoot = domain.ScopeProject, project, filepath.Join(stateRoot, "managed")
	app.DependenciesRoot, app.StateStore = project, journey.cli.store
	app.EnvironmentLockPath, app.EnvironmentStoreRoot = filepath.Join(stateRoot, "environment.lock"), filepath.Join(stateRoot, "store")
	app.Detector = staticDetector{clients: []domain.DetectedClient{client}}
	app.ClientRegistry, err = clients.NewRegistry(windsurf.New())
	if err != nil {
		t.Fatal(err)
	}
	runner := &rawDevinProfileRunner{t: t, project: project, executable: client.ExecutablePath, global: global}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	app.Installer, err = installer.New(installer.Config{ProjectRoot: project, StateRoot: stateRoot, StateFile: journey.cli.store.Path,
		Registry: app.ClientRegistry, Runner: runner, HelperExecutable: helper, EnableNativeObserver: true})
	if err != nil {
		t.Fatal(err)
	}
	return journey, runner, []string{global, legacy, filepath.Join(project, "foreign.txt")}, source
}

func TestRawProjectSkillsRetainOriginalUserPolicyAuthority(t *testing.T) {
	for _, authority := range []string{"observed", "missing", "unknown-policy", "legacy-alias"} {
		t.Run(authority, func(t *testing.T) {
			journey, runner, protected, source := rawDevinProjectProfileFixture(t, authority)
			before := map[string]map[string]savedJourneyFile{}
			for _, path := range protected[:2] {
				before[path] = savedJourneyTree(t, path)
			}
			foreign, err := os.ReadFile(protected[2])
			if err != nil {
				t.Fatal(err)
			}
			out := journey.run(t, "skills", "add", source, "--skill", "pdf", "--target", "windsurf", "--scope", "project",
				"--project-root", runner.project, "--no-ui", "--yes", "--format", "json")
			var document map[string]any
			if err := json.Unmarshal([]byte(out), &document); err != nil {
				t.Fatal(err)
			}
			binding := savedJourneyBinding(t, journey.state(t), "windsurf")
			if binding.NativeProfileRoot != filepath.Join(runner.project, ".devin") || binding.Scope != "project" || binding.Verification != domain.VerificationInstalled {
				t.Fatalf("Project write authority changed: %+v", binding)
			}
			if authority == "observed" {
				if runner.calls != 1 || binding.Activation != domain.ActivationActive || binding.Authentication != domain.AuthenticationNotRequired || document["readiness"] != "ready" {
					t.Fatalf("raw Project Skills lost original User policy authority: calls=%d binding=%+v output=%s", runner.calls, binding, out)
				}
			} else if runner.calls != 0 || binding.Activation != domain.ActivationPrepared || document["readiness"] == "ready" {
				t.Fatalf("unobserved User policy promoted raw Project Skills: calls=%d binding=%+v output=%s", runner.calls, binding, out)
			}
			for _, path := range protected[:2] {
				if !reflect.DeepEqual(before[path], savedJourneyTree(t, path)) {
					t.Fatal("raw Project installation changed original User/Cascade files", path)
				}
			}
			if after, err := os.ReadFile(protected[2]); err != nil || string(after) != string(foreign) {
				t.Fatal("raw Project installation changed foreign project bytes", err)
			}
		})
	}
}

func TestRawProjectSkillsRepeatHydratesOnlySelectedVersion(t *testing.T) {
	journey, runner, _, source := rawDevinProjectProfileFixture(t, "observed")
	probed, err := journey.cli.app.Detector.Detect(t.Context())
	if err != nil || len(probed) != 1 {
		t.Fatalf("fixture detector: %v %v", probed, err)
	}
	discovered := append([]domain.DetectedClient(nil), probed...)
	discovered[0].Version = ""
	detector := &projectVersionDetector{clients: discovered, probed: probed}
	journey.cli.app.Detector = detector
	argv := []string{"skills", "add", source, "--skill", "pdf", "--target", "windsurf", "--scope", "project", "--project-root", runner.project, "--no-ui", "--yes", "--format", "json"}
	first := journey.run(t, argv...)
	var document map[string]any
	if err := json.Unmarshal([]byte(first), &document); err != nil || document["readiness"] != "ready" {
		t.Fatalf("first add: %s %v", first, err)
	}
	binding := savedJourneyBinding(t, journey.state(t), "windsurf")
	managed := savedJourneyTree(t, binding.TargetLocator)
	native := savedJourneyTree(t, filepath.Join(runner.project, ".devin"))
	held := map[string][]byte{}
	for _, name := range []string{".uap/state-v2.json", "uap.toml", "uap.lock"} {
		held[name], err = os.ReadFile(filepath.Join(runner.project, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
	}
	calls, probes := runner.calls, len(detector.targets)
	repeated := journey.run(t, argv...)
	if err := json.Unmarshal([]byte(repeated), &document); err != nil {
		t.Fatal(err)
	}
	items, ok := document["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("repeat items: %s", repeated)
	}
	item, ok := items[0].(map[string]any)
	if !ok || item["status"] != "unchanged" || document["readiness"] != "ready" || runner.calls != calls+6 || len(detector.targets) != probes+2 || detector.broadCalls != 0 {
		t.Fatalf("repeat lacked fresh exact version/catalog: %s native=%d probes=%v", repeated, runner.calls, detector.targets)
	}
	for _, targets := range detector.targets {
		if !reflect.DeepEqual(targets, []domain.ClientID{domain.ClientWindsurf}) {
			t.Fatalf("unrelated version probe: %v", targets)
		}
	}
	if !reflect.DeepEqual(binding, savedJourneyBinding(t, journey.state(t), "windsurf")) || !reflect.DeepEqual(managed, savedJourneyTree(t, binding.TargetLocator)) || !reflect.DeepEqual(native, savedJourneyTree(t, filepath.Join(runner.project, ".devin"))) {
		t.Fatal("read-only repeat rewrote saved binding or owned native/package bytes")
	}
	for name, body := range held {
		current, err := os.ReadFile(filepath.Join(runner.project, filepath.FromSlash(name)))
		if err != nil || string(current) != string(body) {
			t.Fatalf("repeat changed state or lock %s: %v", name, err)
		}
	}
}
