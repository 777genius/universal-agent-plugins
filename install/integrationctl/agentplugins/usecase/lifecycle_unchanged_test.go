package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type resumeObservationActivator struct {
	observedActivator
	available bool
	requests  []domain.ActivationRequest
}

func (a *resumeObservationActivator) VerifierAvailable(domain.DetectedClient, domain.DeliveryPlan, string) bool {
	return a.available
}

func (a *resumeObservationActivator) Activate(ctx context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	a.requests = append(a.requests, request)
	return a.observedActivator.Activate(ctx, request)
}

func TestResumeUncheckedAuthenticationNoChangeRequiresVerifiedObservation(t *testing.T) {
	for _, tc := range []struct {
		name           string
		auth           domain.AuthenticationState
		plannedAuth    domain.AuthenticationState
		available      bool
		action         string
		observationErr error
		wantNoChange   bool
	}{
		{name: "verified unchecked", auth: domain.AuthenticationNotChecked, plannedAuth: domain.AuthenticationNotChecked, available: true, wantNoChange: true},
		{name: "no auth action required", auth: domain.AuthenticationNotChecked, plannedAuth: domain.AuthenticationNotRequired, available: true, wantNoChange: true},
		{name: "no verifier", auth: domain.AuthenticationNotChecked, plannedAuth: domain.AuthenticationNotChecked},
		{name: "pending auth", auth: domain.AuthenticationPending, plannedAuth: domain.AuthenticationPending, available: true},
		{name: "failed auth", auth: domain.AuthenticationFailed, plannedAuth: domain.AuthenticationPending, available: true},
		{name: "new auth requirement", auth: domain.AuthenticationNotChecked, plannedAuth: domain.AuthenticationPending, available: true},
		{name: "outstanding action", auth: domain.AuthenticationNotChecked, plannedAuth: domain.AuthenticationNotChecked, available: true, action: "authenticate"},
		{name: "unknown observation", auth: domain.AuthenticationNotChecked, plannedAuth: domain.AuthenticationNotChecked, available: true, observationErr: errors.New("client observation unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, store, client := serviceFixture(t)
			input := addInput(t, client, "https://example.com/unchanged-auth")
			input.Confirmed = true
			installed, err := service.Add(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			state, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			binding := onlyBinding(state.Installations[0])
			binding.Activation = domain.ActivationActive
			binding.Verification = domain.VerificationInstalled
			binding.Authentication = tc.auth
			state.Installations[0].Clients[binding.ClientBindingID] = binding
			if err := store.Save(state); err != nil {
				t.Fatal(err)
			}
			outcome := lifecycleOutcome(binding)
			// Client registration success must not attest authentication.
			outcome.Authentication = domain.AuthenticationNotChecked
			if tc.action != "" {
				outcome.UserActions = []string{tc.action}
			}
			activator := &resumeObservationActivator{
				observedActivator: observedActivator{outcome: outcome, err: tc.observationErr},
				available:         tc.available,
			}
			service.Activator = activator
			plan := installed.Plan
			plan.Authentication = tc.plannedAuth
			result, err := service.resume(context.Background(), input, AddResult{Plan: plan}, installed.InstallationID, binding.ClientBindingID, binding)
			if !errors.Is(err, tc.observationErr) || result.NoChange != tc.wantNoChange || result.Mutated {
				t.Fatalf("resume: %+v %v", result, err)
			}
			if len(activator.requests) != 1 || !activator.requests[0].VerifyOnly {
				t.Fatalf("resume requests: %+v", activator.requests)
			}
			if result.Activation.Authentication != tc.auth || result.Activation.AuthenticationAttested {
				t.Fatalf("authentication escalated: %+v", result.Activation)
			}
			after, err := store.Load()
			if err != nil || !reflect.DeepEqual(state, after) {
				t.Fatalf("unchanged observation wrote state: %v", err)
			}
		})
	}
}
