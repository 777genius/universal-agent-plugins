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

func TestRemoveUninstallsManagedClientsWithoutExternalFlag(t *testing.T) {
	t.Parallel()
	for _, spec := range []struct {
		name          string
		targets       string
		clients       []domain.ClientID
		wantUninstall string
	}{
		{name: "cursor", targets: "cursor", clients: []domain.ClientID{domain.ClientCursor}},
		{name: "cursor,codex", targets: "cursor,codex", clients: []domain.ClientID{domain.ClientCursor, domain.ClientCodex}, wantUninstall: "plugin remove"},
		{name: "copilot", targets: "copilot", clients: []domain.ClientID{domain.ClientCopilot}, wantUninstall: "plugin uninstall"},
		{name: "copilot,vscode", targets: "copilot,vscode", clients: []domain.ClientID{domain.ClientCopilot, domain.ClientVSCode}, wantUninstall: "plugin uninstall"},
	} {
		spec := spec
		t.Run(spec.name, func(t *testing.T) {
			t.Parallel()
			var detected []domain.DetectedClient
			for _, id := range spec.clients {
				client := fixtureClient(t, id)
				switch id {
				case domain.ClientCodex:
					client.ExecutablePath = "/test/bin/codex"
				case domain.ClientCopilot, domain.ClientVSCode:
					client.ExecutablePath = "/test/bin/copilot"
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
			var managed []string
			for _, binding := range before.Installations[0].Clients {
				managed = append(managed, binding.TargetLocator)
			}
			stdout, _, err := fixture.execute(false, "remove", "demo", "--target", spec.targets, "--format", "json")
			if err != nil {
				t.Fatalf("remove without --external-uninstalled failed: %v\n%s", err, stdout)
			}
			assertBatchJSON(t, stdout, "remove", len(spec.clients), 0)
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
		})
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
