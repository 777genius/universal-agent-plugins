package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

type removeSession struct {
	service           Service
	ctx               context.Context
	input             RemoveInput
	result            RemoveResult
	state             domain.StateFileV2
	installationIndex int
	installation      domain.Installation
	clientKey         string
	client            domain.ClientBinding
	expectedDigest    string
	target            domain.DeliveryTarget
}

func (session *removeSession) loadRemoveTarget() error {
	state, err := session.service.StateStore.Load()
	if err != nil {
		return err
	}
	installationIndex, installation, err := findInstallation(state, session.input.Selector)
	if err != nil {
		return err
	}
	if err := session.service.validatePurgeReceipts(session.ctx, session.input.PurgeData, installation); err != nil {
		return err
	}
	clientKey, client, err := findClientBinding(installation, session.input.Client.ClientID, session.input.Scope)
	if err != nil {
		return err
	}
	if err := validatePurgeCoversActiveBindings(session.input.PurgeData, installation, clientKey); err != nil {
		return err
	}
	session.state = state
	session.installationIndex = installationIndex
	session.installation = installation
	session.clientKey = clientKey
	session.client = client
	session.result = RemoveResult{
		InstallationID:   installation.InstallationID,
		Plugin:           installation.DeclaredName,
		ClientID:         session.input.Client.ClientID,
		AffectedSurfaces: preparedAffectedSurfaces(client, session.input.Client.ClientID),
	}
	if len(session.result.AffectedSurfaces) == 0 {
		session.result.AffectedSurfaces = []string{client.ClientID}
	}
	return nil
}

func (session *removeSession) deactivateRemoveTarget() error {
	deactivation, err := session.service.Activator.Deactivate(session.ctx, domain.DeactivationRequest{
		Client: session.input.Client, DeclaredName: session.installation.DeclaredName,
		CurrentActivation: session.client.Activation, Interactive: session.input.Interactive,
		ExternalUninstalled: session.input.ExternalUninstalled,
		Confirmed:           session.input.Confirmed && !session.input.DryRun,
		PhysicalArtifactID:  session.client.PhysicalArtifact,
		BackendExecutable:   session.input.BackendExecutable,
		ManagedArtifactPath: session.client.TargetLocator,
		NativeObjects:       append([]domain.NativeObjectOwnership(nil), session.client.NativeObjects...),
	})
	session.result.Deactivation = deactivation
	return err
}

func (session *removeSession) commitRemove() (RemoveResult, error) {
	if err := session.reloadAfterExternalRemoval(); err != nil {
		return session.result, err
	}
	if err := session.verifyManagedRemoval(); err != nil {
		return session.result, err
	}
	return session.removeManagedDirectories()
}

func (session *removeSession) reloadAfterExternalRemoval() error {
	if !session.result.Deactivation.ExternalRemovalComplete || session.client.Activation != domain.ActivationActive {
		return nil
	}
	if err := session.service.markDeactivated(session.state, session.installationIndex, session.clientKey); err != nil {
		return err
	}
	state, err := session.service.StateStore.Load()
	if err != nil {
		return err
	}
	session.state = state
	session.installation = state.Installations[session.installationIndex]
	session.client = session.installation.Clients[session.clientKey]
	return nil
}

func (session *removeSession) verifyManagedRemoval() error {
	expectedDigest := managedDigest(session.client)
	if expectedDigest == "" {
		return fmt.Errorf("managed package digest is missing; refusing removal and retaining state for reviewed recovery")
	}
	target, err := session.service.Targets.ResolveTarget(session.ctx, session.input.Client, session.input.Scope, session.client.PhysicalArtifact)
	if err != nil {
		return fmt.Errorf("resolve managed removal target: %w", err)
	}
	if err := session.service.Paths.RequireExactPath(target.ActivePath, session.client.TargetLocator); err != nil {
		return fmt.Errorf("refuse removal from untrusted persisted target: %w", err)
	}
	if err := session.service.Stager.Verify(session.ctx, session.client.TargetLocator, expectedDigest); err != nil {
		return fmt.Errorf("managed package was changed or is missing; refusing silent removal and retaining state: %w", err)
	}
	session.expectedDigest = expectedDigest
	session.target = target
	return nil
}

func (session *removeSession) removeManagedDirectories() (RemoveResult, error) {
	operationID := strings.TrimSpace(session.input.OperationID)
	if operationID == "" {
		var err error
		operationID, err = newOperationID()
		if err != nil {
			return session.result, err
		}
	}
	kernel := session.service.Kernel
	kernel.StateStore = session.service.StateStore
	desired, purgeReceipts, createdData, err := session.service.prepareRemovedBindingsState(session.ctx, session.state, session.installationIndex, []string{session.clientKey}, session.input.PurgeData, operationID)
	if err != nil {
		return session.result, err
	}
	removals := session.directoryRemovals(operationID, purgeReceipts)
	receipts, err := kernel.RemoveDirectoryGroup(session.ctx, transaction.DirectoryRemovalGroup{OperationGroupID: operationID, Removals: removals, DesiredState: desired})
	if len(receipts) > 0 {
		session.result.Receipt = receipts[0]
	}
	if err != nil {
		session.service.cleanupUncommittedPluginData(createdData, transaction.FailurePhase(err))
		return session.result, err
	}
	session.result.Mutated = true
	return session.result, nil
}

func (session *removeSession) directoryRemovals(operationID string, purgeReceipts []domain.DataReceipt) []transaction.DirectoryRemoval {
	extra := dataReceiptRemovals(session.service, operationID, purgeReceipts)
	removals := make([]transaction.DirectoryRemoval, 0, 1+len(extra))
	removals = append(removals, transaction.DirectoryRemoval{
		OperationID: operationID, InstallationID: session.installation.InstallationID,
		ClientBindingID: session.clientKey, Sequence: nextSequence(session.client),
		OwnedBase: session.target.TargetRoot, ActivePath: session.client.TargetLocator,
		BeforeDigest: session.expectedDigest,
		Verify: func(verifyContext context.Context, activePath string) error {
			return session.service.Stager.Verify(verifyContext, activePath, session.expectedDigest)
		},
	})
	return append(removals, extra...)
}
