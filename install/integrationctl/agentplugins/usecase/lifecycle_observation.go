package usecase

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

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
	outcome.LocalEntryObservation = nil
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
	if err := service.refuseImplicitLocalObservation(installationID, clientBindingID); err != nil {
		return false, err
	}
	return service.applyActivationResult(installationID, clientBindingID, outcome, activationErr, previousNativeObjects, nil)
}

func (service Service) applyActivationResult(installationID, clientBindingID string, outcome domain.ActivationOutcome, activationErr error, previousNativeObjects []domain.NativeObjectOwnership, predecessor *activationObservationPredecessor) (bool, error) {
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
		return service.updateLifecycleAndNativeObjectsWithAttempt(installationID, clientBindingID, outcome, &confirmed, true, true, predecessor)
	case domain.NativeEffectUnchanged:
		prior := append([]domain.NativeObjectOwnership(nil), previousNativeObjects...)
		return service.updateLifecycleAndNativeObjectsWithAttempt(installationID, clientBindingID, outcome, &prior, true, true, predecessor)
	case domain.NativeEffectUncertain:
		return service.updateLifecycleAndNativeObjectsWithAttempt(installationID, clientBindingID, outcome, nil, false, false, predecessor)
	}
	// Legacy non-native activators have no native effect contract.
	if activationErr == nil {
		return service.updateLifecycle(installationID, clientBindingID, outcome)
	}
	prior := append([]domain.NativeObjectOwnership(nil), previousNativeObjects...)
	return service.updateLifecycleAndNativeObjects(installationID, clientBindingID, outcome, &prior, true)
}

func (service Service) updateLifecycleAndNativeObjects(installationID, clientBindingID string, outcome domain.ActivationOutcome, nativeObjects *[]domain.NativeObjectOwnership, preserveCommittedPackage bool) (bool, error) {
	return service.updateLifecycleAndNativeObjectsWithAttempt(installationID, clientBindingID, outcome, nativeObjects, preserveCommittedPackage, false, nil)
}

func (service Service) updateLifecycleAndNativeObjectsWithAttempt(installationID, clientBindingID string, outcome domain.ActivationOutcome, nativeObjects *[]domain.NativeObjectOwnership, preserveCommittedPackage, clearAttempt bool, predecessor *activationObservationPredecessor) (bool, error) {
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
		if err := validateActivationPredecessor(client, predecessor, outcome); err != nil {
			return false, err
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
			client.PendingNativeIntent = nil
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
	reconciled, err := retainRecordedLocalSelector(client.SelectedDelivery, client, reconciled)
	if err != nil {
		return nil, err
	}
	return &reconciled, nil
}

func lifecycleOutcomeUnchanged(client domain.ClientBinding, outcome domain.ActivationOutcome, nativeObjects *[]domain.NativeObjectOwnership) bool {
	lifecycleUnchanged := client.Activation == outcome.Activation && client.Authentication == outcome.Authentication &&
		client.Policy == outcome.Policy && client.Verification == outcome.Verification
	nativeObjectsUnchanged := nativeObjects == nil || reflect.DeepEqual(client.NativeObjects, *nativeObjects)
	return lifecycleUnchanged && nativeObjectsUnchanged && (outcome.LocalEntryObservation == nil || client.LocalEntryObservation.Equal(outcome.LocalEntryObservation))
}

func applyLifecycleOutcome(client *domain.ClientBinding, outcome domain.ActivationOutcome, nativeObjects *[]domain.NativeObjectOwnership, now time.Time) {
	client.Activation = outcome.Activation
	client.Authentication = outcome.Authentication
	client.Policy = outcome.Policy
	client.Verification = outcome.Verification
	if nativeObjects != nil {
		client.NativeObjects = append([]domain.NativeObjectOwnership(nil), (*nativeObjects)...)
	}
	if outcome.LocalEntryObservation != nil {
		client.LocalEntryObservation = outcome.LocalEntryObservation.Clone()
	}
	client.UpdatedAt = now.Format(time.RFC3339Nano)
}

func lifecycleOutcome(client domain.ClientBinding) domain.ActivationOutcome {
	return domain.ActivationOutcome{LocalEntryObservation: client.LocalEntryObservation.Clone(), Activation: client.Activation, Authentication: client.Authentication, Policy: client.Policy, Verification: client.Verification}
}

func (service Service) verifyClientReadOnly(ctx context.Context, input AddInput, result AddResult, client domain.ClientBinding) (domain.ActivationOutcome, error) {
	if preparedProjectFileCandidate(input, result.Plan) {
		delivery := domain.StagedDelivery{ClientID: input.Client.ClientID, OwnedBase: result.Plan.TargetRoot, ActivePath: client.TargetLocator, ArtifactDigest: managedDigest(client), NativeObjects: client.NativeObjects}
		return service.observePreparedProjectFiles(ctx, input, result.Plan, result.InstallationID, client, delivery)
	}
	if result.Plan.SelectedDelivery.IsZero() && !domain.ShouldReadOnlyVerifyForScope(input.Client.ClientID, input.Scope, input.BackendExecutable, input.InstallIntent) && !service.confirmedPreparedUserObservation(input, result.Plan, client) {
		return domain.ActivationOutcome{}, nil
	}
	delivery := domain.StagedDelivery{ClientID: input.Client.ClientID, OwnedBase: result.Plan.TargetRoot, ActivePath: client.TargetLocator, ArtifactDigest: managedDigest(client), NativeObjects: client.NativeObjects}
	outcome, err := service.activateReadOnlyWithObservation(ctx, client, domain.ActivationRequest{Client: input.Client, Plan: result.Plan, Delivery: delivery, DeclaredName: input.Envelope.Manifest.Name, Replacing: true, BackendExecutable: input.BackendExecutable, PreviousNativeObjects: append([]domain.NativeObjectOwnership(nil), client.NativeObjects...), VerifyOnly: true})
	outcome = preserveManagedAuthentication(outcome, client.Authentication)
	return outcome, err
}

// A confirmed repair may observe an intact Prepared User binding through the
// same scoped verifier used by repeated add. Preview and file-only profiles do
// not acquire native authority, and successful inventory does not mean Active.
func (service Service) confirmedPreparedUserObservation(input AddInput, plan domain.DeliveryPlan, client domain.ClientBinding) bool {
	return input.Scope == domain.ScopeUser && plan.Scope == domain.ScopeUser && client.Scope == string(domain.ScopeUser) &&
		client.Materialization == domain.MaterializationMaterialized && client.Activation == domain.ActivationPrepared &&
		input.Confirmed && !input.DryRun && service.canResumePreparedObservation(input, plan)
}

func preserveManagedAuthentication(outcome domain.ActivationOutcome, current domain.AuthenticationState) domain.ActivationOutcome {
	if current == domain.AuthenticationComplete || outcome.Authentication == "" || outcome.Authentication == domain.AuthenticationNotChecked {
		outcome.Authentication = current
	}
	return outcome
}
