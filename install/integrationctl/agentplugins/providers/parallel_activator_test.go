package providers

import (
	"context"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func parallelRequest(client domain.ClientID, scope domain.InstallScope) domain.ActivationRequest {
	return domain.ActivationRequest{
		Client: domain.DetectedClient{ClientID: client, Status: domain.DetectionDetected},
		Plan:   domain.DeliveryPlan{ClientID: client, Scope: scope},
	}
}

// Only the adapters that attested effect isolation may be offered for concurrent
// activation, and only when the composition set Concurrent.
func TestParallelActivatorOffersOnlyAttestedClients(t *testing.T) {
	activator := testActivator(Activator{Concurrent: true})
	for _, client := range []domain.ClientID{domain.ClientClaude, domain.ClientCodex} {
		operation, err := activator.PrepareIndependentActivation(context.Background(), parallelRequest(client, domain.ScopeUser))
		if err != nil || operation == nil {
			t.Fatalf("%s user activation must be independent: %v", client, err)
		}
	}
	for name, request := range map[string]domain.ActivationRequest{
		"project scope keeps serial custody": parallelRequest(domain.ClientCodex, domain.ScopeProject),
		"unattested client":                  parallelRequest(domain.ClientCursor, domain.ScopeUser),
	} {
		if operation, err := activator.PrepareIndependentActivation(context.Background(), request); err == nil || operation != nil {
			t.Fatalf("%s: must be refused, got operation=%v err=%v", name, operation != nil, err)
		}
	}
}

func TestActivatorStaysSerialUnlessTheCompositionOptsIn(t *testing.T) {
	serial := testActivator(Activator{})
	for _, client := range []domain.ClientID{domain.ClientClaude, domain.ClientCodex} {
		request := parallelRequest(client, domain.ScopeUser)
		if serial.SupportsIndependentActivation(request) {
			t.Fatalf("%s must stay serial by default", client)
		}
		if operation, err := serial.PrepareIndependentActivation(context.Background(), request); err == nil || operation != nil {
			t.Fatal("a serial activator must refuse to prepare an independent effect")
		}
	}
}
