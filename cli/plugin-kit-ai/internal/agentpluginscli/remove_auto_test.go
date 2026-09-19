package agentpluginscli

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

type autoRemoveCLIRunner struct {
	commands    [][]string
	lastAdd     string
	lastInstall string
}

func (runner *autoRemoveCLIRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	argv := append([]string(nil), command.Argv...)
	runner.commands = append(runner.commands, argv)
	joined := strings.Join(argv, " ")
	if len(argv) >= 4 && argv[1] == "plugin" && argv[2] == "add" {
		runner.lastAdd = argv[3]
		return legacyports.CommandResult{}, nil
	}
	if len(argv) >= 4 && argv[1] == "plugin" && argv[2] == "install" {
		runner.lastInstall = argv[3]
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

func (runner *autoRemoveCLIRunner) contains(fragment string) bool {
	for _, argv := range runner.commands {
		if strings.Contains(strings.Join(argv, " "), fragment) {
			return true
		}
	}
	return false
}

func (runner *autoRemoveCLIRunner) invoked(executable string) bool {
	for _, argv := range runner.commands {
		if len(argv) > 0 && argv[0] == executable {
			return true
		}
	}
	return false
}

func TestRemoveUninstallsManagedClientsWithoutExternalFlag(t *testing.T) {
	t.Parallel()
	for _, spec := range []struct {
		name          string
		targets       string
		detected      []domain.ClientID
		wantUninstall string
	}{
		{name: "cursor", targets: "cursor", detected: []domain.ClientID{domain.ClientCursor}},
		{name: "cursor,codex", targets: "cursor,codex", detected: []domain.ClientID{domain.ClientCursor, domain.ClientCodex}, wantUninstall: "plugin remove"},
		{name: "copilot", targets: "copilot", detected: []domain.ClientID{domain.ClientCopilot}, wantUninstall: "plugin uninstall"},
		{name: "vscode with Copilot CLI", targets: "vscode", detected: []domain.ClientID{domain.ClientVSCode, domain.ClientCopilot}, wantUninstall: "plugin uninstall"},
		{name: "copilot,vscode", targets: "copilot,vscode", detected: []domain.ClientID{domain.ClientCopilot, domain.ClientVSCode}, wantUninstall: "plugin uninstall"},
	} {
		t.Run(spec.name, func(t *testing.T) {
			t.Parallel()
			detected := make([]domain.DetectedClient, 0, len(spec.detected))
			for _, id := range spec.detected {
				client := fixtureClient(t, id)
				switch id {
				case domain.ClientCodex:
					client.ExecutablePath = "/test/bin/codex"
				case domain.ClientCopilot:
					client.ExecutablePath = "/test/bin/copilot"
				case domain.ClientVSCode:
					client.ExecutablePath = "/test/bin/code"
				}
				detected = append(detected, client)
			}
			fixture := newCLIFixture(t, detected)
			runner := &autoRemoveCLIRunner{}
			fixture.app.Lifecycle.Activator = providers.Activator{Runner: runner}
			plugin := writeCLIPlugin(t)
			if _, _, err := fixture.execute(false, "add", plugin, "--target", spec.targets); err != nil {
				t.Fatal(err)
			}
			before, err := fixture.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			managed := make([]string, 0, len(before.Installations[0].Clients))
			for _, binding := range before.Installations[0].Clients {
				managed = append(managed, binding.TargetLocator)
			}
			stdout, _, err := fixture.execute(false, "remove", "demo", "--target", spec.targets, "--format", "json")
			if err != nil {
				t.Fatalf("remove without --external-uninstalled failed: %v\n%s", err, stdout)
			}
			assertBatchJSON(t, stdout, "remove", len(strings.Split(spec.targets, ",")), 0)
			if strings.Contains(stdout, `"status":"blocked"`) {
				t.Fatalf("remove was blocked without the flag: %s", stdout)
			}
			for _, path := range managed {
				if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
					t.Fatalf("managed path survived: %s (%v)", path, statErr)
				}
			}
			state, err := fixture.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 0 {
				t.Fatalf("bindings survived: %+v", state.Installations)
			}
			if spec.wantUninstall != "" && !runner.contains(spec.wantUninstall) {
				t.Fatalf("never ran %q: %#v", spec.wantUninstall, runner.commands)
			}
			if runner.invoked("/test/bin/code") {
				t.Fatalf("invoked VS Code CLI instead of Copilot: %#v", runner.commands)
			}
		})
	}
}

func TestVSCodeRemoveWithoutCopilotCLIRequiresExternalFlag(t *testing.T) {
	t.Parallel()
	vscode := fixtureClient(t, domain.ClientVSCode)
	vscode.ExecutablePath = "/test/bin/code"
	copilot := domain.DetectedClient{ClientID: domain.ClientCopilot, DisplayName: "copilot", Status: domain.DetectionNotDetected}
	fixture := newCLIFixture(t, []domain.DetectedClient{vscode, copilot})
	runner := &autoRemoveCLIRunner{}
	fixture.app.Lifecycle.Activator = providers.Activator{Runner: runner}
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "vscode"); err != nil {
		t.Fatal(err)
	}
	before, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	managed := onlyCLIClient(before.Installations[0]).TargetLocator
	stdout, _, err := fixture.execute(false, "remove", "demo", "--target", "vscode", "--format", "json")
	if err == nil {
		t.Fatalf("vscode remove without Copilot CLI succeeded: %s", stdout)
	}
	if !strings.Contains(err.Error(), "did not authorize managed artifact removal") && !strings.Contains(stdout, `"status":"blocked"`) {
		t.Fatalf("vscode remove error = %v stdout=%s", err, stdout)
	}
	if runner.invoked("/test/bin/code") {
		t.Fatalf("invoked VS Code CLI: %#v", runner.commands)
	}
	if _, statErr := os.Stat(managed); statErr != nil {
		t.Fatalf("blocked vscode remove touched managed files: %v", statErr)
	}
	stdout, _, err = fixture.execute(false, "remove", "demo", "--target", "vscode", "--external-uninstalled", "--format", "json")
	if err != nil {
		t.Fatalf("vscode remove with --external-uninstalled failed: %v\n%s", err, stdout)
	}
	if _, statErr := os.Stat(managed); !os.IsNotExist(statErr) {
		t.Fatalf("managed path survived acknowledged vscode remove: %v", statErr)
	}
	if runner.invoked("/test/bin/code") {
		t.Fatalf("acknowledged remove invoked VS Code CLI: %#v", runner.commands)
	}
}

func TestChatGPTRemoveWithoutExternalFlagStaysBlocked(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientChatGPT)})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "chatgpt"); err != nil {
		t.Fatal(err)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	managed := onlyCLIClient(state.Installations[0]).TargetLocator
	stdout, _, err := fixture.execute(false, "remove", "demo", "--target", "chatgpt", "--format", "json")
	if err == nil {
		t.Fatalf("ChatGPT remove without --external-uninstalled succeeded: %s", stdout)
	}
	if !strings.Contains(err.Error(), "did not authorize managed artifact removal") && !strings.Contains(stdout, `"status":"blocked"`) {
		t.Fatalf("ChatGPT remove error = %v stdout=%s", err, stdout)
	}
	if _, statErr := os.Stat(managed); statErr != nil {
		t.Fatalf("blocked ChatGPT remove touched managed files: %v", statErr)
	}
	if _, err := os.Lstat(filepath.Join(fixture.root, "data", "state-v2.json")); err != nil {
		t.Fatal(err)
	}
	after, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Installations[0].Clients) == 0 {
		t.Fatalf("blocked ChatGPT remove dropped the binding: %+v", after.Installations)
	}
}
