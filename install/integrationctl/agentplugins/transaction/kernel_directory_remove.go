package transaction

import (
	"context"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (kernel Kernel) RemoveDirectory(ctx context.Context, removal DirectoryRemoval) (domain.MutationReceipt, error) {
	owners, err := kernel.removalOwners(ctx, []DirectoryRemoval{removal})
	if err != nil {
		return domain.MutationReceipt{}, err
	}
	kernel = kernel.guardOwners(ctx, owners)
	if kernel.StateStore == nil {
		return domain.MutationReceipt{}, fmt.Errorf("transaction state store is required")
	}
	if err := requireMutationReady(kernel.StateStore); err != nil {
		return domain.MutationReceipt{}, err
	}
	if removal.Sequence < 1 {
		return domain.MutationReceipt{}, fmt.Errorf("removal sequence must be positive")
	}
	state, installationIndex, client, err := kernel.loadClient(removal.InstallationID, removal.ClientBindingID)
	if err != nil {
		return domain.MutationReceipt{}, err
	}
	beforeStateJSON, err := marshalComparableState(state)
	if err != nil {
		return domain.MutationReceipt{}, fmt.Errorf("encode state before directory removal: %w", err)
	}
	if operationIDExists(state, removal.OperationID) {
		return domain.MutationReceipt{}, fmt.Errorf("mutation operation id %q was already used", removal.OperationID)
	}
	if removal.Verify != nil {
		if err := removal.Verify(ctx, removal.ActivePath); err != nil {
			return domain.MutationReceipt{}, fmt.Errorf("verify directory before removal: %w", err)
		}
	}
	directoryReceipt, err := kernel.Directory.Apply(ctx, dirswap.Input{EnvironmentID: environmentID(removal.EnvironmentAuthority), ProfileOwners: neutralOwners(owners),
		DataReceiptID: removal.DataReceiptID, OperationID: removal.OperationID, ClientBindingID: dataBindingID(removal, owners), Sequence: removal.Sequence,
		OwnedBase: removal.OwnedBase, ActivePath: removal.ActivePath, Remove: true, VerifyActive: removal.Verify,
	})
	if err != nil {
		if directoryReceipt.OperationID != "" {
			if rollbackErr := kernel.Directory.Rollback(context.Background(), directoryReceipt); rollbackErr != nil {
				return domain.MutationReceipt{}, fmt.Errorf("%w; rollback failed: %w", err, rollbackErr)
			}
		}
		return domain.MutationReceipt{}, err
	}
	if err := kernel.Directory.VerifyPending(directoryReceipt); err != nil {
		if rollbackErr := kernel.Directory.Rollback(context.Background(), directoryReceipt); rollbackErr != nil {
			return domain.MutationReceipt{}, fmt.Errorf("%w; rollback failed: %w", err, rollbackErr)
		}
		return domain.MutationReceipt{}, err
	}
	receipt := domain.MutationReceipt{DirectoryProof: directoryProof(directoryReceipt), ProfileOwners: copyOwners(owners), DataReceiptID: directoryReceipt.DataReceiptID,
		OperationID:          removal.OperationID,
		OperationGroupID:     firstNonEmpty(removal.OperationGroupID, removal.OperationID),
		Sequence:             removal.Sequence,
		MutationType:         "directory_remove",
		ClientBindingID:      directoryReceipt.ClientBindingID,
		ActivePath:           directoryReceipt.ActivePath,
		BackupPath:           directoryReceipt.BackupPath,
		BeforeDigest:         removal.BeforeDigest,
		Phase:                ReceiptPhaseStateCommitted,
		EnvironmentAuthority: removal.EnvironmentAuthority,
	}
	client.Receipts = append(client.Receipts, receipt)
	client.NativeObjects = nil
	client.Materialization = domain.MaterializationAbsent
	client.Activation = domain.ActivationNotRequired
	client.Authentication = domain.AuthenticationNotRequired
	client.Policy = domain.PolicyAllowed
	client.Verification = domain.VerificationNotRun
	installation := state.Installations[installationIndex]
	installation.OperationGroupID = receipt.OperationGroupID
	installation.Clients[removal.ClientBindingID] = client
	state.Installations[installationIndex] = installation
	if oldState, err := kernel.persistCommitDecision(state, beforeStateJSON); err != nil {
		if oldState {
			if rollbackErr := kernel.Directory.Rollback(context.Background(), directoryReceipt); rollbackErr != nil {
				return domain.MutationReceipt{}, fmt.Errorf("commit removal state: %w; rollback failed: %w", err, rollbackErr)
			}
		}
		return domain.MutationReceipt{}, fmt.Errorf("commit removal state: %w", err)
	}
	if err := kernel.Directory.Commit(ctx, directoryReceipt); err != nil {
		return receipt, fmt.Errorf("finalize committed directory removal: %w", err)
	}
	receipt.Phase = ReceiptPhaseCommitted
	state.Installations[installationIndex].Clients[removal.ClientBindingID] = replaceReceipt(client, receipt)
	if err := kernel.StateStore.Save(state); err != nil {
		return receipt, fmt.Errorf("finalize removal receipt state: %w", err)
	}
	return receipt, nil
}

func (kernel Kernel) RemoveDirectoryGroup(ctx context.Context, group DirectoryRemovalGroup) ([]domain.MutationReceipt, error) {
	owners, err := kernel.removalOwners(ctx, group.Removals)
	if err != nil {
		return nil, err
	}
	kernel = kernel.guardOwners(ctx, owners)
	if kernel.StateStore == nil || len(group.Removals) == 0 || group.OperationGroupID == "" {
		return nil, fmt.Errorf("complete directory removal group is required")
	}
	if err := requireMutationReady(kernel.StateStore); err != nil {
		return nil, err
	}
	before, err := kernel.StateStore.Load()
	if err != nil {
		return nil, err
	}
	beforeJSON, err := marshalComparableState(before)
	if err != nil {
		return nil, err
	}
	if operationGroupIDExists(before, group.OperationGroupID) {
		return nil, fmt.Errorf("operation group id %q was already used", group.OperationGroupID)
	}
	state := group.DesiredState
	applied := make([]appliedGroupMutation, 0, len(group.Removals))
	seenOperations, seenPaths := map[string]struct{}{}, map[string]struct{}{}
	for index, removal := range group.Removals {
		if removal.OperationID == "" || removal.Sequence < 1 {
			return nil, fmt.Errorf("removal %d has incomplete identity", index)
		}
		if _, ok := seenOperations[removal.OperationID]; ok || operationIDExists(before, removal.OperationID) {
			return nil, fmt.Errorf("mutation operation id %q was already used", removal.OperationID)
		}
		seenOperations[removal.OperationID] = struct{}{}
		if _, ok := seenPaths[removal.ActivePath]; ok {
			return nil, fmt.Errorf("physical backend %q occurs more than once in removal group", removal.ActivePath)
		}
		seenPaths[removal.ActivePath] = struct{}{}
		if !removal.Standalone {
			if _, _, err := loadClientFromState(before, removal.InstallationID, removal.ClientBindingID); err != nil {
				return nil, err
			}
		}
		if removal.Verify != nil {
			if err := removal.Verify(ctx, removal.ActivePath); err != nil {
				return nil, fmt.Errorf("verify directory before grouped removal: %w", err)
			}
		}
	}
	seenOperations, seenPaths = map[string]struct{}{}, map[string]struct{}{}
	rollback := func() error {
		var result error
		for index := len(applied) - 1; index >= 0; index-- {
			if err := kernel.Directory.Rollback(context.Background(), applied[index].receipt); err != nil {
				result = errors.Join(result, err)
			}
		}
		return result
	}
	for index, removal := range group.Removals {
		if removal.OperationID == "" || removal.Sequence < 1 {
			return nil, fmt.Errorf("removal %d has incomplete identity", index)
		}
		if _, ok := seenOperations[removal.OperationID]; ok || operationIDExists(before, removal.OperationID) {
			return nil, fmt.Errorf("mutation operation id %q was already used", removal.OperationID)
		}
		seenOperations[removal.OperationID] = struct{}{}
		if _, ok := seenPaths[removal.ActivePath]; ok {
			return nil, fmt.Errorf("physical backend %q occurs more than once in removal group", removal.ActivePath)
		}
		seenPaths[removal.ActivePath] = struct{}{}
		installationIndex := -1
		if !removal.Standalone {
			var loadErr error
			installationIndex, _, loadErr = loadClientFromState(before, removal.InstallationID, removal.ClientBindingID)
			if loadErr != nil {
				return nil, loadErr
			}
		}
		directoryReceipt, err := kernel.Directory.Apply(ctx, dirswap.Input{EnvironmentID: environmentID(removal.EnvironmentAuthority), ProfileOwners: neutralOwners(owners), DataReceiptID: removal.DataReceiptID, OperationID: removal.OperationID, ClientBindingID: dataBindingID(removal, owners),
			Sequence: removal.Sequence, OwnedBase: removal.OwnedBase, ActivePath: removal.ActivePath, Remove: true, VerifyActive: removal.Verify})
		if err != nil {
			if directoryReceipt.OperationID != "" {
				applied = append(applied, appliedGroupMutation{receipt: directoryReceipt})
			}
			if rollbackErr := rollback(); rollbackErr != nil {
				return nil, groupError(GroupFailureUnknown, fmt.Errorf("apply grouped removal: %w; rollback failed: %w", err, rollbackErr))
			}
			if len(applied) > 0 {
				return nil, groupError(GroupFailureRolledBack, err)
			}
			return nil, groupError(GroupFailureUnchanged, err)
		}
		receipt := domain.MutationReceipt{DirectoryProof: directoryProof(directoryReceipt), ProfileOwners: copyOwners(owners), DataReceiptID: directoryReceipt.DataReceiptID, OperationID: removal.OperationID, OperationGroupID: group.OperationGroupID,
			Sequence: removal.Sequence, MutationType: "directory_remove", ClientBindingID: directoryReceipt.ClientBindingID,
			ActivePath: directoryReceipt.ActivePath, BackupPath: directoryReceipt.BackupPath, BeforeDigest: removal.BeforeDigest,
			Phase: ReceiptPhaseStateCommitted}
		receipt.EnvironmentAuthority = removal.EnvironmentAuthority
		recordedInBinding := false
		if installationIndex >= 0 {
			// The final desired state may intentionally have removed this binding;
			// its durable receipt remains at state-file level for restart recovery.
			for desiredIndex := range state.Installations {
				if state.Installations[desiredIndex].InstallationID == removal.InstallationID {
					state.Installations[desiredIndex].OperationGroupID = group.OperationGroupID
					if desiredClient, ok := state.Installations[desiredIndex].Clients[removal.ClientBindingID]; ok {
						desiredClient.Receipts = append(desiredClient.Receipts, receipt)
						desiredClient.NativeObjects = nil
						desiredClient.Materialization, desiredClient.Activation = domain.MaterializationAbsent, domain.ActivationNotRequired
						desiredClient.Authentication, desiredClient.Policy = domain.AuthenticationNotRequired, domain.PolicyAllowed
						desiredClient.Verification = domain.VerificationNotRun
						state.Installations[desiredIndex].Clients[removal.ClientBindingID] = desiredClient
						recordedInBinding = true
					}
					break
				}
			}
		}
		if !recordedInBinding {
			state.TransactionReceipts = append(state.TransactionReceipts, receipt)
		}
		applied = append(applied, appliedGroupMutation{receipt: directoryReceipt, state: receipt})
	}
	for _, item := range applied {
		if err := kernel.Directory.VerifyPending(item.receipt); err != nil {
			if rollbackErr := rollback(); rollbackErr != nil {
				return nil, groupError(GroupFailureUnknown, fmt.Errorf("%w; rollback failed: %w", err, rollbackErr))
			}
			return nil, groupError(GroupFailureRolledBack, err)
		}
	}
	if old, err := kernel.persistCommitDecision(state, beforeJSON); err != nil {
		if old {
			if rollbackErr := rollback(); rollbackErr != nil {
				return nil, groupError(GroupFailureUnknown, fmt.Errorf("commit grouped removal: %w; rollback failed: %w", err, rollbackErr))
			}
			return nil, groupError(GroupFailureRolledBack, fmt.Errorf("commit grouped removal state: %w", err))
		}
		if !old {
			return groupReceipts(applied), groupError(GroupFailureUnknown, fmt.Errorf("commit grouped removal state: %w", err))
		}
	}
	for _, item := range applied {
		if err := kernel.Directory.Commit(ctx, item.receipt); err != nil {
			return groupReceipts(applied), groupError(GroupFailureCommitted, fmt.Errorf("finalize grouped removal %s: %w", item.state.OperationID, err))
		}
	}
	for installationIndex, installation := range state.Installations {
		for clientKey, client := range installation.Clients {
			for receiptIndex := range client.Receipts {
				if client.Receipts[receiptIndex].OperationGroupID == group.OperationGroupID {
					client.Receipts[receiptIndex].Phase = ReceiptPhaseCommitted
				}
			}
			installation.Clients[clientKey] = client
		}
		state.Installations[installationIndex] = installation
	}
	for index := range state.TransactionReceipts {
		if state.TransactionReceipts[index].OperationGroupID == group.OperationGroupID {
			state.TransactionReceipts[index].Phase = ReceiptPhaseCommitted
		}
	}
	if err := kernel.StateStore.Save(state); err != nil {
		return groupReceipts(applied), groupError(GroupFailureCommitted, fmt.Errorf("finalize grouped removal receipt state: %w", err))
	}
	result := groupReceipts(applied)
	for index := range result {
		result[index].Phase = ReceiptPhaseCommitted
	}
	return result, nil
}
