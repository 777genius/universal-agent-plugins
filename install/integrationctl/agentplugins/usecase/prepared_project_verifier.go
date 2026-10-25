package usecase

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// This error separates a file-only verifier refusal from native absence that
// an acting repair could reconcile. Its actual outcome remains in AddResult.
type preparedProjectVerificationError struct{ cause error }

func (e *preparedProjectVerificationError) Error() string {
	return "prepared project file verification: " + e.cause.Error()
}
func (e *preparedProjectVerificationError) Unwrap() error { return e.cause }

func preparedProjectFileCandidate(input AddInput, plan domain.DeliveryPlan) bool {
	project := input.Scope == domain.ScopeProject || plan.Scope == domain.ScopeProject || plan.TargetContext.Scope == domain.ScopeProject || plan.TargetContext.ProjectRoot != ""
	traits := (domain.SelectedDelivery{}).EffectiveTraitsForScope(input.Client.ClientID, domain.ScopeProject)
	return project && traits.LifecycleKind == domain.LifecyclePrepared && !traits.TracksNativeEffects
}

// Applicability is separate from authority and capability. A malformed project
// candidate must not fall through to a legacy executable-based verifier gate.
func (service Service) preparedProjectFileVerifier(input AddInput, plan domain.DeliveryPlan, id string, binding domain.ClientBinding, delivery domain.StagedDelivery) (bool, error) {
	if input.Scope != domain.ScopeProject || plan.Scope != domain.ScopeProject || plan.TargetContext.Scope != domain.ScopeProject || plan.ClientID != input.Client.ClientID || !plan.SelectedDelivery.IsZero() {
		return false, fmt.Errorf("file verifier scope, client or delivery differs from project")
	}
	target, err := domain.NewTargetContext(plan.TargetContext.Scope, plan.TargetContext.ProjectRoot, plan.TargetContext.UserConfigRoot)
	if err != nil || target.ProjectRoot == "" || target.ProjectRoot != service.ProjectRoot || target.UserConfigRoot == "" || target.UserConfigRoot != input.Client.ConfigRoot {
		return false, fmt.Errorf("file verifier target differs from original project context")
	}
	if err := matchPreparedProjectBinding(id, plan, binding, delivery); err != nil {
		return false, err
	}
	copyInput := input
	copyInput.Client = clonePreparedVerifierClient(input.Client)
	return service.clientVerifierAvailable(copyInput, clonePreparedVerifierPlan(plan)), nil
}

