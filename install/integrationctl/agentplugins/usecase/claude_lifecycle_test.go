package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type fakeClaudeLifecycleRunner struct {
	configRoot string
	commands   []legacyports.Command
	hide       bool
}

func (runner *fakeClaudeLifecycleRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.commands = append(runner.commands, command)
	if runner.hide {
		return legacyports.CommandResult{Stdout: []byte("[]")}, nil
	}
	root := filepath.Join(runner.configRoot, "skills")
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return legacyports.CommandResult{Stdout: []byte("[]")}, nil
	}
	if err != nil {
		return legacyports.CommandResult{}, err
	}
	listed := make([]map[string]any, 0)
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name()[0] == '.' {
			continue
		}
		path := filepath.Join(root, entry.Name())
		body, readErr := os.ReadFile(filepath.Join(path, ".claude-plugin", "plugin.json"))
		if readErr != nil {
			continue
		}
		var manifest map[string]any
		if json.Unmarshal(body, &manifest) != nil {
			continue
		}
		name, _ := manifest["name"].(string)
		listed = append(listed, map[string]any{"id": name + "@skills-dir", "version": manifest["version"], "scope": "user", "enabled": true, "installPath": path})
	}
	sort.Slice(listed, func(i, j int) bool { return listed[i]["id"].(string) < listed[j]["id"].(string) })
	body, err := json.Marshal(listed)
	return legacyports.CommandResult{Stdout: body}, err
}

func TestClaudeLifecycleAddUpdateRepairRemoveWithIsolatedConfig(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	config := filepath.Join(t.TempDir(), "claude-config")
	client := domain.DetectedClient{ClientID: domain.ClientClaude, Status: domain.DetectionDetected, ConfigRoot: config, ExecutablePath: "/test/bin/claude"}
	planner := clientplanner.Planner{ManagedRoot: t.TempDir()}
	runner := &fakeClaudeLifecycleRunner{configRoot: config}
	stager := providers.Stager{}
	service.Planner, service.Targets, service.Stager = planner, planner, stager
	service.Activator = providers.Activator{Runner: runner}
	service.NativeObserver = providers.NativeIdentityObserver{Runner: runner, Stager: stager}

	input := addInput(t, client, "https://example.com/claude")
	input.BackendExecutable = client.ExecutablePath
	input.DryRun = true
	if _, err := service.Add(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if len(runner.commands) != 0 {
		t.Fatalf("dry-run executed Claude CLI: %+v", runner.commands)
	}

	input.DryRun, input.Confirmed = false, true
	added, err := service.Add(context.Background(), input)
	if err != nil || added.Activation.Activation != domain.ActivationActive {
		entries, _ := os.ReadDir(filepath.Join(config, "skills"))
		t.Fatalf("add=%+v err=%v commands=%+v entries=%+v", added, err, runner.commands, entries)
	}
	active := added.Plan.ActivePath
	if _, err := os.Stat(filepath.Join(active, ".claude-plugin", "plugin.json")); err != nil {
		t.Fatal(err)
	}

	setEnvelopeVersion(t, &input.Envelope, "1.1.0", "sha256:source-tree-v2", "sha256:manifest-v2")
	input.Envelope.Source.ResolvedRevision = "def456"
	input.OperationID = "operation-two"
	updated, err := service.Update(context.Background(), input)
	if err != nil || updated.Activation.Activation != domain.ActivationActive {
		t.Fatalf("update=%+v err=%v", updated, err)
	}

	if err := os.WriteFile(filepath.Join(active, ".claude-plugin", "plugin.json"), []byte(`{"name":"demo","version":"tampered"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range state.Installations[0].Clients {
		binding.Authentication = domain.AuthenticationComplete
		state.Installations[0].Clients[key] = binding
	}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	input.InstallationID = added.InstallationID
	input.OperationID = "operation-three"
	repaired, err := service.Repair(context.Background(), input)
	if err != nil || repaired.Activation.Activation != domain.ActivationActive || repaired.Activation.Authentication != domain.AuthenticationComplete {
		t.Fatalf("repair=%+v err=%v", repaired, err)
	}
	state, err = store.Load()
	if err != nil || onlyBinding(state.Installations[0]).Authentication != domain.AuthenticationComplete {
		t.Fatalf("repair downgraded completed authentication: state=%+v err=%v", state, err)
	}

	removed, err := service.Remove(context.Background(), RemoveInput{
		Selector: added.InstallationID, Client: client, Scope: domain.ScopeUser, Confirmed: true,
		OperationID: "operation-four", BackendExecutable: client.ExecutablePath,
	})
	if err != nil || !removed.Mutated {
		t.Fatalf("remove=%+v err=%v", removed, err)
	}
	if _, err := os.Stat(active); !os.IsNotExist(err) {
		t.Fatalf("managed Claude path remains after remove: %v", err)
	}
	state, err = store.Load()
	if err != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 0 {
		t.Fatalf("state=%+v err=%v", state, err)
	}
	for _, command := range runner.commands {
		if fmt.Sprint(command.Argv[1:]) != "[plugin list --json]" {
			t.Fatalf("unexpected Claude mutation command: %+v", command.Argv)
		}
	}
}

func TestClaudeFailedInstallVerificationHasDeterministicRemovalCompensation(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	config := filepath.Join(t.TempDir(), "claude-config")
	client := domain.DetectedClient{ClientID: domain.ClientClaude, Status: domain.DetectionDetected, ConfigRoot: config, ExecutablePath: "/test/bin/claude"}
	planner := clientplanner.Planner{ManagedRoot: t.TempDir()}
	runner := &fakeClaudeLifecycleRunner{configRoot: config}
	stager := providers.Stager{}
	service.Planner, service.Targets, service.Stager = planner, planner, stager
	service.Activator = providers.Activator{Runner: runner}
	service.NativeObserver = providers.NativeIdentityObserver{Runner: runner, Stager: stager}
	input := addInput(t, client, "https://example.com/claude-failure")
	input.BackendExecutable, input.Confirmed = client.ExecutablePath, true
	runner.hide = true
	result, err := service.Add(context.Background(), input)
	if err == nil || result.Activation.Activation != domain.ActivationFailed {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if _, statErr := os.Stat(result.Plan.ActivePath); statErr != nil {
		t.Fatalf("failed install lost its auditable managed artifact: %v", statErr)
	}
	removed, removeErr := service.Remove(context.Background(), RemoveInput{
		Selector: result.InstallationID, Client: client, Scope: domain.ScopeUser, Confirmed: true,
		OperationID: "operation-compensate", BackendExecutable: client.ExecutablePath,
	})
	if removeErr != nil || !removed.Mutated {
		t.Fatalf("compensation=%+v err=%v", removed, removeErr)
	}
	state, loadErr := store.Load()
	if loadErr != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 0 {
		t.Fatalf("state after compensation=%+v err=%v", state, loadErr)
	}
}
