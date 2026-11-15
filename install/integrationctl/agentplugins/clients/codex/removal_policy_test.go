package codex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestRemovalAttestationPolicyMatchesDeactivateIncludingNeverActivated(t *testing.T) {
	adapter := New()
	for _, activation := range []domain.ActivationState{domain.ActivationPrepared, domain.ActivationManual, domain.ActivationActive} {
		t.Run(string(activation), func(t *testing.T) {
			request := domain.DeactivationRequest{
				Client:            domain.DetectedClient{ClientID: domain.ClientCodex, ConfigRoot: tempRoot(t)},
				CurrentActivation: activation, DeclaredName: "demo", PhysicalArtifactID: "artifact", Confirmed: true,
				ManagedArtifactPath: tempRoot(t),
			}
			if !adapter.RequiresExternalUninstalled(clients.Env{}, request) {
				t.Fatal("removal policy lost the explicit confirmation requirement")
			}
			outcome, err := adapter.Deactivate(t.Context(), clients.Env{}, request)
			if err != nil || outcome.ArtifactRemovalAllowed || outcome.ExternalRemovalComplete {
				t.Fatal("deactivation diverged from preflight policy", outcome, err)
			}
			request.ExternalUninstalled = true
			outcome, err = adapter.Deactivate(t.Context(), clients.Env{}, request)
			if err != nil || !outcome.ArtifactRemovalAllowed || !outcome.ExternalRemovalComplete {
				t.Fatal("explicit confirmation could not settle an absent native registration", outcome, err)
			}
		})
	}
}

type removalPolicyRunner struct{ commands []legacyports.Command }

func (runner *removalPolicyRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.commands = append(runner.commands, command)
	return legacyports.CommandResult{Stdout: []byte(`{"installed":[]}`)}, nil
}
func TestCodexNativeRemovalNeedsNoAttestationAndConfirmsAbsence(t *testing.T) {
	for _, staleEntry := range []bool{false, true} {
		t.Run(fmt.Sprint(staleEntry), func(t *testing.T) {
			request := domain.DeactivationRequest{Client: domain.DetectedClient{ClientID: domain.ClientCodex, ConfigRoot: tempRoot(t)}, DeclaredName: "demo", PhysicalArtifactID: "artifact", ManagedArtifactPath: tempRoot(t), BackendExecutable: "/installed/codex", Confirmed: true}
			if staleEntry {
				body := "[plugins.\"demo@" + shared.ManagedMarketplaceName("artifact") + "\"]\nenabled = true\n"
				if err := os.WriteFile(filepath.Join(request.Client.ConfigRoot, "config.toml"), []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			runner := &removalPolicyRunner{}
			env := clients.Env{Runner: runner}
			if New().RequiresExternalUninstalled(env, request) {
				t.Fatal("native CLI cleanup required a manual attestation")
			}
			outcome, err := New().Deactivate(t.Context(), env, request)
			if staleEntry {
				if err == nil || outcome.ArtifactRemovalAllowed || outcome.ExternalRemovalComplete {
					t.Fatalf("stale native enablement accepted: %+v %v", outcome, err)
				}
			} else if err != nil || !outcome.ArtifactRemovalAllowed || !outcome.ExternalRemovalComplete {
				t.Fatalf("native removal did not complete: %+v %v", outcome, err)
			}
			if len(runner.commands) != 2 || runner.commands[0].Argv[2] != "remove" || runner.commands[1].Argv[2] != "list" {
				t.Fatalf("commands=%+v", runner.commands)
			}
		})
	}
}
