package usecase

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

func (session *repairSession) commitRepairPackage(delivery domain.StagedDelivery, beforeDigest string) (AddResult, error) {
	verifiedState := lifecycleOutcome(session.client)
	verifiedState.Verification = domain.VerificationPackageValid
	result, err := session.commitRepairDirectory(delivery, beforeDigest, verifiedState)
	if err != nil {
		return result, err
	}
	return session.reactivateRepaired(delivery, verifiedState)
}

func (session *repairSession) commitRepairDirectory(delivery domain.StagedDelivery, beforeDigest string, verifiedState domain.ActivationOutcome) (AddResult, error) {
	if err := session.service.revalidateHost(session.ctx, session.input.Client); err != nil {
		return session.result, err
	}

	if session.service.ClientPreparation != nil {
		if err := session.service.ClientPreparation.RevalidateClient(session.ctx, session.input.Client, session.plan); err != nil {
			return session.result, fmt.Errorf("prepared host changed before repair commit: %w", err)
		}
	}
	if err := session.verifyProjectRepairOwnership(); err != nil {
		return session.result, err
	}
	// The user or client can change the native object while staging runs. Repair
	// may replace the exact absent/digest-mismatched object reviewed above, but
	// never a different object that appeared after preflight.
	kernel := session.service.Kernel
	kernel.StateStore = session.service.StateStore
	desiredClient := session.client
	desiredClient.SelectedDelivery = session.plan.SelectedDelivery
	desiredClient.Materialization = domain.MaterializationMaterialized
	desiredClient.Activation = verifiedState.Activation
	desiredClient.Authentication = verifiedState.Authentication
	desiredClient.Policy = verifiedState.Policy
	desiredClient.Verification = verifiedState.Verification
	desiredClient.NativeObjects = append([]domain.NativeObjectOwnership(nil), delivery.NativeObjects...)
	desiredClient.UpdatedAt = session.service.now().Format("2006-01-02T15:04:05.999999999Z07:00")
	session.installation.Clients[session.clientKey] = desiredClient
	session.installation.UpdatedAt = desiredClient.UpdatedAt
	session.state.Installations[session.index] = session.installation
	_ = transaction.UpsertEnvironmentReceipt(&session.state, session.input.EnvironmentAuthority, session.installation.InstallationID,
		session.clientKey, firstEnvironmentSourceKey(session.input.EnvironmentAuthority, domain.ComputeSourceBindingID(session.input.Envelope.Source)), session.input.Envelope.TreeDigest,
		session.plan.ProfileAuthority(), "committed")
	receipt, err := kernel.ApplyDirectory(session.ctx, transaction.DirectoryMutation{
		OperationID: session.input.OperationID, InstallationID: session.installation.InstallationID, ClientBindingID: session.clientKey,
		Sequence: nextSequence(session.client), OwnedBase: delivery.OwnedBase, ActivePath: session.target.ActivePath,
		StagingPath: delivery.StagingPath, BeforeDigest: beforeDigest, AfterDigest: delivery.ArtifactDigest,
		RequireAbsent: beforeDigest == "",
		VerifyBefore: func(verifyContext context.Context, path string) error {
			return session.service.Stager.Verify(verifyContext, path, beforeDigest)
		},
		NativeObjects: delivery.NativeObjects, Activation: verifiedState.Activation, Authentication: verifiedState.Authentication,
		Policy: verifiedState.Policy, Verification: verifiedState.Verification, DesiredState: session.state,
		Verify: func(verifyContext context.Context, activePath string) error {
			return session.service.Stager.Verify(verifyContext, activePath, delivery.ArtifactDigest)
		},
		EnvironmentAuthority: session.input.EnvironmentAuthority,
	})
	session.result.Receipt = receipt
	if err != nil {
		return session.result, err
	}
	session.result.Mutated = true
	session.result.Activation = verifiedState
	return session.result, nil
}

func (session *repairSession) reactivateRepaired(delivery domain.StagedDelivery, verifiedState domain.ActivationOutcome) (AddResult, error) {
	if preparedProjectFileCandidate(session.input, session.plan) {
		return session.verifyRepairedProjectFiles(delivery)
	}
	if nativeProjectionClient(session.input.Client.ClientID, session.plan.SelectedDelivery, session.input.Scope) {
		return session.reapplyRepairedNative(delivery)
	}
	if session.plan.SelectedDelivery.EffectiveTraitsForScope(session.input.Client.ClientID, session.input.Scope).UsesManagedStdioLauncher {
		return session.verifyRepairedLauncher(delivery)
	}
	_ = verifiedState
	return session.result, nil
}

func (session *repairSession) reapplyRepairedNative(delivery domain.StagedDelivery) (AddResult, error) {
	previousObservation := session.plan.LocalEntryObservation.Clone()
	outcome, activationErr := session.service.activateWithNativeAttempt(session.ctx, session.installation.InstallationID, session.clientKey, domain.ActivationRequest{
		Client: session.input.Client, Plan: session.plan,
		Delivery: domain.StagedDelivery{
			ClientID: delivery.ClientID, OwnedBase: delivery.OwnedBase,
			ActivePath: delivery.ActivePath, ArtifactDigest: delivery.ArtifactDigest,
			NativeObjects: append([]domain.NativeObjectOwnership(nil), delivery.NativeObjects...),
		},
		DeclaredName: session.input.Envelope.Manifest.Name, Replacing: true,
		BackendExecutable:     session.input.BackendExecutable,
		PreviousNativeObjects: append([]domain.NativeObjectOwnership(nil), session.client.NativeObjects...),
	})
	outcome = preserveManagedAuthentication(outcome, session.client.Authentication)
	session.result.Activation = outcome
	if _, updateErr := session.service.updateActivationResultWithObservation(session.installation.InstallationID, session.clientKey, outcome, activationErr, session.client.NativeObjects, previousObservation); updateErr != nil {
		if activationErr != nil {
			return session.result, fmt.Errorf("reapply repaired native state: %w; persist verification state: %w", activationErr, updateErr)
		}
		return session.result, updateErr
	}
	if activationErr != nil {
		return session.result, fmt.Errorf("reapply repaired native state: %w", activationErr)
	}
	return session.result, nil
}

func (session *repairSession) verifyRepairedLauncher(delivery domain.StagedDelivery) (AddResult, error) {
	outcome, activationErr := session.service.Activator.Activate(session.ctx, domain.ActivationRequest{
		Client: session.input.Client, Plan: session.plan,
		Delivery:     domain.StagedDelivery{ClientID: delivery.ClientID, OwnedBase: delivery.OwnedBase, ActivePath: delivery.ActivePath, ArtifactDigest: delivery.ArtifactDigest, NativeObjects: delivery.NativeObjects},
		DeclaredName: session.input.Envelope.Manifest.Name, Replacing: true, BackendExecutable: session.input.BackendExecutable, VerifyOnly: true,
	})
	outcome = preserveManagedAuthentication(outcome, session.client.Authentication)
	session.result.Activation = outcome
	if _, updateErr := session.service.updateLifecycle(session.installation.InstallationID, session.clientKey, outcome); updateErr != nil {
		if activationErr != nil {
			return session.result, fmt.Errorf("verify repaired %s plugin: %w; persist verification state: %w", clientDisplayName(session.input.Client.ClientID), activationErr, updateErr)
		}
		return session.result, updateErr
	}
	if activationErr != nil {
		return session.result, activationErr
	}
	return session.result, nil
}
