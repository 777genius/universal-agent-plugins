package usecase

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func lifecycleConverged(client domain.ClientBinding) bool {
	if client.InstallIntent == domain.InstallIntentPrepare {
		return client.Materialization == domain.MaterializationMaterialized && client.Activation == domain.ActivationPrepared && client.Verification == domain.VerificationPackageValid
	}
	authComplete := client.Authentication == domain.AuthenticationNotRequired || client.Authentication == domain.AuthenticationComplete
	return client.Materialization == domain.MaterializationMaterialized &&
		client.Activation == domain.ActivationActive &&
		client.Verification == domain.VerificationInstalled && authComplete
}

// resume retries only the external client lifecycle against an existing,
// digest-verified managed package. It deliberately creates no directory
// transaction or receipt, so repeating add cannot replace or duplicate the
// materialized artifact.
func (service Service) resume(
	ctx context.Context,
	input AddInput,
	result AddResult,
	installationID, clientBindingID string,
	client domain.ClientBinding,
) (AddResult, error) {
	if input.DryRun {
		return result, nil
	}
	if err := service.verifyManagedTarget(ctx, input.Client, input.Scope, client, "resume"); err != nil {
		return result, err
	}
	result.Activation = lifecycleOutcome(client)
	if !input.Confirmed {
		result.RequiresConfirmation = true
		return result, nil
	}
	state, err := service.StateStore.Load()
	if err != nil {
		return result, err
	}
	var installation domain.Installation
	for _, item := range state.Installations {
		if item.InstallationID == installationID {
			installation = item
			break
		}
	}
	delivery, complete, err := service.activeNativeDelivery(ctx, input, result.Plan, installation, client)
	if err != nil {
		return result, err
	}
	if client.NativeActivationAttempt != "" {
		return result, fmt.Errorf("native activation attempt %s is unresolved; inspect the owned client state before retry", client.NativeActivationAttempt)
	}
	outcome, activationErr := service.activateWithNativeAttempt(ctx, installationID, clientBindingID, domain.ActivationRequest{
		Client: input.Client, Plan: result.Plan, Delivery: delivery,
		DeclaredName: input.Envelope.Manifest.Name, Replacing: true,
		Interactive: input.Interactive, BackendExecutable: input.BackendExecutable,
		PreviousNativeObjects: append([]domain.NativeObjectOwnership(nil), client.NativeObjects...),
		VerifyOnly:            complete, ActivationComplete: input.ActivationComplete,
	})
	outcome = service.resumeActivationOutcome(input, result.Plan, client, outcome, activationErr)
	result.Activation = outcome
	changed, updateErr := service.updateActivationResult(installationID, clientBindingID, outcome, activationErr, client.NativeObjects)
	result.Mutated = changed
	if updateErr != nil {
		if activationErr != nil {
			return result, fmt.Errorf("resume client activation: %w; persist activation state: %w", activationErr, updateErr)
		}
		return result, updateErr
	}
	if activationErr != nil {
		return result, activationErr
	}
	result.NoChange = complete && !changed && service.verifiedRegistrationUnchanged(input, result.Plan, client, outcome)
	return result, nil
}

func (service Service) resumeActivationOutcome(input AddInput, plan domain.DeliveryPlan, client domain.ClientBinding, outcome domain.ActivationOutcome, activationErr error) domain.ActivationOutcome {
	// Authentication completion is a separate phase. Client installation/list
	// evidence must never silently complete it.
	if activationErr == nil && !service.clientVerifierAvailable(input, plan) && client.Activation == domain.ActivationActive && client.Verification == domain.VerificationInstalled {
		outcome.Activation = client.Activation
		outcome.Verification = client.Verification
	}
	if (client.Authentication == domain.AuthenticationPending || client.Authentication == domain.AuthenticationNotChecked) && input.AuthComplete {
		outcome.Authentication = domain.AuthenticationComplete
		outcome.AuthenticationAttested = true
	} else if client.Authentication != "" {
		outcome.Authentication = client.Authentication
	}
	if activationErr != nil && outcome.Activation == "" {
		effect, objects := outcome.NativeEffect, outcome.NativeObjects
		outcome = domain.ActivationOutcome{
			Activation: domain.ActivationFailed, Authentication: plan.Authentication,
			Policy: domain.PolicyAllowed, Verification: domain.VerificationFailed,
			NativeEffect: effect, NativeObjects: objects,
		}
	}
	return outcome
}

// Unchecked authentication is not an outstanding action by itself. Report
// an unchanged verified registration without manufacturing an auth attestation.
func (service Service) verifiedRegistrationUnchanged(input AddInput, plan domain.DeliveryPlan, client domain.ClientBinding, outcome domain.ActivationOutcome) bool {
	return service.clientVerifierAvailable(input, plan) &&
		client.Materialization == domain.MaterializationMaterialized &&
		outcome.Activation == domain.ActivationActive && outcome.Verification == domain.VerificationInstalled &&
		outcome.Authentication == domain.AuthenticationNotChecked &&
		(plan.Authentication == domain.AuthenticationNotChecked || plan.Authentication == domain.AuthenticationNotRequired) &&
		!input.AuthComplete && !outcome.AuthenticationAttested && len(outcome.UserActions) == 0
}