// Classifier and activation each receive a distinct packet. Neither owns the
// caller's frozen slices or authority, even if it mutates its request in place.
func clonePreparedVerifierClient(client domain.DetectedClient) domain.DetectedClient {
	client.ProfileAuthority = domain.CloneProfileAuthority(client.ProfileAuthority)
	client.Surfaces = append([]domain.ClientSurface(nil), client.Surfaces...)
	return client
}
func clonePreparedVerifierPlan(plan domain.DeliveryPlan) domain.DeliveryPlan {
	plan = cloneLocalObservationPlan(plan)
	plan = plan.WithProfileAuthority(plan.ProfileAuthority(), plan.ProfileNamespace())
	plan.Components = append([]domain.ComponentDecision(nil), plan.Components...)
	plan.UserActions = append([]string(nil), plan.UserActions...)
	plan.LocalActions = append([]string(nil), plan.LocalActions...)
	plan.Warnings = append([]string(nil), plan.Warnings...)
	plan.Diagnostics = append([]domain.Diagnostic(nil), plan.Diagnostics...)
	return plan
}
func matchPreparedProjectBinding(id string, plan domain.DeliveryPlan, binding domain.ClientBinding, delivery domain.StagedDelivery) error {
	if !samePreparedProjectBinding(id, plan, binding) || !samePreparedProjectProfile(plan, binding) {
		return fmt.Errorf("file verifier binding differs from exact prepared project")
	}
	if binding.NativeActivationAttempt != "" || binding.PendingNativeIntent != nil {
		return fmt.Errorf("file verifier cannot acknowledge an unresolved native attempt")
	}
	if !samePreparedProjectDelivery(plan, binding, delivery) {
		return fmt.Errorf("file verifier requires the exact current managed receipt")
	}
	object := binding.NativeObjects[0]
	if object.Kind != "managed_package_directory" || object.ObjectID != "package:"+string(plan.ClientID)+":"+plan.PhysicalArtifactID || object.Path != plan.ActivePath || object.LogicalName != plan.DeclaredName {
		return fmt.Errorf("file verifier receipt is not its managed package")
	}
	return nil
}
func samePreparedProjectBinding(id string, plan domain.DeliveryPlan, binding domain.ClientBinding) bool {
	return id != "" && binding.ClientBindingID == domain.ComputeClientBindingID(id, string(plan.ClientID), "project", plan.ActivePath) &&
		binding.ClientID == string(plan.ClientID) && binding.Scope == "project" && binding.Activation == domain.ActivationPrepared &&
		binding.TargetLocator == plan.ActivePath && binding.PhysicalArtifact == plan.PhysicalArtifactID && binding.SelectedDelivery.IsZero()
}
func samePreparedProjectProfile(plan domain.DeliveryPlan, binding domain.ClientBinding) bool {
	root := plan.NativeRegistryRoot
	if authority := plan.ProfileAuthority(); authority != nil {
		root = authority.Facts().CanonicalRoot
	}
	return root != "" && binding.NativeProfileRoot == root && binding.ProfileNamespace == plan.ProfileNamespace() && domain.SameProfileAuthority(binding.ProfileAuthority, plan.ProfileAuthority())
}
func samePreparedProjectDelivery(plan domain.DeliveryPlan, binding domain.ClientBinding, delivery domain.StagedDelivery) bool {
	return delivery.ClientID == plan.ClientID && delivery.ActivePath == plan.ActivePath && delivery.OwnedBase == plan.TargetRoot &&
		delivery.ArtifactDigest != "" && delivery.ArtifactDigest == managedDigest(binding) && reflect.DeepEqual(delivery.NativeObjects, binding.NativeObjects) && len(binding.NativeObjects) == 1
}
func validPreparedProjectFileOutcome(binding domain.ClientBinding, out domain.ActivationOutcome) bool {
	return out.Activation == domain.ActivationPrepared && out.Verification == domain.VerificationInstalled && out.Policy == domain.PolicyAllowed && out.Authentication == binding.Authentication &&
		!out.ActivationAttested && !out.AuthenticationAttested && !out.AuthoritativeObservation && out.LocalEntryObservation == nil && len(out.NativeObjects) == 0 && len(out.UserActions) > 0 &&
		(out.NativeEffect == "" || out.NativeEffect == domain.NativeEffectUnchanged)
}
func (service Service) observePreparedProjectFiles(ctx context.Context, input AddInput, plan domain.DeliveryPlan, id string, binding domain.ClientBinding, delivery domain.StagedDelivery) (domain.ActivationOutcome, error) {
	available, err := service.preparedProjectFileVerifier(input, plan, id, binding, delivery)
	if err != nil {
		return domain.ActivationOutcome{}, &preparedProjectVerificationError{cause: err}
	}
	if !available {
		return domain.ActivationOutcome{}, nil
	}
	delivery.NativeObjects = append([]domain.NativeObjectOwnership(nil), delivery.NativeObjects...)
	out, err := service.activateReadOnlyWithObservation(ctx, binding, domain.ActivationRequest{Client: clonePreparedVerifierClient(input.Client), Plan: clonePreparedVerifierPlan(plan), Delivery: delivery, DeclaredName: plan.DeclaredName, Replacing: true, BackendExecutable: input.BackendExecutable, PreviousNativeObjects: append([]domain.NativeObjectOwnership(nil), binding.NativeObjects...), VerifyOnly: true})
	out = preserveManagedAuthentication(out, binding.Authentication)
	out.UserActions = append([]string(nil), out.UserActions...)
	out.LocalActions = append([]string(nil), out.LocalActions...)
	out.LocalEntryObservation = out.LocalEntryObservation.Clone()
	out.NativeObjects = append([]domain.NativeObjectOwnership(nil), out.NativeObjects...)
	if err == nil && !validPreparedProjectFileOutcome(binding, out) {
		err = fmt.Errorf("verifier returned contradictory file-only evidence (native effect %q)", out.NativeEffect)
	}
	if err != nil {
		return out, &preparedProjectVerificationError{cause: err}
	}
	return out, nil
}
func retainPreparedProjectVerificationFailure(result *AddResult, out domain.ActivationOutcome, err error) bool {
	var refusal *preparedProjectVerificationError
	if !errors.As(err, &refusal) {
		return false
	}
	result.Activation = out
	result.NoChange = false
	result.Mutated = result.Mutated || out.NativeEffect == domain.NativeEffectCommitted
	return true
}

func (session *repairSession) verifyRepairedProjectFiles(delivery domain.StagedDelivery) (AddResult, error) {
	// commitRepairDirectory has already placed the current receipt in this exact
	// session binding; never reuse the predecessor's native ownership objects.
	if session.index < 0 || session.index >= len(session.state.Installations) {
		return session.result, fmt.Errorf("fresh repaired project installation is missing")
	}
	binding, ok := session.state.Installations[session.index].Clients[session.clientKey]
	if !ok {
		return session.result, fmt.Errorf("fresh repaired project binding is missing")
	}
	out, err := session.service.observePreparedProjectFiles(session.ctx, session.input, session.plan, session.installation.InstallationID, binding, delivery)
	session.result.Activation = out
	if err != nil {
		retainPreparedProjectVerificationFailure(&session.result, out, err)
		return session.result, err
	}
	if out.Activation == "" {
		session.result.Activation = lifecycleOutcome(binding)
		return session.result, nil
	}
	if _, err := session.service.updateLifecycle(session.installation.InstallationID, session.clientKey, out); err != nil {
		return session.result, fmt.Errorf("persist repaired project file verification: %w", err)
	}
	return session.result, nil
}
