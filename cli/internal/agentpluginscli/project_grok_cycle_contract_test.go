//go:build linux

package agentpluginscli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/plannertest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type cycleGrokCLIRunner struct{ project, executable string }

func (r cycleGrokCLIRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	if command.Dir != r.project || !reflect.DeepEqual(command.Argv, []string{r.executable, "inspect", "--json"}) {
		return legacyports.CommandResult{}, fmt.Errorf("unregistered inert Grok CLI command")
	}
	plugins := []any{}
	root := filepath.Join(r.project, ".grok", "plugins")
	entries, err := os.ReadDir(root)
	if err != nil && !os.IsNotExist(err) {
		return legacyports.CommandResult{}, err
	}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		body, err := os.ReadFile(filepath.Join(path, ".grok-plugin", "plugin.json"))
		if err != nil {
			return legacyports.CommandResult{}, err
		}
		var manifest map[string]any
		if err := json.Unmarshal(body, &manifest); err != nil {
			return legacyports.CommandResult{}, err
		}
		plugins = append(plugins, map[string]any{"name": manifest["name"], "scope": "project", "path": path, "enabled": true})
	}
	body, err := json.Marshal(map[string]any{"grokVersion": "1.0.70", "cwd": r.project, "projectRoot": r.project + "/", "projectTrusted": true, "plugins": plugins, "skills": []any{}})
	return legacyports.CommandResult{Stdout: body}, err
}

// Exercise public Cobra routing and the real grouped lifecycle with an inert
// in-memory inspector. No vendor executable is run or qualified.
func TestProjectGrokPublicGroupedRepairRestagesIntactAndMissingPackage(t *testing.T) {
	client := fixtureClient(t, domain.ClientGrok)
	f := newCLIFixture(t, []domain.DetectedClient{client})
	project := filepath.Join(f.root, "project")
	for _, path := range []string{project, client.ConfigRoot} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	client.ExecutablePath, client.Version = filepath.Join(f.root, "inert-grok"), "1.0.70"
	if err := os.WriteFile(client.ExecutablePath, []byte("inert bytes; never executed\n"), 0700); err != nil {
		t.Fatal(err)
	}
	f.app.Detector = staticDetector{clients: []domain.DetectedClient{client}}
	stateRoot := filepath.Join(project, ".uap")
	f.store = statev2.Store{Path: filepath.Join(stateRoot, "state-v2.json"), ProjectRoot: project}
	f.app.Scope, f.app.ProjectRoot, f.app.ManagedRoot, f.app.StateStore = domain.ScopeProject, project, filepath.Join(stateRoot, "managed"), f.store
	f.app.EnvironmentLockPath = filepath.Join(stateRoot, "environment.lock")
	planner := plannertest.NewPlanner(clientplanner.Planner{ProjectRoot: project, ManagedRoot: f.app.ManagedRoot})
	f.app.Planner, f.app.Targets = planner, planner
	runner := cycleGrokCLIRunner{project, client.ExecutablePath}
	f.app.Lifecycle.ProjectRoot, f.app.Lifecycle.StateStore = project, f.store
	f.app.Lifecycle.Planner, f.app.Lifecycle.Targets = planner, planner
	f.app.Lifecycle.Activator = providerstest.NewActivator(providers.Activator{Runner: runner})
	f.app.Lifecycle.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Runner: runner, Stager: f.app.Lifecycle.Stager})
	f.app.Lifecycle.Lock = processlock.Lock{Path: filepath.Join(stateRoot, "mutation.lock")}
	f.app.Lifecycle.Kernel = transaction.Kernel{StateStore: f.store, Directory: dirswap.Manager{JournalDir: filepath.Join(stateRoot, "operations")}}
	var err error
	f.app.Installer, err = installer.New(installer.Config{ProjectRoot: project, StateRoot: stateRoot, Registry: f.app.ClientRegistry, Runner: runner, HelperExecutable: client.ExecutablePath, EnableNativeObserver: true, TrustedLocalPackages: true})
	if err != nil {
		t.Fatal(err)
	}
	f.app.Lifecycle.CheckNamespace = f.app.Installer.CheckProjectNamespace
	source := writeCLIPlugin(t)
	skill := []byte("---\nname: guide\ndescription: Inert public repair\n---\nRecorded source.\n")
	if err := os.MkdirAll(filepath.Join(source, "skills", "guide"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "skills", "guide", "SKILL.md"), skill, 0600); err != nil {
		t.Fatal(err)
	}
	run := func(verb, selector string, mutated bool) {
		t.Helper()
		args := []string{verb, selector, "--scope", "project", "--project-root", project, "--target", "grok", "--format", "json"}
		if verb == "add" {
			args = append(args, "--no-save")
		}
		stdout, stderr, err := f.execute(false, args...)
		if err != nil {
			t.Fatalf("actual public %s: %v\n%s\n%s", verb, err, stdout, stderr)
		}
		var result map[string]any
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatal(err)
		}
		data := result["data"].(map[string]any)
		out := data["targets"].([]any)[0].(map[string]any)["output"].(map[string]any)["result"].(map[string]any)
		if out["mutated"] != mutated {
			t.Fatalf("actual public %s mutated=%v want%t", verb, out["mutated"], mutated)
		}
	}
	run("add", source, true)
	state, err := f.store.Load()
	if err != nil || len(state.Installations) != 1 {
		t.Fatal(err)
	}
	i := state.Installations[0]
	var binding domain.ClientBinding
	for _, b := range i.Clients {
		binding = b
	}
	run("repair", i.InstallationID, true)
	path := filepath.Join(binding.TargetLocator, "skills", "guide", "SKILL.md")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(f.store.Path)
	if err != nil {
		t.Fatal(err)
	}
	files := preparedClaudeCLIFiles(t, binding.TargetLocator)
	_, _, err = f.execute(false, "repair", i.InstallationID, "--scope", "project", "--project-root", project, "--target", "grok", "--format", "json")
	after, readErr := os.ReadFile(f.store.Path)
	if err == nil || !strings.Contains(err.Error(), "native identity ownership is indeterminate") || readErr != nil || string(before) != string(after) || !reflect.DeepEqual(files, preparedClaudeCLIFiles(t, binding.TargetLocator)) {
		t.Fatal("public grouped repair adopted or changed partially missing package", err, readErr)
	}
	if len(files) != 2 || files[filepath.Join(binding.TargetLocator, "plugin.json")] == "" || files[filepath.Join(binding.TargetLocator, ".grok-plugin", "plugin.json")] == "" {
		t.Fatal("inert owned package membership changed before exact absence fault")
	}
	for _, relative := range []string{"plugin.json", ".grok-plugin/plugin.json", "skills/guide", "skills", ".grok-plugin", "."} {
		if err := os.Remove(filepath.Join(binding.TargetLocator, filepath.FromSlash(relative))); err != nil {
			t.Fatal("exact nonrecursive owned package absence fault", err)
		}
	}
	run("repair", i.InstallationID, true)
	restored, err := os.ReadFile(path)
	if err != nil || string(restored) != string(skill) {
		t.Fatal("public grouped repair did not restore exact missing leaf", err)
	}
}
