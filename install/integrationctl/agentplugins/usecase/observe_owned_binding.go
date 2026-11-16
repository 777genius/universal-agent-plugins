package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"

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

// InspectOwnedMaterialization accepts a fresh, adapter-qualified prepared
// installation without treating it as observed native runtime readiness.
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
	if !materializationOnly {
		return false, nil
	}
	return service.inspectPreparedMaterialization(ctx, input, plan, binding, outcome)
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

// The only input to this optional proof is the fresh observation above. Stored
// readiness and guidance cannot replace it, and success never persists Active.
func (service Service) inspectPreparedMaterialization(ctx context.Context, input AddInput, plan domain.DeliveryPlan, binding domain.ClientBinding, outcome domain.ActivationOutcome) (bool, error) {
	verifier, ok := service.Activator.(ports.PreparedMaterializationVerifier)
	if !ok || !verifier.PreparedMaterializationVerifierAvailable(clonePreparedVerifierClient(input.Client)) || !preparedMaterializationCandidate(binding, outcome) {
		return false, nil
	}
	if err := service.checkMaterializationContext(ctx); err != nil {
		return false, err
	}
	if service.StateStore == nil {
		return false, errors.New("materialization state verification is unavailable")
	}
	before, err := service.StateStore.Load()
	if err != nil {
		return false, err
	}
	if !retainedMaterializationBinding(before, binding) {
		return false, errors.New("materialization binding changed before classification")
	}
	before, err = freezeMaterializationState(before)
	if err != nil {
		return false, err
	}
	request := preparedMaterializationRequest(input, plan, binding)
	verified := verifier.PreparedMaterializationVerified(request, cloneMaterializationOutcome(outcome))
	if err := service.checkMaterializationContext(ctx); err != nil {
		return false, err
	}
	after, err := service.StateStore.Load()
	if err != nil {
		return false, err
	}
	if !reflect.DeepEqual(before, after) {
		return false, errors.New("materialization classifier changed retained state")
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return verified, nil
}

func preparedMaterializationCandidate(binding domain.ClientBinding, outcome domain.ActivationOutcome) bool {
	return binding.Materialization == domain.MaterializationMaterialized &&
		binding.NativeActivationAttempt == "" && binding.PendingNativeIntent == nil &&
		len(binding.NativeObjects) > 0 && reflect.DeepEqual(binding.NativeObjects, outcome.NativeObjects) &&
		outcome.Activation == domain.ActivationPrepared && outcome.Verification == domain.VerificationInstalled &&
		outcome.Policy == domain.PolicyAllowed && outcome.NativeEffect == domain.NativeEffectUnchanged &&
		!outcome.ActivationAttested && !outcome.AuthenticationAttested && !outcome.AuthoritativeObservation &&
		outcome.LocalEntryObservation == nil
}

func preparedMaterializationRequest(input AddInput, plan domain.DeliveryPlan, binding domain.ClientBinding) domain.ActivationRequest {
	plan = clonePreparedVerifierPlan(plan)
	plan.LocalEntryObservation = binding.LocalEntryObservation.Clone()
	plan.PreviousNativeObjects = append([]domain.NativeObjectOwnership(nil), binding.NativeObjects...)
	return domain.ActivationRequest{
		Client: clonePreparedVerifierClient(input.Client), Plan: plan,
		Delivery: domain.StagedDelivery{ClientID: input.Client.ClientID, OwnedBase: plan.TargetRoot,
			ActivePath: binding.TargetLocator, ArtifactDigest: managedDigest(binding),
			NativeObjects: append([]domain.NativeObjectOwnership(nil), binding.NativeObjects...)},
		DeclaredName: input.Envelope.Manifest.Name, Replacing: true, VerifyOnly: true,
		BackendExecutable:     input.BackendExecutable,
		PreviousNativeObjects: append([]domain.NativeObjectOwnership(nil), binding.NativeObjects...),
	}
}

func retainedMaterializationBinding(state domain.StateFileV2, expected domain.ClientBinding) bool {
	if expected.ClientBindingID == "" {
		return false
	}
	matches := 0
	for _, installation := range state.Installations {
		if actual, ok := installation.Clients[expected.ClientBindingID]; ok {
			if !reflect.DeepEqual(actual, expected) {
				return false
			}
			matches++
		}
	}
	return matches == 1
}

func (service Service) checkMaterializationContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if service.profileCheck != nil {
		return service.profileCheck()
	}
	return nil
}

func freezeMaterializationState(state domain.StateFileV2) (domain.StateFileV2, error) {
	// As for native removal, snapshot the entire persisted representation.
	// A memory-backed store must not retain aliases into this comparison basis.
	body, err := json.Marshal(state)
	if err != nil {
		return domain.StateFileV2{}, err
	}
	var frozen domain.StateFileV2
	if err := json.Unmarshal(body, &frozen); err != nil {
		return domain.StateFileV2{}, err
	}
	return frozen, nil
}
