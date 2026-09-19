package providers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestActivatorRequiresInjectedRegistry(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := filepath.Join(root, "owned")
	active := filepath.Join(base, "demo")
	if err := os.MkdirAll(active, 0o755); err != nil {
		t.Fatal(err)
	}
	request := domain.ActivationRequest{
		Client:       domain.DetectedClient{ClientID: domain.ClientCursor},
		DeclaredName: "demo",
		Plan:         domain.DeliveryPlan{ClientID: domain.ClientCursor, ActivePath: active},
		Delivery:     domain.StagedDelivery{ClientID: domain.ClientCursor, OwnedBase: base, ActivePath: active},
	}
	bare := Activator{}
	if _, err := bare.Activate(context.Background(), request); !errors.Is(err, clients.ErrRegistryRequired) {
		t.Fatalf("Activate without a registry = %v, want ErrRegistryRequired", err)
	}
	if _, err := bare.Deactivate(context.Background(), domain.DeactivationRequest{Client: domain.DetectedClient{ClientID: domain.ClientCursor}}); !errors.Is(err, clients.ErrRegistryRequired) {
		t.Fatalf("Deactivate without a registry = %v, want ErrRegistryRequired", err)
	}
	if err := bare.PreflightActivation(request); !errors.Is(err, clients.ErrRegistryRequired) {
		t.Fatalf("PreflightActivation without a registry = %v, want ErrRegistryRequired", err)
	}

	// Leftover 7c clients would otherwise claim automatic activation and a
	// native verifier; nil registry must still fail closed on the predicates.
	automatic := request
	automatic.Client = domain.DetectedClient{ClientID: domain.ClientCline, ConfigRoot: filepath.Join(root, "cline")}
	automatic.Plan.ClientID = domain.ClientCline
	automatic.Plan.InstallIntent = domain.InstallIntentAutomatic
	automatic.Plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentSkill, Support: domain.SupportNative}}
	automatic.Delivery.ClientID = domain.ClientCline
	if bare.AutomaticallyActivates(automatic) {
		t.Fatal("AutomaticallyActivates without a registry claimed a leftover native client")
	}
	if bare.VerifierAvailable(domain.DetectedClient{ClientID: domain.ClientGemini}, domain.DeliveryPlan{Components: automatic.Plan.Components}, "/bin/gemini") {
		t.Fatal("VerifierAvailable without a registry claimed a leftover native client")
	}
}
