package usecase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// lifecycleCLIRunner answers Codex and Copilot inventory verbs so add/remove
// can run the real Activator without a live binary.
type lifecycleCLIRunner struct {
	commands    []legacyports.Command
	lastAdd     string
	lastInstall string
}

func (runner *lifecycleCLIRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.commands = append(runner.commands, command)
	joined := strings.Join(command.Argv, " ")
	if len(command.Argv) >= 4 && command.Argv[1] == "plugin" && command.Argv[2] == "add" {
		runner.lastAdd = command.Argv[3]
		return legacyports.CommandResult{}, nil
	}
	if len(command.Argv) >= 4 && command.Argv[1] == "plugin" && command.Argv[2] == "install" {
		runner.lastInstall = command.Argv[3]
		return legacyports.CommandResult{}, nil
	}
	if strings.Contains(joined, "plugin list --json") {
		name, marketplace := "demo", "managed"
		if parts := strings.SplitN(runner.lastAdd, "@", 2); len(parts) == 2 {
			name, marketplace = parts[0], parts[1]
		}
		return legacyports.CommandResult{Stdout: []byte(fmt.Sprintf(
			`{"installed":[{"pluginId":%q,"name":%q,"marketplaceName":%q,"installed":true,"enabled":true}]}`,
			name+"@"+marketplace, name, marketplace,
		))}, nil
	}
	if strings.HasSuffix(joined, "plugin list") && runner.lastInstall != "" {
		return legacyports.CommandResult{Stdout: []byte("Installed plugins:\n  • " + runner.lastInstall + " (v1.0.0)")}, nil
	}
	return legacyports.CommandResult{}, nil
}

func (runner *lifecycleCLIRunner) contains(fragment string) bool {
	for _, command := range runner.commands {
		if strings.Contains(strings.Join(command.Argv, " "), fragment) {
			return true
		}
	}
	return false
}

