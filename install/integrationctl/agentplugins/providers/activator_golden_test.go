package providers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/internal/goldentest"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type activationRecord struct {
	Outcome  domain.ActivationOutcome
	Error    string
	Commands [][]string
}

type deactivationRecord struct {
	Outcome  domain.DeactivationOutcome
	Error    string
	Commands [][]string
}

// TestActivateGoldenAcrossClients freezes the lifecycle of all clients,
// including which commands each one runs, before the switch in activator.go is
// replaced by a registry lookup. The runner here answers every command with an
// empty result, which is the silent-client path: Claude and Kiro end in their
// unknown-evidence error, the others in their normal outcome.
// TestActivateGoldenWhenClientsRespond covers the happy path for every client.
func TestActivateGoldenAcrossClients(t *testing.T) {
	t.Parallel()
	for _, verifyOnly := range []bool{false, true} {
		name := "activate"
		if verifyOnly {
			// VerifyOnly forbids client mutation, so no mutating argv may appear.
			name = "activate_verify_only"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			records := make(map[string]activationRecord, len(domain.ClientDefinitions()))
			for _, definition := range domain.ClientDefinitions() {
				runner := &recordingRunner{}
				request := goldenActivationRequest(t, definition.ID, root)
				request.VerifyOnly = verifyOnly
				outcome, err := testActivator(Activator{Runner: runner}).Activate(context.Background(), request)
				records[string(definition.ID)] = activationRecord{
					Outcome: outcome, Error: errorText(err), Commands: commandArgv(runner.commands),
				}
			}
			goldenActivator(root).Assert(t, name, records)
		})
	}
}

// TestActivateGoldenWhenClientsRespond is the happy path: every client's CLI
// answers as it does when the plugin really is installed. Part 7 moves this
// dispatch into per-client adapters, so both the responding and the silent side
// have to stay identical for all clients.
func TestActivateGoldenWhenClientsRespond(t *testing.T) {
	t.Parallel()
	for _, verifyOnly := range []bool{false, true} {
		name := "activate_client_responds"
		if verifyOnly {
			name = "activate_client_responds_verify_only"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			records := make(map[string]activationRecord, len(domain.ClientDefinitions()))
			for _, definition := range domain.ClientDefinitions() {
				request := goldenActivationRequest(t, definition.ID, root)
				request.VerifyOnly = verifyOnly
				runner := goldenRespondingRunner(definition.ID, request)
				outcome, err := testActivator(Activator{Runner: runner}).Activate(context.Background(), request)
				records[string(definition.ID)] = activationRecord{
					Outcome: outcome, Error: errorText(err), Commands: commandArgv(runner.commands),
				}
			}
			goldenActivator(root).Assert(t, name, records)
		})
	}
}

// TestDeactivateGoldenAcrossClients pins the unconfirmed preview alongside the
// confirmed removal: a preview must never run a mutating command.
func TestDeactivateGoldenAcrossClients(t *testing.T) {
	t.Parallel()
	for _, confirmed := range []bool{false, true} {
		name := "deactivate_unconfirmed"
		if confirmed {
			name = "deactivate_confirmed"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			records := make(map[string]deactivationRecord, len(domain.ClientDefinitions()))
			for _, definition := range domain.ClientDefinitions() {
				runner := &recordingRunner{}
				request := goldenDeactivationRequest(t, definition.ID, root)
				request.Confirmed = confirmed
				outcome, err := testActivator(Activator{Runner: runner}).Deactivate(context.Background(), request)
				records[string(definition.ID)] = deactivationRecord{
					Outcome: outcome, Error: errorText(err), Commands: commandArgv(runner.commands),
				}
			}
			goldenActivator(root).Assert(t, name, records)
		})
	}
}

// TestDeactivateGoldenWhenOwnershipIsRecorded is the removal happy path: the
// request carries the native objects a real installation recorded, so Cline
// reaches its removal instead of stopping at the missing-ownership guard.
// Not parallel: the Cline settings location is read from the environment.
func TestDeactivateGoldenWhenOwnershipIsRecorded(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLINE_MCP_SETTINGS_PATH", filepath.Join(root, "cline", "cline_mcp_settings.json"))
	records := make(map[string]deactivationRecord, len(domain.ClientDefinitions()))
	for _, definition := range domain.ClientDefinitions() {
		request := goldenDeactivationRequest(t, definition.ID, root)
		request.Confirmed = true
		request.NativeObjects = goldenNativeObjects(t, definition.ID, root)
		runner := goldenRespondingRunner(definition.ID, domain.ActivationRequest{
			DeclaredName: request.DeclaredName,
			Delivery:     domain.StagedDelivery{ActivePath: request.ManagedArtifactPath},
		})
		outcome, err := testActivator(Activator{Runner: runner}).Deactivate(context.Background(), request)
		records[string(definition.ID)] = deactivationRecord{
			Outcome: outcome, Error: errorText(err), Commands: commandArgv(runner.commands),
		}
	}
	goldenActivator(root).Assert(t, "deactivate_recorded_ownership", records)
}

