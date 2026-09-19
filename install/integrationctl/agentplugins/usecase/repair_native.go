package usecase

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (session *repairSession) repairNative() (AddResult, error) {
	verified, clientVerifyErr := session.service.verifyClientReadOnly(session.ctx, session.input, session.result, session.client)
	if clientVerifyErr != nil {
		if !nativeLifecycleClient(session.input.Client.ClientID) {
			return session.result, clientVerifyErr
		}
		return session.reapplyIntactNative(clientVerifyErr)
	}
	return session.correctIntactLifecycle(verified)
}

func (session *repairSession) reapplyIntactNative(clientVerifyErr error) (AddResult, error) {
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
	delivery := domain.StagedDelivery{
		ClientID: session.input.Client.ClientID, OwnedBase: session.result.Plan.TargetRoot,
		ActivePath: session.client.TargetLocator, ArtifactDigest: session.expectedDigest,
		NativeObjects: append([]domain.NativeObjectOwnership(nil), session.client.NativeObjects...),
	}
	outcome, activationErr := session.service.Activator.Activate(session.ctx, domain.ActivationRequest{
		Client: session.input.Client, Plan: session.result.Plan, Delivery: delivery,
		DeclaredName: session.input.Envelope.Manifest.Name, Replacing: true,
		BackendExecutable:     session.input.BackendExecutable,
		PreviousNativeObjects: append([]domain.NativeObjectOwnership(nil), session.client.NativeObjects...),
	})
	outcome = preserveManagedAuthentication(outcome, session.client.Authentication)
	session.result.Activation = outcome
	if activationErr != nil {
		return session.result, fmt.Errorf("repair managed native state: %w", activationErr)
	}
	repairedClient := session.client
	repairedClient.Materialization = domain.MaterializationMaterialized
	repairedClient.Activation = outcome.Activation
	repairedClient.Authentication = outcome.Authentication
	repairedClient.Policy = outcome.Policy
	repairedClient.Verification = outcome.Verification
	if err := session.persistRepair(repairedClient); err != nil {
		return session.result, fmt.Errorf("persist repaired native lifecycle: %w", err)
	}
	session.result.Mutated = true
	_ = clientVerifyErr
	return session.result, nil
}

func (session *repairSession) correctIntactLifecycle(verified domain.ActivationOutcome) (AddResult, error) {
	corrected := session.client
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
