package usecase

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type activationObservationPredecessor struct{ binding domain.ClientBinding }

func (service Service) activationBinding(installationID, bindingID string) (domain.ClientBinding, error) {
	state, err := service.StateStore.Load()
	if err != nil {
		return domain.ClientBinding{}, err
	}
	for _, installation := range state.Installations {
		if installation.InstallationID == installationID {
			if binding, ok := installation.Clients[bindingID]; ok {
				return binding, nil
			}
		}
	}
	return domain.ClientBinding{}, fmt.Errorf("activation binding disappeared")
}

func (service Service) refuseImplicitLocalObservation(installationID, bindingID string) error {
	binding, err := service.activationBinding(installationID, bindingID)
	if err != nil {
		return err
	}
	if !binding.SelectedDelivery.IsZero() || binding.LocalEntryObservation != nil {
		return fmt.Errorf("selected Local activation requires an explicit frozen observation predecessor")
	}
	return nil
}

func validateActivationPredecessor(binding domain.ClientBinding, predecessor *activationObservationPredecessor, outcome domain.ActivationOutcome) error {
	if predecessor != nil && !reflect.DeepEqual(binding, predecessor.binding) {
		return fmt.Errorf("activation binding changed before observation acknowledgement")
	}
	if outcome.LocalEntryObservation != nil {
		return validateObservedOutcome(binding, outcome, binding.LocalEntryObservation)
	}
	return nil
}

func (service Service) updateActivationResultWithObservation(installationID, bindingID string, outcome domain.ActivationOutcome, activationErr error, previousObjects []domain.NativeObjectOwnership, previousObservation *domain.LocalEntryObservation) (bool, error) {
	previousObservation = previousObservation.Clone()
	binding, err := service.activationBinding(installationID, bindingID)
	if err != nil {
		return false, err
	}
	if binding.SelectedDelivery.IsZero() && binding.LocalEntryObservation == nil && previousObservation == nil {
		return service.updateActivationResult(installationID, bindingID, outcome, activationErr, previousObjects)
	}
	if !binding.LocalEntryObservation.Equal(previousObservation) {
		return false, fmt.Errorf("activation observation predecessor changed")
	}
	if binding.PendingNativeIntent != nil {
		if err := binding.PendingNativeIntent.Validate(binding); err != nil {
			return false, err
		}
	}
	predecessor := &activationObservationPredecessor{binding: binding}
	outcome.LocalEntryObservation = outcome.LocalEntryObservation.Clone()
	if !binding.SelectedDelivery.IsZero() {
		if activationErr != nil || outcome.NativeEffect == domain.NativeEffectUncertain {
			outcome.LocalEntryObservation = nil
			outcome.NativeEffect = domain.NativeEffectUncertain
		} else if err := validateObservedOutcome(binding, outcome, previousObservation); err != nil {
			return false, err
		}
	}
	return service.applyActivationResult(installationID, bindingID, outcome, activationErr, previousObjects, predecessor)
}

func validateObservedOutcome(binding domain.ClientBinding, outcome domain.ActivationOutcome, previous *domain.LocalEntryObservation) error {
	if f, ok := binding.SelectedDelivery.CursorFacts(); ok {
		return validateCursorObservedOutcome(binding, outcome, previous, f)
	}
	observation := outcome.LocalEntryObservation
	if observation == nil {
		if previous != nil {
			return fmt.Errorf("local outcome did not verify the recorded observation")
		}
		return nil // Historical nil remains readable; no observation is manufactured.
	}
	if outcome.NativeEffect != domain.NativeEffectCommitted && outcome.NativeEffect != domain.NativeEffectUnchanged {
		return fmt.Errorf("local observation requires a certain verified effect")
	}
	if err := observation.Validate(); err != nil {
		return err
	}
	facts := observation.Facts()
	if !reflect.DeepEqual(facts.RevisionBasis, binding.SelectedDelivery) {
		return fmt.Errorf("local observation does not match the committed revision")
	}
	if previous != nil && !previous.Facts().Enabled && facts.Enabled {
		return fmt.Errorf("local observation cannot re-enable a recorded false entry")
	}
	if outcome.NativeEffect == domain.NativeEffectCommitted && !facts.RevisionBasis.OwnsProfileEntry(outcome.NativeObjects) {
		return fmt.Errorf("committed observation lacks owned selector outcome")
	}
	if outcome.NativeEffect == domain.NativeEffectUnchanged && !binding.SelectedDelivery.OwnsProfileEntry(binding.NativeObjects) && binding.PendingNativeIntent == nil {
		return fmt.Errorf("unchanged Local observation has no independent owned predecessor")
	}
	return nil
}

