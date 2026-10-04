package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type liveCopilotRunner struct {
	path string
	spec string
}

func (runner *liveCopilotRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	args := command.Argv
	if len(args) >= 5 && args[1] == "plugin" && args[2] == "marketplace" && args[3] == "add" {
		runner.path = args[4]
	}
	if len(args) >= 4 && args[1] == "plugin" && args[2] == "install" {
		runner.spec = args[3]
	}
	if len(args) >= 3 && args[1] == "plugin" && args[2] == "list" {
		return legacyports.CommandResult{Stdout: []byte("Live Plugins (loaded from a local marketplace directory, never copied):\n  • " + runner.spec + " (v1.0.0) (enabled)\n      from " + runner.path + "\n")}, nil
	}
	return legacyports.CommandResult{}, nil
}

func TestImmediateMultiTargetUpdateReusesNativeBindings(t *testing.T) {
	t.Parallel()
	targets := []domain.ClientID{
		domain.ClientKiro,
		domain.ClientGemini,
		domain.ClientOpenCode,
		domain.ClientCline,
		domain.ClientWindsurf,
	}
	clients := make([]domain.DetectedClient, 0, len(targets))
	names := make([]string, 0, len(targets))
	for _, target := range targets {
		clients = append(clients, fixtureClient(t, target))
		names = append(names, string(target))
	}
	fixture := newCLIFixture(t, clients)
	plugin := writeCLIPlugin(t)
	writeCLIMCP(t, plugin)
	selected := strings.Join(names, ",")
	if _, _, err := fixture.execute(false, "add", plugin, "--target", selected, "--format", "json"); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := fixture.execute(false, "update", "demo", "--target", selected, "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Data struct {
			Status  string `json:"status"`
			Targets []struct {
				Output struct {
					Result struct {
						NoChange bool `json:"no_change"`
						Mutated  bool `json:"mutated"`
					} `json:"result"`
				} `json:"output"`
			} `json:"targets"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Status != "completed" || len(envelope.Data.Targets) != len(targets) {
		t.Fatalf("update result = %+v", envelope.Data)
	}
	for _, target := range envelope.Data.Targets {
		if !target.Output.Result.NoChange || target.Output.Result.Mutated {
			t.Fatalf("update was not idempotent: %+v", target)
		}
	}
}

func TestImmediateSharedCopilotUpdateAcceptsExactLiveRegistration(t *testing.T) {
	t.Parallel()
	copilot := fixtureClient(t, domain.ClientCopilot)
	copilot.ExecutablePath = "/test/bin/copilot"
	fixture := newCLIFixture(t, []domain.DetectedClient{copilot, fixtureClient(t, domain.ClientVSCode)})
	runner := &liveCopilotRunner{}
	fixture.app.Lifecycle.Activator = providerstest.NewActivator(providers.Activator{Runner: runner})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "copilot,vscode", "--format", "json"); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := fixture.execute(false, "update", "demo", "--target", "copilot,vscode", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Data struct {
			Status  string `json:"status"`
			Targets []struct {
				Output struct {
					Result struct {
						NoChange bool `json:"no_change"`
						Mutated  bool `json:"mutated"`
					} `json:"result"`
				} `json:"output"`
			} `json:"targets"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Status != "completed" || len(envelope.Data.Targets) != 2 {
		t.Fatalf("update result = %+v", envelope.Data)
	}
	for _, target := range envelope.Data.Targets {
		if !target.Output.Result.NoChange || target.Output.Result.Mutated {
			t.Fatalf("update was not idempotent: %+v", target)
		}
	}
}

// Two persisted logical owners of one physical backend cannot authorize an
// arbitrary map-order predecessor, even if their ownership bytes agree.
func TestSharedAddRefusesAmbiguousPhysicalOwners(t *testing.T) {
	t.Parallel()
	copilot := fixtureClient(t, domain.ClientCopilot)
	copilot.ExecutablePath = "/test/bin/copilot"
	fixture := newCLIFixture(t, []domain.DetectedClient{copilot, fixtureClient(t, domain.ClientVSCode)})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "copilot,vscode", "--format", "json"); err != nil {
		t.Fatal(err)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	owner := onlyCLIClient(state.Installations[0])
	alias := owner
	alias.ClientBindingID = owner.ClientBindingID + "-other"
	alias.Receipts = nil
	state.Installations[0].Clients[alias.ClientBindingID] = alias
	if err := fixture.store.Save(state); err != nil {
		t.Fatal(err)
	}
	paths := []string{fixture.store.Path, filepath.Join(owner.TargetLocator, "plugin.json")}
	bodies := make([][]byte, len(paths))
	infos := make([]os.FileInfo, len(paths))
	for i, path := range paths {
		bodies[i], err = os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		infos[i], err = os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "vscode", "--format", "json"); err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("ambiguous shared add = %v", err)
	}
	for i, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(body, bodies[i]) || !info.ModTime().Equal(infos[i].ModTime()) {
			t.Fatalf("ambiguous add changed %s", path)
		}
	}
}
