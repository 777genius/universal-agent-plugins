package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

type RemoveGroupInput struct {
	Selector         string
	Targets          []RemoveInput
	OperationGroupID string
	DryRun           bool
	Confirmed        bool
	PurgeData        bool
}

type RemoveGroupResult struct {
	InstallationID   string                   `json:"installation_id"`
	OperationGroupID string                   `json:"operation_group_id,omitempty"`
	Targets          []RemoveResult           `json:"targets"`
	Receipts         []domain.MutationReceipt `json:"-"`
	Mutated          bool                     `json:"mutated"`
	Phase            GroupPhase               `json:"phase"`
	ManagedPhase     GroupPhase               `json:"managed_phase,omitempty"`
}

type plannedRemoval struct {
	input         RemoveInput
	clientKey     string
	client        domain.ClientBinding
	targetRoot    string
	resultIndexes []int
}

func (service Service) RemoveGroup(ctx context.Context, input RemoveGroupInput) (RemoveGroupResult, error) {
	if len(input.Targets) == 0 {
		return RemoveGroupResult{}, fmt.Errorf("at least one installed target is required")
	}
	if service.StateStore == nil || service.Paths == nil || service.Targets == nil || service.Stager == nil || service.Activator == nil {
		return RemoveGroupResult{}, fmt.Errorf("agentplugins group removal dependencies are incomplete")
	}
	release, err := service.beginMutation(ctx, input.DryRun, input.Confirmed)
	if err != nil {
		return RemoveGroupResult{}, err
	}
	if release != nil {
		defer func() { _ = release() }()
	}
	state, err := service.StateStore.Load()
	if err != nil {
		return RemoveGroupResult{}, err
	}
	installationIndex, installation, err := findInstallation(state, input.Selector)
	if err != nil {
		return RemoveGroupResult{}, err
	}
	groupID := strings.TrimSpace(input.OperationGroupID)
	if groupID == "" {
		groupID, err = newOperationID()
		if err != nil {
			return RemoveGroupResult{}, err
		}
	}
	result := RemoveGroupResult{InstallationID: installation.InstallationID, OperationGroupID: groupID, Targets: make([]RemoveResult, len(input.Targets)), Phase: GroupPhasePlanned}
	if input.PurgeData {
		if service.PluginData == nil {
			return result, fmt.Errorf("PLUGIN_DATA manager is required for purge")
		}
		for _, receipt := range installation.DataReceipts {
			if receipt.State != domain.DataReceiptOwned {
				return result, fmt.Errorf("PLUGIN_DATA receipt %s is not safely owned", receipt.DataReceiptID)
			}
			if err := service.PluginData.ValidateData(ctx, receipt); err != nil {
				return result, err
			}
		}
	}
	byBinding := map[string]int{}
	planned := []plannedRemoval{}
	for targetIndex, targetInput := range input.Targets {
		clientKey, client, err := findClientBinding(installation, targetInput.Client.ClientID, targetInput.Scope)
		if err != nil {
			// Either shared Copilot surface addresses the one physical binding.
			for key, candidate := range installation.Clients {
				if sameNativeBackend(domain.ClientID(candidate.ClientID), targetInput.Client.ClientID) && candidate.Scope == string(targetInput.Scope) && candidate.Materialization != domain.MaterializationAbsent {
					clientKey, client, err = key, candidate, nil
					break
				}
			}
			if err != nil {
				return result, err
			}
		}
		result.Targets[targetIndex] = RemoveResult{InstallationID: installation.InstallationID, Plugin: installation.DeclaredName, ClientID: targetInput.Client.ClientID,
			AffectedSurfaces: preparedAffectedSurfaces(client, targetInput.Client.ClientID)}
		if prior, ok := byBinding[clientKey]; ok {
			item := &planned[prior]
			result.Targets[targetIndex].Deactivation = result.Targets[item.resultIndexes[0]].Deactivation
			item.resultIndexes = append(item.resultIndexes, targetIndex)
			continue
		}
		expected := managedDigest(client)
		if expected == "" {
			return result, fmt.Errorf("managed package digest is missing")
		}
		target, err := service.Targets.ResolveTarget(ctx, targetInput.Client, targetInput.Scope, client.PhysicalArtifact)
		if err != nil {
			return result, err
		}
		if sameNativeBackend(domain.ClientID(client.ClientID), targetInput.Client.ClientID) {
			target.ActivePath = client.TargetLocator
			target.TargetRoot = filepath.Dir(client.TargetLocator)
		}
		if err := service.Paths.RequireExactPath(target.ActivePath, client.TargetLocator); err != nil {
			return result, err
		}
		if err := service.Stager.Verify(ctx, client.TargetLocator, expected); err != nil {
			return result, fmt.Errorf("ownership verification failed before grouped removal: %w", err)
		}
		outcome, err := service.Activator.Deactivate(ctx, domain.DeactivationRequest{Client: targetInput.Client, DeclaredName: installation.DeclaredName,
			CurrentActivation: client.Activation, Interactive: targetInput.Interactive, ExternalUninstalled: targetInput.ExternalUninstalled,
			Confirmed: false, PhysicalArtifactID: client.PhysicalArtifact, BackendExecutable: targetInput.BackendExecutable,
			ManagedArtifactPath: client.TargetLocator,
			NativeObjects:       append([]domain.NativeObjectOwnership(nil), client.NativeObjects...)})
		result.Targets[targetIndex].Deactivation = outcome
		if err != nil {
			return result, err
		}
		byBinding[clientKey] = len(planned)
		planned = append(planned, plannedRemoval{input: targetInput, clientKey: clientKey, client: client, targetRoot: target.TargetRoot, resultIndexes: []int{targetIndex}})
	}
	if input.PurgeData {
		selectedBindings := make(map[string]bool, len(planned))
		for _, item := range planned {
			selectedBindings[item.clientKey] = true
		}
		for clientKey, binding := range installation.Clients {
			if binding.Materialization != domain.MaterializationAbsent && !selectedBindings[clientKey] {
				return result, fmt.Errorf("--purge-data requires ownership-preflight and removal of every active target")
			}
		}
	}
	if input.DryRun || !input.Confirmed {
		return result, nil
	}
	externalCompleted := 0
	for plannedIndex := range planned {
		item := &planned[plannedIndex]
		outcome, err := service.Activator.Deactivate(ctx, domain.DeactivationRequest{Client: item.input.Client, DeclaredName: installation.DeclaredName,
			CurrentActivation: item.client.Activation, Interactive: item.input.Interactive, ExternalUninstalled: item.input.ExternalUninstalled,
			Confirmed: true, PhysicalArtifactID: item.client.PhysicalArtifact, BackendExecutable: item.input.BackendExecutable,
			ManagedArtifactPath: item.client.TargetLocator,
			NativeObjects:       append([]domain.NativeObjectOwnership(nil), item.client.NativeObjects...)})
		for _, resultIndex := range item.resultIndexes {
			result.Targets[resultIndex].Deactivation = outcome
		}
		if err != nil {
			for _, resultIndex := range item.resultIndexes {
				result.Targets[resultIndex].GroupPhase = GroupTargetExternalFailed
			}
			if externalCompleted > 0 {
				result.Phase = GroupPhaseExternalPartialFailure
			} else {
				result.Phase = GroupPhaseManagedUnchanged
			}
			return result, fmt.Errorf("external deactivation failed; managed materialization was retained for repair: %w", err)
		}
		if !outcome.ArtifactRemovalAllowed {
			result.Phase = GroupPhaseManagedUnchanged
			if externalCompleted > 0 {
				result.Phase = GroupPhaseExternalPartialFailure
			}
			return result, fmt.Errorf("client did not authorize managed artifact removal")
		}
		for _, resultIndex := range item.resultIndexes {
			result.Targets[resultIndex].GroupPhase = GroupTargetExternalCompleted
		}
		externalCompleted += len(item.resultIndexes)
	}
	removals := make([]transaction.DirectoryRemoval, 0, len(planned))
	for index, item := range planned {
		expected := managedDigest(item.client)
		activePath := item.client.TargetLocator
		operationID := fmt.Sprintf("%s-%03d", groupID, index+1)
		removals = append(removals, transaction.DirectoryRemoval{OperationID: operationID, OperationGroupID: groupID,
			InstallationID: installation.InstallationID, ClientBindingID: item.clientKey, Sequence: nextSequence(item.client), OwnedBase: item.targetRoot,
			ActivePath: activePath, BeforeDigest: expected, Verify: func(verifyContext context.Context, path string) error {
				return service.Stager.Verify(verifyContext, path, expected)
			}})
	}
	keys := make([]string, 0, len(planned))
	for _, item := range planned {
		keys = append(keys, item.clientKey)
	}
	sort.Strings(keys)
	desired, purgeReceipts, createdData, err := service.prepareRemovedBindingsState(ctx, state, installationIndex, keys, input.PurgeData, groupID)
	if err != nil {
		service.cleanupUncommittedPluginData(createdData, transaction.FailurePhase(err))
		result.Phase = GroupPhaseExternalPartialFailure
		for index := range result.Targets {
			result.Targets[index].GroupPhase = GroupTargetExternalPartial
		}
		return result, err
	}
	for index, dataReceipt := range purgeReceipts {
		receipt := dataReceipt
		operationID := fmt.Sprintf("%s-data-%03d", groupID, index+1)
		removals = append(removals, transaction.DirectoryRemoval{OperationID: operationID, OperationGroupID: groupID,
			ClientBindingID: receipt.DataReceiptID, Sequence: 1, OwnedBase: filepath.Dir(receipt.Locator), ActivePath: receipt.Locator,
			BeforeDigest: receipt.OwnershipDigest, Standalone: true, Verify: func(verifyContext context.Context, _ string) error {
				return service.PluginData.ValidateData(verifyContext, receipt)
			}})
	}
	kernel := service.Kernel
	kernel.StateStore = service.StateStore
	receipts, err := kernel.RemoveDirectoryGroup(ctx, transaction.DirectoryRemovalGroup{OperationGroupID: groupID, Removals: removals, DesiredState: desired})
	result.Receipts = receipts
	byReceiptBinding := make(map[string]domain.MutationReceipt, len(receipts))
	for _, receipt := range receipts {
		byReceiptBinding[receipt.ClientBindingID] = receipt
	}
	for _, item := range planned {
		receipt, ok := byReceiptBinding[item.clientKey]
		if !ok {
			continue
		}
		for _, resultIndex := range item.resultIndexes {
			result.Targets[resultIndex].Receipt = receipt
		}
	}
	if err != nil {
		service.cleanupUncommittedPluginData(createdData, transaction.FailurePhase(err))
		switch transaction.FailurePhase(err) {
		case transaction.GroupFailureRolledBack:
			result.ManagedPhase = GroupPhaseManagedRolledBack
			for index := range result.Targets {
				result.Targets[index].GroupPhase = GroupTargetExternalPartial
			}
		case transaction.GroupFailureCommitted:
			result.ManagedPhase = GroupPhaseManagedCommitted
			for _, item := range planned {
				if _, ok := byReceiptBinding[item.clientKey]; !ok {
					continue
				}
				for _, resultIndex := range item.resultIndexes {
					result.Targets[resultIndex].GroupPhase = GroupTargetManagedCommitted
				}
			}
		case transaction.GroupFailureUnknown:
			result.ManagedPhase = GroupPhaseManagedCommitUnknown
			for index := range result.Targets {
				result.Targets[index].GroupPhase = GroupTargetExternalPartial
			}
		default:
			result.ManagedPhase = GroupPhaseManagedUnchanged
			for index := range result.Targets {
				result.Targets[index].GroupPhase = GroupTargetExternalPartial
			}
		}
		if result.ManagedPhase == GroupPhaseManagedCommitted {
			result.Phase = GroupPhaseManagedCommitted
		} else if externalCompleted > 0 {
			result.Phase = GroupPhaseExternalPartialFailure
		} else {
			result.Phase = result.ManagedPhase
		}
		return result, err
	}
	result.Phase = GroupPhaseCompleted
	result.Mutated = true
	for index := range result.Targets {
		result.Targets[index].Mutated = true
		result.Targets[index].GroupPhase = GroupTargetExternalCompleted
	}
	return result, nil
}