// clientVerifierAvailable asks the activator whether an exact client-side
// verifier can observe this plan. An activator without the capability reports
// no verifier, so prior evidence is re-derived rather than retained.
func (service Service) clientVerifierAvailable(input AddInput, plan domain.DeliveryPlan) bool {
	classifier, ok := service.Activator.(ports.ActivationVerifierClassifier)
	if !ok {
		return false
	}
	return classifier.VerifierAvailable(input.Client, plan, input.BackendExecutable)
}

func (service Service) persistObservedLifecycle(input AddInput, result AddResult, installationID, clientBindingID string, outcome domain.ActivationOutcome) (AddResult, error) {
	result.Activation = outcome
	if input.DryRun {
		return result, nil
	}
	if !input.Confirmed {
		result.RequiresConfirmation = true
		return result, nil
	}
	changed, err := service.updateLifecycle(installationID, clientBindingID, outcome)
	result.Mutated = changed
	return result, err
}

func (service Service) persistAuthoritativeObservation(ctx context.Context, input AddInput, installationID, clientBindingID string, observed domain.ClientBinding, outcome domain.ActivationOutcome) (bool, error) {
	if input.Confirmed {
		return service.updateLifecycle(installationID, clientBindingID, outcome)
	}
	release, err := service.beginMutation(ctx, false, true)
	if err != nil {
		return false, err
	}
	defer func() { _ = release() }()
	state, err := service.StateStore.Load()
	if err != nil {
		return false, err
	}
	for _, installation := range state.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		latest, ok := installation.Clients[clientBindingID]
		if !ok || !reflect.DeepEqual(latest, observed) {
			return false, fmt.Errorf("stale client binding observation; state changed before negative evidence could be persisted, retry the command")
		}
		return service.updateLifecycle(installationID, clientBindingID, outcome)
	}
	return false, fmt.Errorf("stale client binding observation; installation changed before negative evidence could be persisted, retry the command")
}
func (service Service) updateLifecycle(installationID, clientBindingID string, outcome domain.ActivationOutcome) (bool, error) {
	return service.updateLifecycleAndNativeObjects(installationID, clientBindingID, outcome, nil, false)
}

func (service Service) updateActivationResult(installationID, clientBindingID string, outcome domain.ActivationOutcome, activationErr error, previousNativeObjects []domain.NativeObjectOwnership) (bool, error) {
	// A discoverable native record owns this exact state decision. Generic
	// failure/lifecycle timestamps cannot change its bound old/target snapshot.
	if fence, ok := service.Kernel.NativeRecovery.(ports.NativeTransitionActivationFence); ok {
		held, err := fence.HoldsActivation(installationID, clientBindingID)
		if err != nil {
			return false, err
		}
		if held {
			return outcome.NativeEffect == domain.NativeEffectCommitted, nil
		}
	}

	switch outcome.NativeEffect {
	case domain.NativeEffectCommitted:
		confirmed := append([]domain.NativeObjectOwnership(nil), outcome.NativeObjects...)
		return service.updateLifecycleAndNativeObjectsWithAttempt(installationID, clientBindingID, outcome, &confirmed, true, true)
	case domain.NativeEffectUnchanged:
		prior := append([]domain.NativeObjectOwnership(nil), previousNativeObjects...)
		return service.updateLifecycleAndNativeObjectsWithAttempt(installationID, clientBindingID, outcome, &prior, true, true)
	case domain.NativeEffectUncertain:
		return service.updateLifecycleAndNativeObjectsWithAttempt(installationID, clientBindingID, outcome, nil, false, false)
	}
	// Legacy non-native activators have no native effect contract.
	if activationErr == nil {
		return service.updateLifecycle(installationID, clientBindingID, outcome)
	}
	prior := append([]domain.NativeObjectOwnership(nil), previousNativeObjects...)
	return service.updateLifecycleAndNativeObjects(installationID, clientBindingID, outcome, &prior, true)
}

func (service Service) updateLifecycleAndNativeObjects(installationID, clientBindingID string, outcome domain.ActivationOutcome, nativeObjects *[]domain.NativeObjectOwnership, preserveCommittedPackage bool) (bool, error) {
	return service.updateLifecycleAndNativeObjectsWithAttempt(installationID, clientBindingID, outcome, nativeObjects, preserveCommittedPackage, false)
}

