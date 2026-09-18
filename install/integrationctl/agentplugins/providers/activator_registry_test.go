package providers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

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
	if _, err := bare.Activate(context.Background(), request); !errors.Is(err, errActivatorRegistryRequired) {
		t.Fatalf("Activate without a registry = %v, want errActivatorRegistryRequired", err)
	}
	if _, err := bare.Deactivate(context.Background(), domain.DeactivationRequest{Client: domain.DetectedClient{ClientID: domain.ClientCursor}}); !errors.Is(err, errActivatorRegistryRequired) {
		t.Fatalf("Deactivate without a registry = %v, want errActivatorRegistryRequired", err)
	}
	if err := bare.PreflightActivation(request); !errors.Is(err, errActivatorRegistryRequired) {
		t.Fatalf("PreflightActivation without a registry = %v, want errActivatorRegistryRequired", err)
	}
}
