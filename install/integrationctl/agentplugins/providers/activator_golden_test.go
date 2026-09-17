package providers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/internal/goldentest"
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

// TestActivateGoldenAcrossClients freezes the lifecycle of all eleven clients,
// including which commands each one runs, before the switch in activator.go is
// replaced by a registry lookup.
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
				outcome, err := (Activator{Runner: runner}).Activate(context.Background(), request)
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
				outcome, err := (Activator{Runner: runner}).Deactivate(context.Background(), request)
				records[string(definition.ID)] = deactivationRecord{
					Outcome: outcome, Error: errorText(err), Commands: commandArgv(runner.commands),
				}
			}
			goldenActivator(root).Assert(t, name, records)
		})
	}
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
