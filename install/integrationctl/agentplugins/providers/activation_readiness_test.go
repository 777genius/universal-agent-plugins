package providers

import (
	"context"
	"errors"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type readinessAdapter struct {
	required bool
	err      error
	calls    int
}

func (*readinessAdapter) ID() domain.ClientID { return domain.ClientCopilot }
func (a *readinessAdapter) RequiresActivation(_ context.Context, _ clients.Env, client domain.DetectedClient, plan domain.DeliveryPlan) (bool, error) {
	a.calls++
	if client.ConfigRoot != "selected" || plan.ActivePath != "immutable" {
		return false, errors.New("selected authority was changed")
	}
	return a.required, a.err
}

type noReadinessAdapter struct{}

func (noReadinessAdapter) ID() domain.ClientID { return domain.ClientCopilot }

func TestActivationReadinessProviderDelegatesAndDefaults(t *testing.T) {
	client, plan := domain.DetectedClient{ClientID: domain.ClientCopilot, ConfigRoot: "selected"}, domain.DeliveryPlan{ActivePath: "immutable"}
	if _, err := (Activator{}).RequiresActivation(t.Context(), client, plan); !errors.Is(err, clients.ErrRegistryRequired) {
		t.Fatal(err)
	}
	registry, err := clients.NewRegistry(noReadinessAdapter{})
	if err != nil {
		t.Fatal(err)
	}
	if required, err := (Activator{Registry: registry}).RequiresActivation(t.Context(), client, plan); err != nil || required {
		t.Fatal("adapter without capability changed behavior")
	}
	cause := errors.New("readiness failure")
	adapter := &readinessAdapter{required: true, err: cause}
	registry, err = clients.NewRegistry(adapter)
	if err != nil {
		t.Fatal(err)
	}
	a := Activator{Registry: registry}
	if required, err := a.RequiresActivation(t.Context(), client, plan); !required || !errors.Is(err, cause) || adapter.calls != 1 {
		t.Fatalf("delegation %v %v", required, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := a.RequiresActivation(ctx, client, plan); !errors.Is(err, context.Canceled) || adapter.calls != 1 {
		t.Fatal("canceled inspection called adapter")
	}
}