func validateCursorObservedOutcome(binding domain.ClientBinding, outcome domain.ActivationOutcome, previous *domain.LocalEntryObservation, f domain.CursorDeliveryFacts) error {
	if previous != nil || outcome.LocalEntryObservation != nil {
		return fmt.Errorf("cursor outcome carries Local observation")
	}
	receipt := f.PlannedReceipt
	if binding.PendingNativeIntent != nil {
		planned, _ := binding.PendingNativeIntent.Delivery.CursorFacts()
		receipt = planned.PlannedReceipt
	}
	if outcome.NativeEffect != domain.NativeEffectCommitted && outcome.NativeEffect != domain.NativeEffectUnchanged || len(outcome.NativeObjects) != 1 || outcome.NativeObjects[0] != binding.SelectedDelivery.CursorOwnership(receipt) {
		return fmt.Errorf("cursor acknowledgement differs from actual planned ownership")
	}
	return nil
}

func frozenPlanningBinding(input AddInput, installation *domain.Installation) (*domain.ClientBinding, error) {
	if installation == nil {
		return nil, nil
	}
	var found *domain.ClientBinding
	for _, binding := range installation.Clients {
		if !matchesPlanningBinding(input, installation, binding) {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("conflicting exact or shared client/scope bindings before planning")
		}
		if (!binding.SelectedDelivery.IsZero() || sharesPhysicalBackend(input.Client.ClientID, binding.SelectedDelivery)) && (binding.NativeActivationAttempt != "" || binding.PendingNativeIntent != nil) {
			return nil, fmt.Errorf("pending Local attempt forbids replacement planning")
		}
		if err := binding.ValidateLocalEntryObservation(); err != nil {
			return nil, err
		}
		binding.LocalEntryObservation = binding.LocalEntryObservation.Clone()
		binding.NativeObjects = append([]domain.NativeObjectOwnership(nil), binding.NativeObjects...)
		found = &binding
	}
	return found, nil
}

// Historical shared backends freeze their existing physical owner's authority
// before either logical surface reaches the planner. Selected Local stays exact.
func matchesPlanningBinding(input AddInput, installation *domain.Installation, binding domain.ClientBinding) bool {
	if binding.Scope != string(input.Scope) {
		return false
	}
	if !sharesPhysicalBackend(input.Client.ClientID, binding.SelectedDelivery) {
		return binding.ClientID == string(input.Client.ClientID)
	}
	return binding.Materialization != domain.MaterializationAbsent &&
		binding.PhysicalArtifact == domain.ComputePhysicalArtifactID(input.Envelope.Manifest.Name, installation.InstallationID) &&
		sameNativeBackend(domain.ClientID(binding.ClientID), input.Client.ClientID)
}

func validatePlannedObservation(plan domain.DeliveryPlan, observation *domain.LocalEntryObservation, objects []domain.NativeObjectOwnership) error {
	if !plan.LocalEntryObservation.Equal(observation) || (plan.PreviousNativeObjects != nil && !reflect.DeepEqual(plan.PreviousNativeObjects, objects)) {
		return fmt.Errorf("planner changed frozen Local observation or predecessor ownership")
	}
	return nil
}

