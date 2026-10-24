package usecase

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// InspectOwnedBinding proves the complete recorded materialization through the
// same projection and verify-only adapter paths used by repair. It never repairs,
// acknowledges an attempt, or persists readiness. Known manual registration can
// be coherent while authentication and activation remain incomplete.
func (service Service) InspectOwnedBinding(ctx context.Context, input AddInput, plan domain.DeliveryPlan, installation domain.Installation, binding domain.ClientBinding) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := validateNativeBinding(binding, input.Client); err != nil {
		return false, err
	}
	if service.Stager == nil || service.Activator == nil {
		return false, errors.New("native materialization verification is unavailable")
	}
	service = service.guardFrozenProfiles(ctx, []domain.DetectedClient{input.Client}, nil)
	if err := service.Stager.Verify(ctx, binding.TargetLocator, managedDigest(binding)); err != nil {
		return false, err
	}
	input.DryRun, input.Confirmed, input.InstallIntent = true, false, plan.InstallIntent
	_, complete, manual, err := service.activeNativeDelivery(ctx, input, plan, installation, binding)
	if err != nil || !complete {
		return false, err
	}
	if manual {
		return true, nil
	}
	return service.inspectOwnedVerification(ctx, input, plan, binding)
}

func (service Service) inspectOwnedVerification(ctx context.Context, input AddInput, plan domain.DeliveryPlan, binding domain.ClientBinding) (bool, error) {
	if plan.SelectedDelivery.IsZero() && !domain.ShouldReadOnlyVerify(input.Client.ClientID, input.BackendExecutable, input.InstallIntent) {
		return service.inspectOwnedNativeIdentity(ctx, input, plan, binding)
	}
	classifier, ok := service.Activator.(ports.ActivationVerifierClassifier)
	if !ok || !classifier.VerifierAvailable(input.Client, plan, input.BackendExecutable) {
		return false, errors.New("complete native verification is unavailable")
	}
	outcome, err := service.verifyClientReadOnly(ctx, input, AddResult{Plan: plan}, binding)
	if err != nil {
		return false, err
	}
	if outcome.NativeEffect != "" && outcome.NativeEffect != domain.NativeEffectUnchanged {
		return false, errors.New("verify-only native adapter reported an effect")
	}
	verified := outcome.Verification == domain.VerificationInstalled || outcome.Verification == domain.VerificationRuntime
	return outcome.Activation == domain.ActivationActive && verified, nil
}

func (service Service) inspectOwnedNativeIdentity(ctx context.Context, input AddInput, plan domain.DeliveryPlan, binding domain.ClientBinding) (bool, error) {
	if service.NativeObserver == nil {
		return false, errors.New("native identity verification is unavailable")
	}
	observation, err := service.nativeIdentityObservation(ctx, input.Client, plan, &binding)
	if err != nil {
		return false, err
	}
	return observation.State == domain.NativeIdentityManaged && observation.Digest == managedDigest(binding) && observation.NativeDiscoveryReconciled, nil
}