func TestRemoveGroupUninstallsManagedClientsWithoutExternalFlag(t *testing.T) {
	t.Parallel()
	for _, spec := range []struct {
		name            string
		clients         []domain.ClientID
		wantUninstall   string
		wantNoUninstall bool
	}{
		{name: "cursor", clients: []domain.ClientID{domain.ClientCursor}, wantNoUninstall: true},
		{name: "copilot", clients: []domain.ClientID{domain.ClientCopilot}, wantUninstall: "plugin uninstall"},
		{name: "vscode", clients: []domain.ClientID{domain.ClientVSCode}, wantUninstall: "plugin uninstall"},
		{name: "cursor-codex", clients: []domain.ClientID{domain.ClientCursor, domain.ClientCodex}, wantUninstall: "plugin remove"},
	} {
		spec := spec
		t.Run(spec.name, func(t *testing.T) {
			t.Parallel()
			service, store, _ := serviceFixture(t)
			runner := &lifecycleCLIRunner{}
			service.Activator = providers.Activator{Runner: runner}
			var targets []AddInput
			var detected []domain.DetectedClient
			for index, id := range spec.clients {
				client := domain.DetectedClient{
					ClientID: id, Status: domain.DetectionDetected,
					ConfigRoot: filepath.Join(t.TempDir(), "."+string(id)),
				}
				install := addInput(t, client, "https://example.com/remove-auto-"+spec.name)
				install.Confirmed = true
				install.BackendExecutable = backendForRemoveAuto(id)
				install.OperationID = fmt.Sprintf("add-%d", index)
				targets = append(targets, install)
				detected = append(detected, client)
			}
			added, err := service.AddGroup(context.Background(), GroupInput{
				Targets: targets, OperationGroupID: "remove-auto-add-" + spec.name, Confirmed: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			var managed []string
			for _, target := range added.Targets {
				managed = append(managed, target.Plan.ActivePath)
			}
			var removeTargets []RemoveInput
			for _, client := range detected {
				removeTargets = append(removeTargets, RemoveInput{
					Client: client, Scope: domain.ScopeUser, BackendExecutable: backendForRemoveAuto(client.ClientID),
				})
			}
			removed, err := service.RemoveGroup(context.Background(), RemoveGroupInput{
				Selector: added.InstallationID, Targets: removeTargets,
				OperationGroupID: "remove-auto-" + spec.name, Confirmed: true,
			})
			if err != nil {
				t.Fatalf("remove without --external-uninstalled failed: %v", err)
			}
			if !removed.Mutated || removed.Phase != GroupPhaseCompleted {
				t.Fatalf("remove result = %+v, want mutated/completed", removed)
			}
			for _, path := range managed {
				if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
					t.Fatalf("managed directory survived: %s (%v)", path, statErr)
				}
			}
			state, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 0 {
				t.Fatalf("bindings survived remove: %+v", state.Installations)
			}
			if spec.wantUninstall != "" && !runner.contains(spec.wantUninstall) {
				t.Fatalf("never ran %q: %#v", spec.wantUninstall, argvOfLifecycle(runner))
			}
			if spec.wantNoUninstall && (runner.contains("plugin remove") || runner.contains("plugin uninstall")) {
				t.Fatalf("Cursor remove invoked a client CLI: %#v", argvOfLifecycle(runner))
			}
		})
	}
}

func TestRemoveGroupUninstallsCopilotAfterManualAddWhenCLIBecomesAvailable(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	copilot := domain.DetectedClient{
		ClientID: domain.ClientCopilot, Status: domain.DetectionDetected,
		ConfigRoot: filepath.Join(t.TempDir(), ".copilot"),
	}
	install := addInput(t, copilot, "https://example.com/copilot-manual-then-cli")
	install.Confirmed = true
	added, err := service.Add(context.Background(), install)
	if err != nil {
		t.Fatal(err)
	}
	if added.Activation.Activation != domain.ActivationManual {
		t.Fatalf("add without CLI should stay manual: %+v", added.Activation)
	}
	runner := &lifecycleCLIRunner{}
	service.Activator = providers.Activator{Runner: runner}
	removed, err := service.Remove(context.Background(), RemoveInput{
		Selector: added.InstallationID, Client: copilot, Scope: domain.ScopeUser,
		Confirmed: true, BackendExecutable: "/test/bin/copilot", OperationID: "copilot-manual-remove",
	})
	if err != nil {
		t.Fatalf("CLI-backed remove after manual add failed: %v", err)
	}
	if !removed.Mutated || !removed.Deactivation.ExternalRemovalComplete {
		t.Fatalf("remove result = %+v", removed)
	}
	if _, err := os.Stat(added.Plan.ActivePath); !os.IsNotExist(err) {
		t.Fatalf("managed Copilot path survived: %v", err)
	}
	if !runner.contains("plugin uninstall") || !runner.contains("marketplace remove") {
		t.Fatalf("manual-then-CLI remove skipped Copilot uninstall: %#v", argvOfLifecycle(runner))
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations[0].Clients) != 0 {
		t.Fatalf("Copilot binding survived: %+v", state.Installations)
	}
}

func TestRemoveGroupDryRunDoesNotCallCopilotOrCodexUninstall(t *testing.T) {
	t.Parallel()
	service, _, _ := serviceFixture(t)
	runner := &lifecycleCLIRunner{}
	service.Activator = providers.Activator{Runner: runner}
	codex := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".codex")}
	copilot := domain.DetectedClient{ClientID: domain.ClientCopilot, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".copilot")}
	var targets []AddInput
	for index, client := range []domain.DetectedClient{codex, copilot} {
		install := addInput(t, client, "https://example.com/remove-dry-run")
		install.Confirmed = true
		install.BackendExecutable = backendForRemoveAuto(client.ClientID)
		install.OperationID = fmt.Sprintf("dry-add-%d", index)
		targets = append(targets, install)
	}
	added, err := service.AddGroup(context.Background(), GroupInput{
		Targets: targets, OperationGroupID: "remove-dry-add", Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	addCommands := len(runner.commands)
	planned, err := service.RemoveGroup(context.Background(), RemoveGroupInput{
		Selector: added.InstallationID,
		Targets: []RemoveInput{
			{Client: codex, Scope: domain.ScopeUser, BackendExecutable: "/test/bin/codex"},
			{Client: copilot, Scope: domain.ScopeUser, BackendExecutable: "/test/bin/copilot"},
		},
		OperationGroupID: "remove-dry", DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if planned.Mutated || len(runner.commands) != addCommands {
		t.Fatalf("dry-run mutated CLI or state: mutated=%v extra commands=%#v", planned.Mutated, argvOfLifecycle(runner)[addCommands:])
	}
	for _, target := range planned.Targets {
		if !target.Deactivation.ArtifactRemovalAllowed || target.Deactivation.ExternalRemovalComplete {
			t.Fatalf("dry-run deactivation = %+v", target.Deactivation)
		}
	}
	for _, target := range added.Targets {
		if _, err := os.Stat(target.Plan.ActivePath); err != nil {
			t.Fatalf("dry-run deleted managed path %s: %v", target.Plan.ActivePath, err)
		}
	}
}

func TestRemoveGroupBlocksChatGPTUntilExternalUninstalled(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	chatgpt := domain.DetectedClient{ClientID: domain.ClientChatGPT, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".chatgpt")}
	install := addInput(t, chatgpt, "https://example.com/chatgpt-remove-gate")
	install.Confirmed = true
	added, err := service.AddGroup(context.Background(), GroupInput{
		Targets: []AddInput{install}, OperationGroupID: "chatgpt-remove-add", Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	managedPath := added.Targets[0].Plan.ActivePath
	blocked, err := service.RemoveGroup(context.Background(), RemoveGroupInput{
		Selector:         added.InstallationID,
		Targets:          []RemoveInput{{Client: chatgpt, Scope: domain.ScopeUser}},
		OperationGroupID: "chatgpt-remove-blocked", Confirmed: true,
	})
	if err == nil || !strings.Contains(err.Error(), "did not authorize managed artifact removal") {
		t.Fatalf("ChatGPT remove without flag error = %v", err)
	}
	if blocked.Mutated {
		t.Fatalf("blocked ChatGPT remove mutated: %+v", blocked)
	}
	if _, statErr := os.Stat(managedPath); statErr != nil {
		t.Fatalf("blocked ChatGPT remove touched managed files: %v", statErr)
	}
	removed, err := service.RemoveGroup(context.Background(), RemoveGroupInput{
		Selector:         added.InstallationID,
		Targets:          []RemoveInput{{Client: chatgpt, Scope: domain.ScopeUser, ExternalUninstalled: true}},
		OperationGroupID: "chatgpt-remove-ack", Confirmed: true,
	})
	if err != nil || !removed.Mutated {
		t.Fatalf("acknowledged ChatGPT remove = %+v err=%v", removed, err)
	}
	if _, statErr := os.Stat(managedPath); !os.IsNotExist(statErr) {
		t.Fatalf("ChatGPT managed path survived acknowledged remove: %v", statErr)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations[0].Clients) != 0 {
		t.Fatalf("ChatGPT binding survived: %+v", state.Installations)
	}
}

func backendForRemoveAuto(id domain.ClientID) string {
	switch id {
	case domain.ClientCodex:
		return "/test/bin/codex"
	case domain.ClientCopilot, domain.ClientVSCode:
		return "/test/bin/copilot"
	default:
		return ""
	}
}

func argvOfLifecycle(runner *lifecycleCLIRunner) [][]string {
	result := make([][]string, 0, len(runner.commands))
	for _, command := range runner.commands {
		result = append(result, command.Argv)
	}
	return result
}