func (service Service) activateReadOnlyWithObservation(ctx context.Context, binding domain.ClientBinding, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if request.Plan.SelectedDelivery.IsZero() {
		return service.Activator.Activate(ctx, request)
	}
	if err := service.revalidateReadOnlyBinding(binding); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if _, ok := binding.SelectedDelivery.CursorFacts(); ok {
		if !binding.SelectedDelivery.SameSelection(request.Plan.SelectedDelivery) || binding.SelectedDelivery.CanonicalDigest() != request.Plan.SelectedDelivery.CanonicalDigest() || binding.SelectedDelivery.ProjectionDigest() != request.Plan.SelectedDelivery.ProjectionDigest() {
			return domain.ActivationOutcome{}, fmt.Errorf("cursor read-only request changed binding authority")
		}
		// Verification reads acknowledged authority, not a fresh mutation basis.
		request.Plan.SelectedDelivery = binding.SelectedDelivery
	}
	request.Plan.LocalEntryObservation = binding.LocalEntryObservation.Clone()
	request.Plan.PreviousNativeObjects = append([]domain.NativeObjectOwnership(nil), binding.NativeObjects...)
	outcome, err := service.Activator.Activate(ctx, request)
	outcome.LocalEntryObservation = outcome.LocalEntryObservation.Clone()
	if afterErr := service.revalidateReadOnlyBinding(binding); afterErr != nil {
		return outcome, afterErr
	}
	if ctx.Err() != nil {
		return outcome, ctx.Err()
	}
	return outcome, err
}

func (service Service) revalidateReadOnlyBinding(expected domain.ClientBinding) error {
	state, err := service.StateStore.Load()
	if err != nil {
		return err
	}
	for _, installation := range state.Installations {
		if binding, ok := installation.Clients[expected.ClientBindingID]; ok {
			if !reflect.DeepEqual(binding, expected) {
				return fmt.Errorf("readonly callback changed frozen Local binding")
			}
			return nil
		}
	}
	return fmt.Errorf("readonly Local binding disappeared")
}

func cloneLocalObservationPlan(plan domain.DeliveryPlan) domain.DeliveryPlan {
	plan.LocalEntryObservation = plan.LocalEntryObservation.Clone()
	plan.PreviousNativeObjects = append([]domain.NativeObjectOwnership(nil), plan.PreviousNativeObjects...)
	return plan
}

func cloneLocalObservationBinding(binding *domain.ClientBinding) *domain.ClientBinding {
	if binding == nil {
		return nil
	}
	clone := *binding
	clone.ProfileAuthority = domain.CloneProfileAuthority(binding.ProfileAuthority)
	clone.LocalEntryObservation = binding.LocalEntryObservation.Clone()
	clone.PendingNativeIntent = binding.PendingNativeIntent.Clone()
	clone.NativeObjects = append([]domain.NativeObjectOwnership(nil), binding.NativeObjects...)
	return &clone
}

func (service Service) deactivateWithFrozenObservation(ctx context.Context, installationID, bindingID string, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	if request.SelectedDelivery.IsZero() {
		return service.Activator.Deactivate(ctx, request)
	}
	before, err := service.activationBinding(installationID, bindingID)
	if err != nil {
		return domain.DeactivationOutcome{}, err
	}
	if !before.LocalEntryObservation.Equal(request.LocalEntryObservation) || !reflect.DeepEqual(before.NativeObjects, request.NativeObjects) || !reflect.DeepEqual(before.SelectedDelivery, request.SelectedDelivery) {
		return domain.DeactivationOutcome{}, fmt.Errorf("removal request changed frozen authority")
	}
	request.LocalEntryObservation = request.LocalEntryObservation.Clone()
	outcome, err := service.Activator.Deactivate(ctx, request)
	contextErr := ctx.Err()
	after, loadErr := service.activationBinding(installationID, bindingID)
	if loadErr != nil {
		return outcome, errors.Join(err, contextErr, loadErr)
	}
	if !reflect.DeepEqual(before, after) {
		return outcome, errors.Join(err, contextErr, fmt.Errorf("removal callback changed frozen authority"))
	}
	return outcome, errors.Join(err, contextErr)
}
