package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

func (session *applySession) stageAndCommit() (AddResult, error) {
	if !session.input.Confirmed {
		session.result.RequiresConfirmation = true
		return session.result, nil
	}
	operationID, dataReceipt, dataCreated, err := session.prepareCommitResources()
	if err != nil {
		return session.result, err
	}
	delivery, err := session.stageOwnedDelivery(operationID, dataReceipt, dataCreated)
	if err != nil {
		return session.result, err
	}
	keepCreatedData := false
	defer func() {
		if dataCreated && !keepCreatedData {
			_ = session.service.PluginData.PurgeData(context.Background(), dataReceipt)
		}
	}()
	if err := session.service.observeNativeIdentity(session.ctx, session.input.Client, session.plan, session.managedBinding); err != nil {
		_ = session.service.Stager.Discard(context.Background(), delivery)
		return session.result, fmt.Errorf("native identity changed before commit: %w", err)
	}
	if err := session.service.checkMCPNamespace(session.ctx, session.input.Client, &session.plan, session.managedBinding); err != nil {
		_ = session.service.Stager.Discard(context.Background(), delivery)
		return session.result, fmt.Errorf("MCP namespace changed before commit: %w", err)
	}
	previousClient := domain.ClientBinding{}
	if session.existing {
		previousClient = session.state.Installations[session.installationIndex].Clients[session.clientBindingID]
	}
	session.bindDataReceipt(&dataReceipt)
	if err := session.commitDirectory(operationID, delivery, previousClient); err != nil {
		return session.result, err
	}
	keepCreatedData = true
	session.result.Mutated = true
	return session.activateCommitted(delivery, previousClient)
}

func (session *applySession) prepareCommitResources() (operationID string, dataReceipt domain.DataReceipt, dataCreated bool, err error) {
	operationID = strings.TrimSpace(session.input.OperationID)
	if operationID == "" {
		operationID, err = newOperationID()
		if err != nil {
			return "", domain.DataReceipt{}, false, err
		}
	}
	if !packageNeedsPluginData(session.input.Envelope, session.plan) {
		return operationID, domain.DataReceipt{}, false, nil
	}
	if session.service.PluginData == nil {
		return "", domain.DataReceipt{}, false, fmt.Errorf("PLUGIN_DATA manager is required for stdio MCP packages")
	}
	dataReceipt, dataCreated, err = session.service.PluginData.EnsureData(session.ctx, session.installationID, session.plan.PhysicalArtifactID, string(session.input.Scope))
	if err != nil {
		return "", domain.DataReceipt{}, false, err
	}
	return operationID, dataReceipt, dataCreated, nil
}

func (session *applySession) stageOwnedDelivery(operationID string, dataReceipt domain.DataReceipt, dataCreated bool) (domain.StagedDelivery, error) {
	delivery, err := session.service.stagePackage(session.ctx, session.input.Envelope, session.plan, operationID, session.input.Hints, dataReceipt.Locator)
	if err != nil {
		if dataCreated {
			_ = session.service.PluginData.PurgeData(context.Background(), dataReceipt)
		}
		return domain.StagedDelivery{}, err
	}
	delivery, err = bindStagedDeliveryToPhysicalOwner(delivery, session.plan, session.managedBinding)
	if err != nil {
		_ = session.service.Stager.Discard(context.Background(), delivery)
		if dataCreated {
			_ = session.service.PluginData.PurgeData(context.Background(), dataReceipt)
		}
		return domain.StagedDelivery{}, err
	}
	return delivery, nil
}

