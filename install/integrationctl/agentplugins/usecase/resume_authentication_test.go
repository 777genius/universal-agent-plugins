package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestResumeNormalizesOnlyUnknownAuthenticationWithFreshNativeProof(t *testing.T) {
	for _, scenario := range []string{"observed", "error", "unverified", "blocked", "activation-attested", "authentication-attested", "no-verifier", "pending", "failed", "complete", "already-not-required"} {
		t.Run(scenario, func(t *testing.T) {
			service, store, client := serviceFixture(t)
			input := addInput(t, client, "https://example.com/resume-auth")
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
			binding.Activation, binding.Verification, binding.Policy = domain.ActivationActive, domain.VerificationInstalled, domain.PolicyAllowed
			binding.Authentication = domain.AuthenticationNotChecked
			switch scenario {
			case "pending":
				binding.Authentication = domain.AuthenticationPending
			case "failed":
				binding.Authentication = domain.AuthenticationFailed
			case "complete":
				binding.Authentication = domain.AuthenticationComplete
			case "already-not-required":
				binding.Authentication = domain.AuthenticationNotRequired
			}
			state.Installations[0].Clients[binding.ClientBindingID] = binding
			if err := store.Save(state); err != nil {
				t.Fatal(err)
			}
			outcome := lifecycleOutcome(binding)
			outcome.Authentication = domain.AuthenticationNotRequired
			var observationErr error
			switch scenario {
			case "error":
				observationErr = errors.New("native initialization unavailable")
			case "unverified":
				outcome.Verification = domain.VerificationNotRun
			case "blocked":
				outcome.Policy = domain.PolicyBlocked
			case "activation-attested":
				outcome.ActivationAttested = true
			case "authentication-attested":
				outcome.AuthenticationAttested = true
			}
			activator := &resumeObservationActivator{observedActivator: observedActivator{outcome: outcome, err: observationErr}, available: scenario != "no-verifier"}
			service.Activator = activator
			result, err := service.resume(context.Background(), input, AddResult{Plan: installed.Plan}, installed.InstallationID, binding.ClientBindingID, binding)
			if !errors.Is(err, observationErr) {
				t.Fatal(err)
			}
			want := binding.Authentication
			if scenario == "observed" {
				want = domain.AuthenticationNotRequired
			}
			if result.Activation.Authentication != want {
				t.Fatalf("native observation changed authentication incorrectly: %+v", result.Activation)
			}
			after, loadErr := store.Load()
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			if got := onlyBinding(after.Installations[0]).Authentication; got != want {
				t.Fatalf("persisted authentication %q, want %q", got, want)
			}
			if len(activator.requests) != 1 || !activator.requests[0].VerifyOnly {
				t.Fatalf("resume did not use read-only native observation: %+v", activator.requests)
			}
		})
	}
}
