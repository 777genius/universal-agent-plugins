package usecase

import (
	"context"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// TestReadOnlyCodexNoChangeRetainsDesktopHandoff checks the actual use-case result, not just its renderer.
func TestReadOnlyCodexNoChangeRetainsDesktopHandoff(t *testing.T) {
	binding := domain.ClientBinding{
		Activation:     domain.ActivationActive,
		Authentication: domain.AuthenticationNotRequired,
		Policy:         domain.PolicyAllowed,
		Verification:   domain.VerificationInstalled,
	}
	verified := lifecycleOutcome(binding)
	verified.LocalActions = []string{
		"start a new Codex session; ChatGPT Desktop (optional, local): codex://plugins/demo?marketplacePath=%2Fprivate%2Fmarketplace.json",
	}
	activator := &resumeObservationActivator{
		observedActivator: observedActivator{outcome: verified},
		available:         true,
	}
	session := applySession{
		service: Service{Activator: activator},
		ctx:     context.Background(),
		input: AddInput{
			Client:            domain.DetectedClient{ClientID: domain.ClientCodex},
			Scope:             domain.ScopeUser,
			BackendExecutable: "codex",
			Envelope:          domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "demo"}},
		},
		result: AddResult{
			Plan:       domain.DeliveryPlan{ClientID: domain.ClientCodex, Scope: domain.ScopeUser},
			Activation: lifecycleOutcome(binding),
		},
	}

	done, result, err := session.persistReadOnlyObservation(binding)
	if err != nil || !done || !result.NoChange || result.Mutated {
		t.Fatalf("read-only no-change result = %+v, done = %v, err = %v", result, done, err)
	}
	if !reflect.DeepEqual(result.Activation.LocalActions, verified.LocalActions) {
		t.Fatalf("verified local guidance dropped: %+v", result.Activation)
	}
	if !sameLifecycleOutcome(result.Activation, lifecycleOutcome(binding)) {
		t.Fatalf("local guidance changed the lifecycle status: %+v", result.Activation)
	}
	if len(activator.requests) != 1 || !activator.requests[0].VerifyOnly {
		t.Fatalf("expected one read-only native verification, got %+v", activator.requests)
	}
}
