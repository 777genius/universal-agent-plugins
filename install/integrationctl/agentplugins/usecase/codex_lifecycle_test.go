package usecase

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/plannertest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

func TestCodexLifecycleAddRepairReaddsPlugin(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	codex := domain.DetectedClient{
		ClientID: domain.ClientCodex, Status: domain.DetectionDetected,
		ConfigRoot: filepath.Join(t.TempDir(), ".codex"), ExecutablePath: "/test/bin/codex",
	}
	planner := plannertest.NewPlanner(clientplanner.Planner{ManagedRoot: t.TempDir()})
	runner := &codexCleanupUsecaseRunner{configRoot: codex.ConfigRoot}
	stager := providers.Stager{}
	service.Planner, service.Targets, service.Stager = planner, planner, stager
	service.Activator = providers.Activator{Runner: runner}

	input := addInput(t, codex, "https://example.com/codex-repair-cache")
	input.BackendExecutable, input.Confirmed = codex.ExecutablePath, true
	added, err := service.Add(context.Background(), input)
	if err != nil || added.Activation.Activation != domain.ActivationActive {
		t.Fatalf("add=%+v err=%v commands=%+v", added, err, runner.commands)
	}
	if countCodexPluginAdds(runner) != 1 {
		t.Fatalf("add plugin add count=%d commands=%+v", countCodexPluginAdds(runner), argvOf(runner))
	}

	if err := os.WriteFile(filepath.Join(added.Plan.ActivePath, "plugin.json"), []byte(`{"name":"demo","version":"tampered"}`), 0o644); err != nil {
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
	input.OperationID = "operation-repair"
	repaired, err := service.Repair(context.Background(), input)
	if err != nil || repaired.Activation.Activation != domain.ActivationActive {
		t.Fatalf("repair=%+v err=%v commands=%+v", repaired, err, runner.commands)
	}
	if countCodexPluginAdds(runner) < 2 {
		t.Fatalf("repair did not recopy Codex cache: plugin add count=%d commands=%+v", countCodexPluginAdds(runner), argvOf(runner))
	}
	for _, command := range runner.commands {
		joined := strings.Join(command.Argv, " ")
		if strings.Contains(joined, "marketplace upgrade") {
			t.Fatalf("Codex repair used local upgrade: %+v", command.Argv)
		}
	}
}

func TestCodexRepairRecopiesCacheWhenManagedSourceIsIntact(t *testing.T) {
	t.Parallel()
	service, _, _ := serviceFixture(t)
	codex := domain.DetectedClient{
		ClientID: domain.ClientCodex, Status: domain.DetectionDetected,
		ConfigRoot: filepath.Join(t.TempDir(), ".codex"), ExecutablePath: "/test/bin/codex",
	}
	planner := plannertest.NewPlanner(clientplanner.Planner{ManagedRoot: t.TempDir()})
	runner := &codexCleanupUsecaseRunner{configRoot: codex.ConfigRoot}
	stager := providers.Stager{}
	service.Planner, service.Targets, service.Stager = planner, planner, stager
	service.Activator = providers.Activator{Runner: runner}

	input := addInput(t, codex, "https://example.com/codex-stale-cache")
	input.BackendExecutable, input.Confirmed = codex.ExecutablePath, true
	added, err := service.Add(context.Background(), input)
	if err != nil || added.Activation.Activation != domain.ActivationActive {
		t.Fatalf("add=%+v err=%v", added, err)
	}
	runner.omitFromList = true
	input.InstallationID = added.InstallationID
	input.OperationID = "operation-stale-cache"
	repaired, err := service.Repair(context.Background(), input)
	if err != nil || repaired.Activation.Activation != domain.ActivationActive {
		t.Fatalf("repair=%+v err=%v commands=%+v", repaired, err, argvOf(runner))
	}
	if countCodexPluginAdds(runner) < 2 {
		t.Fatalf("intact source did not recopy stale Codex cache: plugin add count=%d commands=%+v", countCodexPluginAdds(runner), argvOf(runner))
	}
}

func countCodexPluginAdds(runner *codexCleanupUsecaseRunner) int {
	count := 0
	for _, command := range runner.commands {
		if len(command.Argv) >= 4 && command.Argv[1] == "plugin" && command.Argv[2] == "add" {
			count++
		}
	}
	return count
}

func argvOf(runner *codexCleanupUsecaseRunner) [][]string {
	out := make([][]string, 0, len(runner.commands))
	for _, command := range runner.commands {
		out = append(out, command.Argv)
	}
	return out
}
