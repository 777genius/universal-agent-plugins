package usecase

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

type failingRuntimeDataManager struct {
	providers.PluginDataManager
	calls int
}

func (manager *failingRuntimeDataManager) PrepareRuntime(context.Context, domain.PackageEnvelope, domain.DeliveryPlan, string) error {
	manager.calls++
	return errors.New("cold npm unavailable")
}

func stdioRuntimeInput(t *testing.T, client domain.DetectedClient, source string) AddInput {
	t.Helper()
	input := addInput(t, client, source)
	input.Envelope.MCP = domain.MCPComponent{Present: true, Enabled: true, Servers: map[string]domain.MCPServer{
		"local": {Name: "local", Type: "stdio", Decoded: map[string]any{
			"type": "stdio", "command": "sh", "args": []any{"-c", "echo ${PLUGIN_DATA}"},
		}},
	}}
	input.Envelope.Inventory.MCPServers = []string{"local"}
	input.Confirmed = true
	return input
}

func TestRuntimePreparationFailureLeavesSingleInstallUnchanged(t *testing.T) {
	service, store, cursor := serviceFixture(t)
	manager := &failingRuntimeDataManager{PluginDataManager: service.PluginData.(providers.PluginDataManager)}
	service.PluginData = manager
	input := stdioRuntimeInput(t, cursor, "./cold-runtime")
	result, err := service.Add(context.Background(), input)
	if err == nil || !strings.Contains(err.Error(), "cold npm unavailable") || result.Mutated || manager.calls != 1 {
		t.Fatalf("add result = %+v, error = %v, calls = %d", result, err, manager.calls)
	}
	state, err := store.Load()
	if err != nil || len(state.Installations) != 0 {
		t.Fatalf("state after runtime failure = %+v, %v", state.Installations, err)
	}
	if _, err := os.Lstat(result.Plan.ActivePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("client package after runtime failure: %v", err)
	}
	if entries, err := os.ReadDir(manager.Base); err == nil && len(entries) != 0 {
		t.Fatalf("new PLUGIN_DATA survived failure: %v", entries)
	}
}

func TestRuntimePreparationFailureLeavesGroupUnchanged(t *testing.T) {
	service, store, cursor := serviceFixture(t)
	manager := &failingRuntimeDataManager{PluginDataManager: service.PluginData.(providers.PluginDataManager)}
	service.PluginData = manager
	kiro := domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
	first := stdioRuntimeInput(t, cursor, "./cold-group")
	second := stdioRuntimeInput(t, kiro, "./cold-group")
	result, err := service.AddGroup(context.Background(), GroupInput{
		Targets: []AddInput{first, second}, OperationGroupID: "cold-group", Confirmed: true,
	})
	if err == nil || !strings.Contains(err.Error(), "cold npm unavailable") || result.Mutated || result.Phase != GroupPhasePreparationFailed || manager.calls != 1 {
		t.Fatalf("group result = %+v, error = %v, calls = %d", result, err, manager.calls)
	}
	state, err := store.Load()
	if err != nil || len(state.Installations) != 0 {
		t.Fatalf("state after group runtime failure = %+v, %v", state.Installations, err)
	}
	for _, target := range result.Targets {
		if _, err := os.Lstat(target.Plan.ActivePath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("client package after group runtime failure: %v", err)
		}
	}
}
