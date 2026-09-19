package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type removeGroupSession struct {
	service           Service
	ctx               context.Context
	input             RemoveGroupInput
	state             domain.StateFileV2
	installation      domain.Installation
	index             int
	groupID           string
	result            RemoveGroupResult
	planned           []plannedRemoval
	byBinding         map[string]int
	externalCompleted int
	desired           domain.StateFileV2
	createdData       []domain.DataReceipt
	byReceiptBinding  map[string]domain.MutationReceipt
}

func (session *removeGroupSession) validateRemoveGroupInput() error {
	if len(session.input.Targets) == 0 {
		return fmt.Errorf("at least one installed target is required")
	}
	if session.service.StateStore == nil || session.service.Paths == nil || session.service.Targets == nil || session.service.Stager == nil || session.service.Activator == nil {
		return fmt.Errorf("agentplugins group removal dependencies are incomplete")
	}
	return nil
}

func (session *removeGroupSession) resolveGroupBindings() error {
	if err := session.loadRemoveGroupInstallation(); err != nil {
		return err
	}
	if err := session.validatePurgeReceipts(); err != nil {
		return err
	}
	session.byBinding = map[string]int{}
	session.planned = []plannedRemoval{}
	for targetIndex, targetInput := range session.input.Targets {
		if err := session.resolveOneRemoval(targetIndex, targetInput); err != nil {
			return err
		}
	}
	return session.validatePurgeCoversActiveBindings()
}

func (session *removeGroupSession) loadRemoveGroupInstallation() error {
	state, err := session.service.StateStore.Load()
	if err != nil {
		return err
	}
	index, installation, err := findInstallation(state, session.input.Selector)
	if err != nil {
		return err
	}
	groupID := strings.TrimSpace(session.input.OperationGroupID)
	if groupID == "" {
		groupID, err = newOperationID()
		if err != nil {
			return err
		}
	}
	session.state = state
	session.index = index
	session.installation = installation
	session.groupID = groupID
	session.result = RemoveGroupResult{
		InstallationID:   installation.InstallationID,
		OperationGroupID: groupID,
		Targets:          make([]RemoveResult, len(session.input.Targets)),
		Phase:            GroupPhasePlanned,
	}
	return nil
}

func (session *removeGroupSession) validatePurgeReceipts() error {
	if !session.input.PurgeData {
		return nil
	}
	if session.service.PluginData == nil {
		return fmt.Errorf("PLUGIN_DATA manager is required for purge")
	}
	for _, receipt := range session.installation.DataReceipts {
		if receipt.State != domain.DataReceiptOwned {
			return fmt.Errorf("PLUGIN_DATA receipt %s is not safely owned", receipt.DataReceiptID)
		}
		if err := session.service.PluginData.ValidateData(session.ctx, receipt); err != nil {
			return err
		}
	}
	return nil
}

func (session *removeGroupSession) resolveOneRemoval(targetIndex int, targetInput RemoveInput) error {
	clientKey, client, err := session.lookupRemovalBinding(targetInput)
	if err != nil {
		return err
	}
	session.result.Targets[targetIndex] = RemoveResult{
		InstallationID:   session.installation.InstallationID,
		Plugin:           session.installation.DeclaredName,
		ClientID:         targetInput.Client.ClientID,
		AffectedSurfaces: preparedAffectedSurfaces(client, targetInput.Client.ClientID),
	}
	if prior, ok := session.byBinding[clientKey]; ok {
		item := &session.planned[prior]
		session.result.Targets[targetIndex].Deactivation = session.result.Targets[item.resultIndexes[0]].Deactivation
		item.resultIndexes = append(item.resultIndexes, targetIndex)
		return nil
	}
	targetRoot, err := session.verifyRemovalOwnership(targetInput, client)
	if err != nil {
		return err
	}
	outcome, err := session.service.Activator.Deactivate(session.ctx, domain.DeactivationRequest{
		Client: targetInput.Client, DeclaredName: session.installation.DeclaredName,
		CurrentActivation: client.Activation, Interactive: targetInput.Interactive, ExternalUninstalled: targetInput.ExternalUninstalled,
		Confirmed: false, PhysicalArtifactID: client.PhysicalArtifact, BackendExecutable: targetInput.BackendExecutable,
		ManagedArtifactPath: client.TargetLocator,
		NativeObjects:       append([]domain.NativeObjectOwnership(nil), client.NativeObjects...),
	})
	session.result.Targets[targetIndex].Deactivation = outcome
	if err != nil {
		return err
	}
	session.byBinding[clientKey] = len(session.planned)
	session.planned = append(session.planned, plannedRemoval{
		input: targetInput, clientKey: clientKey, client: client, targetRoot: targetRoot, resultIndexes: []int{targetIndex},
	})
	return nil
}

func (session *removeGroupSession) lookupRemovalBinding(targetInput RemoveInput) (string, domain.ClientBinding, error) {
	clientKey, client, err := findClientBinding(session.installation, targetInput.Client.ClientID, targetInput.Scope)
	if err == nil {
		return clientKey, client, nil
	}
	for key, candidate := range session.installation.Clients {
		if sameNativeBackend(domain.ClientID(candidate.ClientID), targetInput.Client.ClientID) && candidate.Scope == string(targetInput.Scope) && candidate.Materialization != domain.MaterializationAbsent {
			return key, candidate, nil
		}
	}
	return "", domain.ClientBinding{}, err
}

func (session *removeGroupSession) verifyRemovalOwnership(targetInput RemoveInput, client domain.ClientBinding) (string, error) {
	expected := managedDigest(client)
	if expected == "" {
		return "", fmt.Errorf("managed package digest is missing")
	}
	target, err := session.service.Targets.ResolveTarget(session.ctx, targetInput.Client, targetInput.Scope, client.PhysicalArtifact)
	if err != nil {
		return "", err
	}
	if sameNativeBackend(domain.ClientID(client.ClientID), targetInput.Client.ClientID) {
		target.ActivePath = client.TargetLocator
		target.TargetRoot = filepath.Dir(client.TargetLocator)
	}
	if err := session.service.Paths.RequireExactPath(target.ActivePath, client.TargetLocator); err != nil {
		return "", err
	}
	if err := session.service.Stager.Verify(session.ctx, client.TargetLocator, expected); err != nil {
		return "", fmt.Errorf("ownership verification failed before grouped removal: %w", err)
	}
	return target.TargetRoot, nil
}

func (session *removeGroupSession) validatePurgeCoversActiveBindings() error {
	if !session.input.PurgeData {
		return nil
	}
	selectedBindings := make(map[string]bool, len(session.planned))
	for _, item := range session.planned {
		selectedBindings[item.clientKey] = true
	}
	for clientKey, binding := range session.installation.Clients {
		if binding.Materialization != domain.MaterializationAbsent && !selectedBindings[clientKey] {
			return fmt.Errorf("--purge-data requires ownership-preflight and removal of every active target")
		}
	}
	return nil
}
