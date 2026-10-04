package usecase

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (session *repairSession) repairNative() (AddResult, error) {
	delivery, complete, err := session.service.activeNativeDelivery(session.ctx, session.input, session.plan, session.installation, session.client)
	if err != nil {
		return session.result, err
	}
	if session.input.Confirmed && !session.input.DryRun {
		if err := session.service.prepareExistingRuntime(session.ctx, session.input.Envelope, session.plan, session.installation, session.client); err != nil {
			return session.result, fmt.Errorf("prepare installed MCP runtime before repair: %w", err)
		}
	}
	if !complete {
		return session.reapplyIntactNative(delivery)
	}
	verified, clientVerifyErr := session.service.verifyClientReadOnly(session.ctx, session.input, session.result, session.client)
	if clientVerifyErr != nil {
		if session.recordedAbsenceRepairable(verified, clientVerifyErr) {
			return session.reapplyIntactNative(delivery)
		}
		if !nativeLifecycleClient(session.input.Client.ClientID, session.plan.SelectedDelivery) || session.client.LocalEntryObservation != nil || verified.LocalEntryObservation != nil || session.ctx.Err() != nil {
			return session.result, clientVerifyErr
		}
		return session.reapplyIntactNative(delivery)
	}
	return session.correctIntactLifecycle(verified)
}

func (session *repairSession) reapplyIntactNative(delivery domain.StagedDelivery) (AddResult, error) {
	previousObservation := session.plan.LocalEntryObservation.Clone()
	// The package bytes are intact but an owned native projection is not.
	// A confirmed repair may reconstruct an absent exact-owned object. The
	// provider still observes the live object before any effect and rejects
	// foreign or tampered state.
	if session.input.DryRun {
		return session.result, nil
	}
	if !session.input.Confirmed {
		session.result.RequiresConfirmation = true
		return session.result, nil
	}
	outcome, activationErr := session.service.activateWithNativeAttempt(session.ctx, session.installation.InstallationID, session.clientKey, domain.ActivationRequest{
		Client: session.input.Client, Plan: session.result.Plan, Delivery: delivery,
		DeclaredName: session.input.Envelope.Manifest.Name, Replacing: true,
		BackendExecutable:     session.input.BackendExecutable,
		PreviousNativeObjects: append([]domain.NativeObjectOwnership(nil), session.client.NativeObjects...),
	})
	outcome = preserveManagedAuthentication(outcome, session.client.Authentication)
	session.result.Activation = outcome
	changed, err := session.service.updateActivationResultWithObservation(session.installation.InstallationID, session.clientKey, outcome, activationErr, session.client.NativeObjects, previousObservation)
	if err != nil {
		return session.result, fmt.Errorf("persist repaired native lifecycle: %w", err)
	}
	session.result.Mutated = changed
	if activationErr != nil {
		return session.result, fmt.Errorf("repair managed native state: %w", activationErr)
	}
	return session.result, nil
}

func (session *repairSession) correctIntactLifecycle(verified domain.ActivationOutcome) (AddResult, error) {
	corrected := session.client
	if session.client.LocalEntryObservation != nil || verified.LocalEntryObservation != nil {
		if err := validateObservedOutcome(session.client, verified, session.client.LocalEntryObservation); err != nil {
			return session.result, err
		}
	}
	if verified.LocalEntryObservation != nil {
		corrected.LocalEntryObservation = verified.LocalEntryObservation.Clone()
	}
	corrected.Materialization = domain.MaterializationMaterialized
	if corrected.Verification == domain.VerificationFailed {
		corrected.Verification = domain.VerificationPackageValid
	}
	if verified.Activation != "" {
		corrected.Activation = verified.Activation
	}
	if verified.Authentication != "" {
		corrected.Authentication = verified.Authentication
	}
	if verified.Policy != "" {
		corrected.Policy = verified.Policy
	}
	if verified.Verification != "" {
		corrected.Verification = verified.Verification
	}
	session.result.Activation = lifecycleOutcome(corrected)
	if corrected.Materialization == session.client.Materialization && sameLifecycleOutcome(lifecycleOutcome(corrected), lifecycleOutcome(session.client)) {
		session.result.NoChange = true
		return session.result, nil
	}
	if session.input.DryRun {
		return session.result, nil
	}
	if !session.input.Confirmed {
		session.result.RequiresConfirmation = true
		return session.result, nil
	}
	if err := session.persistRepair(corrected); err != nil {
		return session.result, fmt.Errorf("persist corrected repair verification state: %w", err)
	}
	session.result.Mutated = true
	return session.result, nil
}

// Only a positively classified absence at the exact independently owned,
// recorded revision can use the existing confirmation/effect path. Verification
// has already rechecked callback context and the complete frozen binding.
func (session *repairSession) recordedAbsenceRepairable(outcome domain.ActivationOutcome, err error) bool {
	// A multi-error or unrelated terminal error cannot classify pure absence.
	for {
		if _, multiple := err.(interface{ Unwrap() []error }); multiple {
			return false
		}
		next := errors.Unwrap(err)
		if next == nil {
			break
		}
		err = next
	}
	var absent *domain.LocalEntryAbsence
	binding := session.client
	return session.ctx.Err() == nil && outcome.NativeEffect == domain.NativeEffectUnchanged && outcome.LocalEntryObservation == nil &&
		errors.As(err, &absent) && absent.Matches(binding.LocalEntryObservation) &&
		binding.NativeActivationAttempt == "" && binding.PendingNativeIntent == nil &&
		binding.ValidateLocalEntryObservation() == nil &&
		binding.SelectedDelivery.OwnsProfileEntry(binding.NativeObjects) &&
		binding.LocalEntryObservation.Equal(session.plan.LocalEntryObservation) &&
		reflect.DeepEqual(binding.SelectedDelivery, session.plan.SelectedDelivery) &&
		reflect.DeepEqual(binding.LocalEntryObservation.Facts().RevisionBasis, session.plan.SelectedDelivery)
}