func (service Service) updateLifecycleAndNativeObjectsWithAttempt(installationID, clientBindingID string, outcome domain.ActivationOutcome, nativeObjects *[]domain.NativeObjectOwnership, preserveCommittedPackage, clearAttempt bool) (bool, error) {
	state, err := service.StateStore.Load()
	if err != nil {
		return false, err
	}
	for installationIndex, installation := range state.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		client, ok := installation.Clients[clientBindingID]
		if !ok {
			return false, fmt.Errorf("client binding disappeared during activation")
		}
		nativeObjects, err = reconcilePreservedNativeObjects(client, nativeObjects, preserveCommittedPackage)
		if err != nil {
			return false, err
		}
		materializationChanged := outcome.NativeEffect == domain.NativeEffectCommitted && client.Materialization != domain.MaterializationMaterialized
		if lifecycleOutcomeUnchanged(client, outcome, nativeObjects) && (!clearAttempt || client.NativeActivationAttempt == "") && !materializationChanged {
			return false, nil
		}
		applyLifecycleOutcome(&client, outcome, nativeObjects, service.now())
		if materializationChanged {
			client.Materialization = domain.MaterializationMaterialized
		}
		if clearAttempt {
			client.NativeActivationAttempt = ""
		}
		installation.Clients[clientBindingID] = client
		installation.UpdatedAt = client.UpdatedAt
		state.Installations[installationIndex] = installation
		if err := service.persistLifecycleState(state); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, fmt.Errorf("installation disappeared during activation")
}

func reconcilePreservedNativeObjects(client domain.ClientBinding, nativeObjects *[]domain.NativeObjectOwnership, preserveCommittedPackage bool) (*[]domain.NativeObjectOwnership, error) {
	if nativeObjects == nil || !preserveCommittedPackage {
		return nativeObjects, nil
	}
	reconciledCapacity, capacityErr := checkedCombinedCapacity(len(client.NativeObjects), len(*nativeObjects))
	if capacityErr != nil {
		return nil, fmt.Errorf("reconcile committed native objects: %w", capacityErr)
	}
	reconciled := make([]domain.NativeObjectOwnership, 0, reconciledCapacity)
	for _, object := range client.NativeObjects {
		if object.Kind == "managed_package_directory" {
			reconciled = append(reconciled, object)
		}
	}
	for _, object := range *nativeObjects {
		if object.Kind != "managed_package_directory" {
			reconciled = append(reconciled, object)
		}
	}
	return &reconciled, nil
}

func lifecycleOutcomeUnchanged(client domain.ClientBinding, outcome domain.ActivationOutcome, nativeObjects *[]domain.NativeObjectOwnership) bool {
	lifecycleUnchanged := client.Activation == outcome.Activation && client.Authentication == outcome.Authentication &&
		client.Policy == outcome.Policy && client.Verification == outcome.Verification
	nativeObjectsUnchanged := nativeObjects == nil || reflect.DeepEqual(client.NativeObjects, *nativeObjects)
	return lifecycleUnchanged && nativeObjectsUnchanged
}

func applyLifecycleOutcome(client *domain.ClientBinding, outcome domain.ActivationOutcome, nativeObjects *[]domain.NativeObjectOwnership, now time.Time) {
	client.Activation = outcome.Activation
	client.Authentication = outcome.Authentication
	client.Policy = outcome.Policy
	client.Verification = outcome.Verification
	if nativeObjects != nil {
		client.NativeObjects = append([]domain.NativeObjectOwnership(nil), (*nativeObjects)...)
	}
	client.UpdatedAt = now.Format(time.RFC3339Nano)
}

func lifecycleOutcome(client domain.ClientBinding) domain.ActivationOutcome {
	return domain.ActivationOutcome{Activation: client.Activation, Authentication: client.Authentication, Policy: client.Policy, Verification: client.Verification}
}

func (service Service) verifyClientReadOnly(ctx context.Context, input AddInput, result AddResult, client domain.ClientBinding) (domain.ActivationOutcome, error) {
	if !domain.ShouldReadOnlyVerify(input.Client.ClientID, input.BackendExecutable, input.InstallIntent) {
		return domain.ActivationOutcome{}, nil
	}
	delivery := domain.StagedDelivery{ClientID: input.Client.ClientID, OwnedBase: result.Plan.TargetRoot, ActivePath: client.TargetLocator, ArtifactDigest: managedDigest(client), NativeObjects: client.NativeObjects}
	outcome, err := service.Activator.Activate(ctx, domain.ActivationRequest{Client: input.Client, Plan: result.Plan, Delivery: delivery, DeclaredName: input.Envelope.Manifest.Name, Replacing: true, BackendExecutable: input.BackendExecutable, PreviousNativeObjects: append([]domain.NativeObjectOwnership(nil), client.NativeObjects...), VerifyOnly: true})
	outcome = preserveManagedAuthentication(outcome, client.Authentication)
	return outcome, err
}

func preserveManagedAuthentication(outcome domain.ActivationOutcome, current domain.AuthenticationState) domain.ActivationOutcome {
	if current == domain.AuthenticationComplete || outcome.Authentication == "" || outcome.Authentication == domain.AuthenticationNotChecked {
		outcome.Authentication = current
	}
	return outcome
}
