package usecase

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type prepareNoDuplexRunner struct{ calls int }

func (r *prepareNoDuplexRunner) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	r.calls++
	return legacyports.CommandResult{}, nil
}

func kiroPrepareInput(t *testing.T, client domain.DetectedClient) AddInput {
	t.Helper()
	input := addInput(t, client, "https://example.test/kiro-prepare")
	const server = `{"type":"streamable-http","url":"https://mcp.context7.com/mcp/oauth"}`
	body := []byte(`{"mcpServers":{"docs":` + server + `}}`)
	if err := os.WriteFile(filepath.Join(input.Envelope.SnapshotRoot, "mcp.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	input.Envelope.MCP = domain.MCPComponent{Present: true, Enabled: true, Raw: body, Servers: map[string]domain.MCPServer{"docs": {Name: "docs", Type: "streamable-http", Raw: []byte(server), Decoded: map[string]any{"type": "streamable-http", "url": "https://mcp.context7.com/mcp/oauth"}}}}
	input.Envelope.Inventory.MCPPresent = true
	input.Envelope.Inventory.MCPEnabled = true
	input.Envelope.Inventory.MCPServers = []string{"docs"}
	input.BackendExecutable = "/fixture/kiro-cli"
	input.Confirmed = true
	return input
}

func TestKiroExplicitPreparationLifecycleWithoutDuplex(t *testing.T) {
	for _, purge := range []bool{false, true} {
		t.Run(map[bool]string{false: "retain data", true: "purge data"}[purge], func(t *testing.T) {
			service, store, _ := serviceFixture(t)
			runner := &prepareNoDuplexRunner{}
			service.Activator = providers.Activator{Runner: runner}
			service.NativeObserver = providers.NativeIdentityObserver{Stager: service.Stager, Runner: runner}
			client := domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
			input := kiroPrepareInput(t, client)
			if _, err := service.Add(context.Background(), input); err == nil || !strings.Contains(err.Error(), "duplex") {
				t.Fatalf("automatic preflight: %v", err)
			}
			state, err := store.Load()
			if err != nil || len(state.Installations) != 0 {
				t.Fatalf("automatic mutated state: %+v %v", state, err)
			}
			if _, err := os.Stat(client.ConfigRoot); !os.IsNotExist(err) {
				t.Fatalf("automatic wrote config: %v", err)
			}
			config := filepath.Join(client.ConfigRoot, "settings", "mcp.json")
			if err := os.MkdirAll(filepath.Dir(config), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(config, []byte(`{"mcpServers":{"foreign":{"url":"https://foreign.test/mcp"}},"custom":42}`), 0600); err != nil {
				t.Fatal(err)
			}
			input.InstallIntent = domain.InstallIntentPrepare
			check := func(result AddResult, err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
				if result.Plan.InstallIntent != domain.InstallIntentPrepare || result.Activation.Activation != domain.ActivationPrepared || result.Activation.Verification != domain.VerificationPackageValid || result.Activation.ActivationAttested {
					t.Fatalf("false lifecycle claim: %+v", result)
				}
				state, err := store.Load()
				if err != nil {
					t.Fatal(err)
				}
				if onlyBinding(state.Installations[0]).InstallIntent != domain.InstallIntentPrepare {
					t.Fatal("lost persisted preparation")
				}
				body, err := os.ReadFile(config)
				if err != nil {
					t.Fatal(err)
				}
				var doc map[string]any
				if err := json.Unmarshal(body, &doc); err != nil {
					t.Fatal(err)
				}
				servers := doc["mcpServers"].(map[string]any)
				if len(servers) != 2 || servers["foreign"].(map[string]any)["url"] != "https://foreign.test/mcp" || servers["docs"].(map[string]any)["url"] != "https://mcp.context7.com/mcp/oauth" || doc["custom"] != float64(42) {
					t.Fatalf("config damaged: %s", body)
				}
				if runner.calls != 0 {
					t.Fatalf("executed %d client commands", runner.calls)
				}
			}
			check(service.Add(context.Background(), input))
			input.InstallIntent = ""
			input.ActivationComplete = true
			check(service.Add(context.Background(), input))
			input.ActivationComplete = false
			setEnvelopeVersion(t, &input.Envelope, "2.0.0", "sha256:prepare-v2", "sha256:prepare-manifest-v2")
			input.OperationID = "prepare-update"
			check(service.Update(context.Background(), input))
			input.OperationID = "prepare-repair"
			check(service.Repair(context.Background(), input))
			state, _ = store.Load()
			if err := os.RemoveAll(onlyBinding(state.Installations[0]).TargetLocator); err != nil {
				t.Fatal(err)
			}
			input.OperationID = "prepare-rematerialize"
			check(service.Repair(context.Background(), input))
			removed, err := service.RemoveGroup(context.Background(), RemoveGroupInput{Selector: input.InstallationID, Targets: []RemoveInput{{Client: client, Scope: domain.ScopeUser}}, OperationGroupID: "prepare-remove", Confirmed: true, PurgeData: purge})
			if err != nil || !removed.Mutated {
				t.Fatalf("remove: %+v %v", removed, err)
			}
			body, _ := os.ReadFile(config)
			if strings.Contains(string(body), `"docs"`) || !strings.Contains(string(body), `"foreign"`) {
				t.Fatalf("remove damaged foreign config: %s", body)
			}
			input.OperationID = "prepare-reinstall"
			check(service.Add(context.Background(), input))
			before, _ := os.ReadFile(config)
			changed := strings.ReplaceAll(string(before), "https://mcp.context7.com/mcp/oauth", "https://changed.test/mcp")
			if err := os.WriteFile(config, []byte(changed), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := service.Add(context.Background(), input); err == nil {
				t.Fatal("repeat accepted modified owned entry")
			}
			if _, err := service.Repair(context.Background(), input); err == nil {
				t.Fatal("repair accepted modified owned entry")
			}
			after, _ := os.ReadFile(config)
			if string(after) != changed {
				t.Fatal("overwrote modified entry")
			}
		})
	}
}

func TestPreparedKiroGroupedMaintenanceRetainsPerTargetIntent(t *testing.T) {
	service, store, cursor := serviceFixture(t)
	runner := &prepareNoDuplexRunner{}
	service.Activator = providers.Activator{Runner: runner}
	service.NativeObserver = providers.NativeIdentityObserver{Stager: service.Stager, Runner: runner}
	kiro := domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
	input := kiroPrepareInput(t, kiro)
	input.InstallIntent = domain.InstallIntentPrepare
	peer := input
	peer.Client = cursor
	peer.InstallIntent = ""
	peer.BackendExecutable = ""
	check := func(result GroupResult, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		for _, target := range result.Targets {
			if target.Plan.ClientID == domain.ClientKiro && (target.Plan.InstallIntent != domain.InstallIntentPrepare || target.Activation.Activation != domain.ActivationPrepared || target.Activation.Verification != domain.VerificationPackageValid) {
				t.Fatalf("group lost intent: %+v", target)
			}
		}
		state, err := store.Load()
		if err != nil {
			t.Fatal(err)
		}
		for _, binding := range state.Installations[0].Clients {
			if binding.ClientID == string(domain.ClientKiro) {
				if binding.InstallIntent != domain.InstallIntentPrepare {
					t.Fatal("lost persisted intent")
				}
			} else if binding.InstallIntent != "" {
				t.Fatal("changed other target mode")
			}
		}
		if runner.calls != 0 {
			t.Fatal("group launched client")
		}
	}
	check(service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{input, peer}, OperationGroupID: "prepare-group-add", Confirmed: true}))
	input.InstallIntent = ""
	setEnvelopeVersion(t, &input.Envelope, "2.0.0", "sha256:group-prepare-v2", "sha256:group-prepare-manifest-v2")
	peer.Envelope = input.Envelope
	check(service.UpdateGroup(context.Background(), GroupInput{Targets: []AddInput{input, peer}, CompatibilityChecks: []AddInput{input, peer}, OperationGroupID: "prepare-group-update", Confirmed: true}))
	check(service.RepairGroup(context.Background(), GroupInput{Targets: []AddInput{input, peer}, OperationGroupID: "prepare-group-repair", Confirmed: true}))
}

func TestPreparationRejectsForeignNameAndInvalidIntentBeforeMutation(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		t.Run(map[bool]string{false: "foreign name", true: "invalid intent"}[invalid], func(t *testing.T) {
			service, store, _ := serviceFixture(t)
			runner := &prepareNoDuplexRunner{}
			service.Activator = providers.Activator{Runner: runner}
			service.NativeObserver = providers.NativeIdentityObserver{Stager: service.Stager, Runner: runner}
			client := domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
			input := kiroPrepareInput(t, client)
			input.InstallIntent = domain.InstallIntentPrepare
			if invalid {
				input.InstallIntent = "typo"
			}
			config := filepath.Join(client.ConfigRoot, "settings", "mcp.json")
			if err := os.MkdirAll(filepath.Dir(config), 0700); err != nil {
				t.Fatal(err)
			}
			const foreign = `{"mcpServers":{"docs":{"url":"https://foreign.test/mcp"}}}`
			if err := os.WriteFile(config, []byte(foreign), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := service.Add(context.Background(), input); err == nil {
				t.Fatal("accepted foreign ownership or invalid intent")
			}
			state, err := store.Load()
			if err != nil || len(state.Installations) != 0 {
				t.Fatalf("mutated state %+v %v", state, err)
			}
			body, _ := os.ReadFile(config)
			if string(body) != foreign || runner.calls != 0 {
				t.Fatal("changed foreign configuration or launched client")
			}
		})
	}
}
