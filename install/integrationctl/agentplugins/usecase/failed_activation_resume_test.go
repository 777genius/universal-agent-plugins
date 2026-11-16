package usecase

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type failedActivationResumeActor struct {
	acts, reads  int
	initialError error
}

func (*failedActivationResumeActor) VerifierAvailable(domain.DetectedClient, domain.DeliveryPlan, string) bool {
	return true
}
func (*failedActivationResumeActor) PreparedSetupVerifierAvailable(domain.DetectedClient, domain.DeliveryPlan, string) bool {
	return true
}
func (*failedActivationResumeActor) AutomaticallyActivates(domain.ActivationRequest) bool {
	return true
}
func (*failedActivationResumeActor) CanCompletePreparedSetup(domain.ActivationRequest, domain.ActivationOutcome) bool {
	return true
}
func (a *failedActivationResumeActor) Activate(_ context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if request.VerifyOnly {
		a.reads++
		return domain.ActivationOutcome{Activation: domain.ActivationFailed, Verification: domain.VerificationFailed, Policy: domain.PolicyAllowed, NativeEffect: domain.NativeEffectUnchanged}, fmt.Errorf("%w: exact plugin remains absent", shared.ErrRecognizedNegativeEvidence)
	}
	a.acts++
	if a.acts == 1 {
		return domain.ActivationOutcome{Activation: domain.ActivationFailed, Verification: domain.VerificationFailed, Authentication: domain.AuthenticationNotChecked, Policy: domain.PolicyAllowed, NativeEffect: domain.NativeEffectUnchanged}, a.initialError
	}
	return domain.ActivationOutcome{Activation: domain.ActivationActive, Verification: domain.VerificationInstalled, Authentication: domain.AuthenticationNotRequired, Policy: domain.PolicyAllowed, NativeEffect: domain.NativeEffectUnchanged}, nil
}
func (*failedActivationResumeActor) Deactivate(context.Context, domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	panic("unused")
}

// A failed initial client installation leaves a valid owned package but no
// native registration. Same-pin retry must use the acting resume contract;
// read-only absence cannot turn this recoverable state into a terminal refusal.
func TestFailedInitialActivationRetriesActingResumeOnSamePin(t *testing.T) {
	service, store, _ := serviceFixture(t)
	client := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: canonicalCodexProfile(t, ".codex"), ExecutablePath: "/TEST/codex"}
	cause := errors.New("initial native registration failed")
	actor := &failedActivationResumeActor{initialError: cause}
	service.Activator = actor
	input := addInput(t, client, "https://example.invalid/failed-initial-activation")
	input.Confirmed = true
	input.BackendExecutable = client.ExecutablePath
	failed, err := service.Add(t.Context(), input)
	if !errors.Is(err, cause) || !failed.Mutated {
		t.Fatalf("initial failure lost committed package: %+v %v", failed, err)
	}
	before, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyBinding(before.Installations[0])
	if binding.Activation != domain.ActivationFailed || binding.Materialization != domain.MaterializationMaterialized || binding.NativeActivationAttempt != "" || binding.PendingNativeIntent != nil {
		t.Fatalf("fixture is not a settled failed activation: %+v", binding)
	}
	info, err := os.Stat(binding.TargetLocator)
	if err != nil {
		t.Fatal(err)
	}
	retried, err := service.Add(t.Context(), input)
	if err != nil {
		t.Fatalf("same-pin retry refused native absence rather than resuming: %+v %v", retried, err)
	}
	if actor.reads != 0 || actor.acts != 2 {
		t.Fatalf("retry did not use acting resume: reads=%d acts=%d", actor.reads, actor.acts)
	}
	if retried.NoChange || !retried.Mutated || retried.Receipt.OperationID != "" || retried.Activation.Activation != domain.ActivationActive || retried.Activation.Authentication != domain.AuthenticationNotRequired || retried.Activation.Verification != domain.VerificationInstalled {
		t.Fatalf("retry did not converge without package replacement: %+v", retried)
	}
	after, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	current := onlyBinding(after.Installations[0])
	if !current.LifecycleConverged() || !reflect.DeepEqual(binding.Receipts, current.Receipts) || binding.DataReceiptID != current.DataReceiptID || !reflect.DeepEqual(binding.PackageRevision, current.PackageRevision) {
		t.Fatal("retry changed package/data authority or retained failed lifecycle")
	}
	currentInfo, err := os.Stat(binding.TargetLocator)
	if err != nil || !os.SameFile(info, currentInfo) {
		t.Fatalf("retry replaced committed package: %v", err)
	}
}
