package usecase

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Cancel after committing both packages, before Codex activation. A verify-only
// retry fails on the exact negative listing. Recovery must register and verify
// through the production adapter without replacing package or data ownership.
func TestGroupedCLIRetryAfterCancellationPreservesPackageAndData(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner := &groupCLIRecoveryRunner{cancel: cancel}
	service.Activator = providerstest.NewActivator(providers.Activator{Runner: runner})
	claude := domain.DetectedClient{ClientID: domain.ClientClaude, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".claude"), ExecutablePath: "/test/bin/claude"}
	codex := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: canonicalCodexProfile(t, ".codex"), ExecutablePath: "/test/bin/codex"}
	inputs := []AddInput{addInput(t, claude, "https://example.com/cli-recovery"), addInput(t, codex, "https://example.com/cli-recovery")}
	for i := range inputs {
		inputs[i].BackendExecutable = inputs[i].Client.ExecutablePath
		inputs[i].Envelope.MCP = domain.MCPComponent{Present: true, Enabled: true, Servers: map[string]domain.MCPServer{
			"local":  {Name: "local", Type: "stdio", Decoded: map[string]any{"type": "stdio", "command": "sh", "args": []any{"-c", "echo ${PLUGIN_DATA}"}}},
			"remote": {Name: "remote", Type: "streamable-http", Decoded: map[string]any{"type": "streamable-http", "url": "https://example.invalid/mcp"}},
		}}
		inputs[i].Envelope.Inventory.MCPServers = []string{"local", "remote"}
	}
	stopped, err := service.AddGroup(ctx, GroupInput{Targets: inputs, OperationGroupID: "cli-canceled", Confirmed: true})
	if !errors.Is(err, context.Canceled) || stopped.Targets[1].GroupPhase != GroupTargetExternalNotAttempted {
		t.Fatalf("canceled group = %+v, error = %v", stopped, err)
	}
	if len(runner.commands) != 0 {
		t.Fatalf("Codex ran before cancellation: %v", runner.commands)
	}
	before, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	bindingID := domain.ComputeClientBindingID(stopped.InstallationID, string(domain.ClientCodex), string(domain.ScopeUser), stopped.Targets[1].Plan.ActivePath)
	binding := before.Installations[0].Clients[bindingID]
	if binding.Materialization != domain.MaterializationMaterialized || binding.Verification != domain.VerificationPackageValid || len(binding.Receipts) != 1 || binding.DataReceiptID == "" {
		t.Fatalf("unattempted binding lost committed package/data: %+v", binding)
	}
	data := before.Installations[0].DataReceipts[binding.DataReceiptID]
	marker := filepath.Join(data.Locator, "recovery-marker")
	if err := os.WriteFile(marker, []byte("retained"), 0600); err != nil {
		t.Fatal(err)
	}
	packageInfo, err := os.Stat(binding.TargetLocator)
	if err != nil {
		t.Fatal(err)
	}
	runner.marketplace = shared.ManagedMarketplaceName(binding.PhysicalArtifact)
	retried, err := service.AddGroup(context.Background(), GroupInput{Targets: inputs[1:], OperationGroupID: "cli-retry", Confirmed: true})
	if err != nil {
		t.Fatalf("retry Codex: %v; commands = %v", err, runner.commands)
	}
	if retried.Targets[0].Activation.Activation != domain.ActivationActive || retried.Targets[0].Activation.Verification != domain.VerificationInstalled || len(retried.Receipts) != 0 {
		t.Fatalf("retry outcome = %+v", retried)
	}
	want := [][]string{
		{"/test/bin/codex", "plugin", "marketplace", "add", binding.TargetLocator, "--json"},
		{"/test/bin/codex", "plugin", "add", "demo@" + runner.marketplace, "--json"},
		{"/test/bin/codex", "plugin", "list", "--json"},
	}
	if !reflect.DeepEqual(runner.commands, want) {
		t.Fatalf("retry commands = %v, want %v", runner.commands, want)
	}
	after, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	recovered := after.Installations[0].Clients[bindingID]
	if !reflect.DeepEqual(recovered.Receipts, binding.Receipts) || recovered.DataReceiptID != binding.DataReceiptID || !reflect.DeepEqual(after.Installations[0].DataReceipts, before.Installations[0].DataReceipts) || !reflect.DeepEqual(recovered.NativeObjects, binding.NativeObjects) {
		t.Fatalf("retry changed package/data ownership: before = %+v, after = %+v", binding, recovered)
	}
	recoveredInfo, err := os.Stat(binding.TargetLocator)
	if err != nil || !os.SameFile(packageInfo, recoveredInfo) {
		t.Fatalf("retry replaced package directory: %v", err)
	}
	if body, err := os.ReadFile(marker); err != nil || string(body) != "retained" {
		t.Fatalf("data marker = %q, %v", body, err)
	}
	if binding.Activation != domain.ActivationPrepared {
		t.Fatalf("unattempted activation = %s, want prepared", binding.Activation)
	}
	// Completed bindings stay read-only. Recognized negative listing evidence
	// remains a failure, even with explicit activation attestation.
	runner.commands = nil
	if _, err := service.AddGroup(context.Background(), GroupInput{Targets: inputs[1:], OperationGroupID: "cli-verified", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(runner.commands, want[2:]) {
		t.Fatalf("verified retry performed effects: %v", runner.commands)
	}
	runner.installed = false
	inputs[1].ActivationComplete = true
	if _, err := service.AddGroup(context.Background(), GroupInput{Targets: inputs[1:], OperationGroupID: "cli-missing", Confirmed: true}); !errors.Is(err, shared.ErrRecognizedNegativeEvidence) {
		t.Fatalf("missing plugin was accepted: %v", err)
	}
}

type groupCLIRecoveryRunner struct {
	cancel                context.CancelFunc
	marketplace           string
	registered, installed bool
	commands              [][]string
}

func (r *groupCLIRecoveryRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	if command.Argv[0] == "/test/bin/claude" {
		r.cancel()
		return legacyports.CommandResult{}, context.Canceled
	}
	if command.Argv[0] != "/test/bin/codex" {
		return legacyports.CommandResult{}, fmt.Errorf("unexpected executable: %v", command.Argv)
	}
	r.commands = append(r.commands, append([]string(nil), command.Argv...))
	switch {
	case len(command.Argv) == 6 && reflect.DeepEqual(command.Argv[1:4], []string{"plugin", "marketplace", "add"}):
		r.registered = true
	case len(command.Argv) == 5 && command.Argv[1] == "plugin" && command.Argv[2] == "add":
		if !r.registered || command.Argv[3] != "demo@"+r.marketplace {
			return legacyports.CommandResult{}, fmt.Errorf("add before exact registration: %v", command.Argv)
		}
		r.installed = true
	case reflect.DeepEqual(command.Argv[1:], []string{"plugin", "list", "--json"}):
		if !r.installed {
			return legacyports.CommandResult{Stdout: []byte(`{"installed":[]}`)}, nil
		}
		return legacyports.CommandResult{Stdout: []byte(fmt.Sprintf(`{"installed":[{"pluginId":%q,"name":"demo","marketplaceName":%q,"installed":true,"enabled":true}]}`, "demo@"+r.marketplace, r.marketplace))}, nil
	default:
		return legacyports.CommandResult{}, fmt.Errorf("unexpected command: %v", command.Argv)
	}
	return legacyports.CommandResult{}, nil
}
