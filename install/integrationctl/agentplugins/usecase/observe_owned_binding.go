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
	return service.inspectOwnedBinding(ctx, input, plan, installation, binding, false)
}

// InspectOwnedMaterialization may settle complete installed files with known
// prepared setup. Its fresh read-only proof never attests native readiness or
// authorizes setup effects; those still require the ordinary reviewed repair.
func (service Service) InspectOwnedMaterialization(ctx context.Context, input AddInput, plan domain.DeliveryPlan, installation domain.Installation, binding domain.ClientBinding) (bool, error) {
	return service.inspectOwnedBinding(ctx, input, plan, installation, binding, true)
}

func (service Service) inspectOwnedBinding(ctx context.Context, input AddInput, plan domain.DeliveryPlan, installation domain.Installation, binding domain.ClientBinding, materializationOnly bool) (bool, error) {
	guarded, complete, manual, err := service.inspectOwnedProjection(ctx, input, plan, installation, binding)
	if err != nil || !complete {
		return false, err
	}
	if manual {
		return true, nil
	}
	return guarded.inspectOwnedVerification(ctx, input, plan, binding, materializationOnly)
}

// InspectOwnedProjection verifies immutable ownership and current projection
// without asserting that the client has activated or loaded those components.
func (service Service) InspectOwnedProjection(ctx context.Context, input AddInput, plan domain.DeliveryPlan, installation domain.Installation, binding domain.ClientBinding) (bool, error) {
	_, complete, _, err := service.inspectOwnedProjection(ctx, input, plan, installation, binding)
	return complete, err
}

func (service Service) inspectOwnedProjection(ctx context.Context, input AddInput, plan domain.DeliveryPlan, installation domain.Installation, binding domain.ClientBinding) (Service, bool, bool, error) {
	if err := ctx.Err(); err != nil {
		return service, false, false, err
	}
	if err := service.validateNativeBinding(ctx, binding, input.Client); err != nil {
		return service, false, false, err
	}
	if service.Stager == nil || service.Activator == nil {
		return service, false, false, errors.New("native materialization verification is unavailable")
	}
	activationPolicy := service.Activator
	service = service.guardFrozenProfiles(ctx, []domain.DetectedClient{input.Client}, nil)
	if err := service.Stager.Verify(ctx, binding.TargetLocator, managedDigest(binding)); err != nil {
		return service, false, false, err
	}
	input.DryRun, input.Confirmed, input.InstallIntent = true, false, plan.InstallIntent
	// Projection only consults optional activation policy; the generic profile
	// wrapper does not expose that optional interface. Keep it visible here
	// while all filesystem/data ports and actual verification remain guarded.
	projection := service
	projection.Activator = activationPolicy
	delivery, complete, manual, err := projection.activeNativeDelivery(ctx, input, plan, installation, binding)
	if err != nil || !complete {
		return service, false, false, err
	}
	manual = manual || projection.manualRegistrationSettled(input, plan, binding, delivery)
	if err := service.profileCheck(); err != nil {
		return service, false, false, err
	}
	return service, true, manual, nil
}

func (service Service) inspectOwnedVerification(ctx context.Context, input AddInput, plan domain.DeliveryPlan, binding domain.ClientBinding, materializationOnly bool) (bool, error) {
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
	if outcome.Activation == domain.ActivationActive && verified {
		return true, nil
	}
	if !materializationOnly || outcome.ActivationAttested || outcome.AuthenticationAttested {
		return false, nil
	}
	// Reuse the adapter's existing fresh-proof classifier with the same detached
	// delivery it receives for reviewed repair. Classification has no effects.
	probe := repairSession{service: service, input: input, result: AddResult{Plan: plan}, client: binding}
	delivery := domain.StagedDelivery{ClientID: input.Client.ClientID, OwnedBase: plan.TargetRoot, ActivePath: binding.TargetLocator, ArtifactDigest: managedDigest(binding), NativeObjects: binding.NativeObjects}
	return probe.canCompletePreparedSetup(delivery, outcome), nil
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