// goldenRespondingRunner primes the fake runner with what each client's CLI
// prints once the plugin is installed. The eight clients not named here already
// succeed against the default runner.
func goldenRespondingRunner(id domain.ClientID, request domain.ActivationRequest) *recordingRunner {
	switch id {
	case domain.ClientClaude:
		listing := claudeListing(request.DeclaredName, request.Delivery.ActivePath, true)
		return &recordingRunner{run: func(legacyports.Command) legacyports.CommandResult {
			return legacyports.CommandResult{Stdout: []byte(listing)}
		}}
	case domain.ClientKiro:
		// The plan declares one MCP server and the ACP session reports it
		// connected with a tool. duplexLive keeps the pipe open past the last
		// record; a plain reader hits EOF first and Kiro reads that as the
		// agent exiting before verification finished.
		return &recordingRunner{duplexOutput: connectedACP("remote"), duplexLive: true}
	case domain.ClientGrok:
		listing := fmt.Sprintf(`[{"name":%q,"status":"installed","source":%q,"version":%q}]`, request.DeclaredName, request.Delivery.ActivePath, "1.0.0")
		return &recordingRunner{run: func(legacyports.Command) legacyports.CommandResult {
			return legacyports.CommandResult{Stdout: []byte(listing)}
		}}
	default:
		return &recordingRunner{}
	}
}

// goldenNativeObjects reproduces the ownership a completed installation would
// have recorded for the clients whose removal is driven by it.
func goldenNativeObjects(t *testing.T, id domain.ClientID, root string) []domain.NativeObjectOwnership {
	t.Helper()
	if id != domain.ClientCline {
		return nil
	}
	configRoot := filepath.Join(root, string(id), "config")
	active := filepath.Join(root, "managed", "clients", string(id), "demo-0123456789ab")
	writeTestFile(t, filepath.Join(active, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: Docs\n---\n")
	server := nativeconfig.Server{Type: "stdio", Command: "node", Args: []string{filepath.Join(active, "server.js")}}
	writeClineProjectionFixture(t, active, map[string]nativeconfig.Server{"remote": server})
	return clineFixtureObjects(t, configRoot, active, "docs", "remote", server)
}

// goldenActivationRequest reproduces the layout planner.targetRoot computes for
// this client, so the activator sees the same invariants it sees in production.
func goldenActivationRequest(t *testing.T, id domain.ClientID, root string) domain.ActivationRequest {
	t.Helper()
	configRoot := filepath.Join(root, string(id), "config")
	anchor, targetRoot := goldenTargetRoot(id, root, configRoot)
	activePath := filepath.Join(targetRoot, "demo-0123456789ab")
	if err := os.MkdirAll(activePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(configRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if id == domain.ClientKimi {
		writeTestFile(t, filepath.Join(activePath, ".kimi-plugin", "plugin.json"), `{"name":"demo","skills":"./skills/"}`)
	}
	components := []domain.ComponentDecision{
		{Kind: domain.ComponentSkill, Name: "docs", Support: domain.SupportNative},
		{Kind: domain.ComponentMCPServer, Name: "remote", Support: domain.SupportNative},
	}
	return domain.ActivationRequest{
		Client: domain.DetectedClient{
			ClientID: id, DisplayName: string(id), Status: domain.DetectionDetected,
			ConfigRoot: configRoot, ExecutablePath: filepath.Join(root, "bin", string(id)),
		},
		DeclaredName:      "demo",
		BackendExecutable: filepath.Join(root, "bin", string(id)),
		Plan: domain.DeliveryPlan{
			ClientID: id, Scope: domain.ScopeUser, PhysicalArtifactID: "demo-0123456789ab",
			DeclaredName: "demo", DeclaredVersion: "1.0.0",
			Authentication: domain.AuthenticationNotChecked, Policy: domain.PolicyAllowed,
			Components:   components,
			TargetAnchor: anchor, TargetRoot: targetRoot, ActivePath: activePath,
			NativeRegistryRoot: configRoot, NativeRegistryExecutable: filepath.Join(root, "bin", string(id)),
		},
		Delivery: domain.StagedDelivery{
			ClientID: id, OwnedBase: targetRoot, ActivePath: activePath,
		},
	}
}

func goldenDeactivationRequest(t *testing.T, id domain.ClientID, root string) domain.DeactivationRequest {
	t.Helper()
	request := goldenActivationRequest(t, id, root)
	return domain.DeactivationRequest{
		Client:              request.Client,
		DeclaredName:        request.DeclaredName,
		CurrentActivation:   domain.ActivationActive,
		PhysicalArtifactID:  request.Plan.PhysicalArtifactID,
		BackendExecutable:   request.BackendExecutable,
		ManagedArtifactPath: request.Delivery.ActivePath,
	}
}

// goldenTargetRoot mirrors planner.targetRoot: Claude and Cursor deliver into
// their own config root, everything else into the managed root.
func goldenTargetRoot(id domain.ClientID, root, configRoot string) (string, string) {
	switch id {
	case domain.ClientClaude:
		return configRoot, filepath.Join(configRoot, "skills")
	case domain.ClientCursor:
		return configRoot, filepath.Join(configRoot, "plugins", "local")
	case domain.ClientGrok:
		return configRoot, filepath.Join(configRoot, "plugins")
	case domain.ClientKimi:
		return configRoot, filepath.Join(configRoot, "plugins", "managed")
	default:
		managed := filepath.Join(root, "managed")
		return managed, filepath.Join(managed, "clients", string(id))
	}
}

func goldenActivator(root string) goldentest.Golden {
	return goldentest.Golden{Replace: []goldentest.Replacement{{From: root, To: "<root>"}}}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
