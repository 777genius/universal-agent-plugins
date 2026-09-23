package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

func (session *repairSession) repairPackage() (AddResult, error) {
	verification, ok := repairMismatchKind(session.verifyErr)
	if !ok {
		return session.result, fmt.Errorf("managed package integrity could not be determined; refusing repair: %w", session.verifyErr)
	}
	beforeDigest, err := repairBeforeDigest(verification)
	if err != nil {
		return session.result, err
	}
	if session.input.DryRun {
		return session.result, nil
	}
	if !session.input.Confirmed {
		session.result.RequiresConfirmation = true
		return session.result, nil
	}
	delivery, err := session.stageRepairDelivery(false)
	if err != nil {
		return session.result, err
	}
	defer func() { _ = session.service.Stager.Discard(context.Background(), delivery) }()
	if err := session.service.verifyRepairPrecondition(session.ctx, session.target.ActivePath, session.expectedDigest, verification.Kind, beforeDigest); err != nil {
		return session.result, err
	}
	return session.commitRepairPackage(delivery, beforeDigest)
}

func repairBeforeDigest(verification *ports.VerificationError) (string, error) {
	if verification.Kind != ports.VerificationDigestMismatch {
		return "", nil
	}
	if strings.TrimSpace(verification.ActualDigest) == "" {
		return "", fmt.Errorf("managed package verifier reported a digest mismatch without the actual digest; refusing repair")
	}
	return verification.ActualDigest, nil
}

func (session *repairSession) stageRepairDelivery(allowProjectionChange bool) (domain.StagedDelivery, error) {
	operationID := strings.TrimSpace(session.input.OperationID)
	if operationID == "" {
		id, err := newOperationID()
		if err != nil {
			return domain.StagedDelivery{}, err
		}
		operationID = id
	}
	dataPath, err := session.repairDataPath()
	if err != nil {
		return domain.StagedDelivery{}, err
	}
	delivery, err := session.service.stagePackage(session.ctx, session.input.Envelope, session.plan, operationID, session.input.Hints, dataPath)
	if err != nil {
		return domain.StagedDelivery{}, err
	}
	delivery, err = bindStagedDeliveryToPhysicalOwner(delivery, session.plan, &session.client)
	if err != nil {
		_ = session.service.Stager.Discard(context.Background(), delivery)
		return domain.StagedDelivery{}, err
	}
	if delivery.ActivePath != session.target.ActivePath {
		_ = session.service.Stager.Discard(context.Background(), delivery)
		return domain.StagedDelivery{}, fmt.Errorf("stager returned an unexpected repair target")
	}
	if !allowProjectionChange && delivery.ArtifactDigest != session.expectedDigest {
		_ = session.service.Stager.Discard(context.Background(), delivery)
		return domain.StagedDelivery{}, fmt.Errorf("resolved repair projection digest differs from the originally managed package")
	}
	session.input.OperationID = operationID
	return delivery, nil
}

func (session *repairSession) repairDataPath() (string, error) {
	if !packageNeedsPluginData(session.input.Envelope, session.plan) {
		return "", nil
	}
	if session.service.PluginData == nil {
		return "", fmt.Errorf("PLUGIN_DATA manager is required for stdio repair")
	}
	receipt, ok := session.installation.DataReceipts[session.client.DataReceiptID]
	if !ok || receipt.DataReceiptID == "" {
		return "", fmt.Errorf("exact repair is missing its owned PLUGIN_DATA receipt; run reviewed state recovery")
	}
	if err := session.service.PluginData.ValidateData(session.ctx, receipt); err != nil {
		return "", fmt.Errorf("validate repair PLUGIN_DATA ownership: %w", err)
	}
	return receipt.Locator, nil
}

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
	// The user or client can change the native object while staging runs. Repair
	// may replace the exact absent/digest-mismatched object reviewed above, but
	// never a different object that appeared after preflight.
	kernel := session.service.Kernel
	kernel.StateStore = session.service.StateStore
	desiredClient := session.client
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
	receipt, err := kernel.ApplyDirectory(session.ctx, transaction.DirectoryMutation{
		OperationID: session.input.OperationID, InstallationID: session.installation.InstallationID, ClientBindingID: session.clientKey,
		Sequence: nextSequence(session.client), OwnedBase: delivery.OwnedBase, ActivePath: session.target.ActivePath,
		StagingPath: delivery.StagingPath, BeforeDigest: beforeDigest, AfterDigest: delivery.ArtifactDigest,
		NativeObjects: delivery.NativeObjects, Activation: verifiedState.Activation, Authentication: verifiedState.Authentication,
		Policy: verifiedState.Policy, Verification: verifiedState.Verification, DesiredState: session.state,
		Verify: func(verifyContext context.Context, activePath string) error {
			return session.service.Stager.Verify(verifyContext, activePath, delivery.ArtifactDigest)
		},
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
	if nativeLifecycleClient(session.input.Client.ClientID) {
		return session.reapplyRepairedNative(delivery)
	}
	if domain.ClientTraitsFor(session.input.Client.ClientID).UsesManagedStdioLauncher {
		return session.verifyRepairedLauncher(delivery)
	}
	_ = verifiedState
	return session.result, nil
}

func (session *repairSession) reapplyRepairedNative(delivery domain.StagedDelivery) (AddResult, error) {
	outcome, activationErr := session.service.Activator.Activate(session.ctx, domain.ActivationRequest{
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
	if _, updateErr := session.service.updateLifecycle(session.installation.InstallationID, session.clientKey, outcome); updateErr != nil {
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