func (session *applySession) bindDataReceipt(dataReceipt *domain.DataReceipt) {
	session.state, session.installationIndex = upsertPreparedInstallation(session.state, session.installationIndex, session.existing, session.input, session.plan, session.installationID, session.clientBindingID, session.sourceBindingID, session.service.now())
	if dataReceipt.DataReceiptID == "" {
		for _, retained := range session.state.Installations[session.installationIndex].DataReceipts {
			if retained.PhysicalBackend == session.plan.PhysicalArtifactID && retained.Scope == string(session.input.Scope) {
				*dataReceipt = retained
				break
			}
		}
	}
	if dataReceipt.DataReceiptID == "" {
		return
	}
	installation := session.state.Installations[session.installationIndex]
	if installation.DataReceipts == nil {
		installation.DataReceipts = map[string]domain.DataReceipt{}
	}
	installation.DataReceipts[dataReceipt.DataReceiptID] = *dataReceipt
	client := installation.Clients[session.clientBindingID]
	client.DataReceiptID = dataReceipt.DataReceiptID
	installation.Clients[session.clientBindingID] = client
	installation.DataRetained = false
	session.state.Installations[session.installationIndex] = installation
}

func (session *applySession) commitDirectory(operationID string, delivery domain.StagedDelivery, previousClient domain.ClientBinding) error {
	kernel := session.service.Kernel
	kernel.StateStore = session.service.StateStore
	initialActivation, initialVerification := initialLifecycle(session.plan)
	receipt, applyErr := kernel.ApplyDirectory(session.ctx, transaction.DirectoryMutation{
		OperationID:     operationID,
		InstallationID:  session.installationID,
		ClientBindingID: session.clientBindingID,
		Sequence:        nextSequence(session.state.Installations[session.installationIndex].Clients[session.clientBindingID]),
		OwnedBase:       delivery.OwnedBase,
		ActivePath:      delivery.ActivePath,
		StagingPath:     delivery.StagingPath,
		BeforeDigest:    managedDigest(previousClient),
		AfterDigest:     delivery.ArtifactDigest,
		NativeObjects:   delivery.NativeObjects,
		Activation:      initialActivation,
		Authentication:  session.plan.Authentication,
		Policy:          domain.PolicyAllowed,
		Verification:    initialVerification,
		DesiredState:    session.state,
		Verify: func(verifyContext context.Context, activePath string) error {
			return session.service.Stager.Verify(verifyContext, activePath, delivery.ArtifactDigest)
		},
	})
	session.result.Receipt = receipt
	if applyErr != nil {
		_ = session.service.Stager.Discard(context.Background(), delivery)
		return applyErr
	}
	return nil
}

func (session *applySession) activateCommitted(delivery domain.StagedDelivery, previousClient domain.ClientBinding) (AddResult, error) {
	outcome, activationErr := session.service.Activator.Activate(session.ctx, domain.ActivationRequest{
		Client: session.input.Client, Plan: session.plan, Delivery: domain.StagedDelivery{
			ClientID: delivery.ClientID, OwnedBase: delivery.OwnedBase, ActivePath: delivery.ActivePath,
			ArtifactDigest: delivery.ArtifactDigest, NativeObjects: delivery.NativeObjects,
		},
		DeclaredName: session.input.Envelope.Manifest.Name, Replacing: session.replace || session.registrationMigration,
		Interactive: session.input.Interactive, BackendExecutable: session.input.BackendExecutable,
		PreviousNativeObjects: append([]domain.NativeObjectOwnership(nil), previousClient.NativeObjects...),
	})
	session.result.Activation = outcome
	if activationErr != nil && outcome.Activation == "" {
		outcome = domain.ActivationOutcome{
			Activation: domain.ActivationFailed, Authentication: session.plan.Authentication,
			Policy: domain.PolicyAllowed, Verification: domain.VerificationFailed,
		}
		session.result.Activation = outcome
	}
	if _, updateErr := session.service.updateActivationResult(session.installationID, session.clientBindingID, outcome, activationErr, previousClient.NativeObjects); updateErr != nil {
		if activationErr != nil {
			return session.result, fmt.Errorf("activate client: %w; persist activation state: %w", activationErr, updateErr)
		}
		return session.result, updateErr
	}
	if activationErr != nil {
		return session.result, activationErr
	}
	return session.result, nil
}
