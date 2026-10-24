package codex

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
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
			if !adapter.RequiresExternalUninstalled(request) {
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
