package transaction

// Directory mutation orchestration; state, ownership and proof helpers live in focused files.

import (
	"context"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/profileauthority"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (kernel Kernel) ApplyDirectory(ctx context.Context, mutation DirectoryMutation) (domain.MutationReceipt, error) {
	owners, err := kernel.mutationOwners(ctx, mutation.DesiredState, []DirectoryMutation{mutation})
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
	if mutation.Sequence < 1 {
		return domain.MutationReceipt{}, fmt.Errorf("mutation sequence must be positive")
	}
	state := mutation.DesiredState
	beforeState, err := kernel.StateStore.Load()
	if err != nil {
		return domain.MutationReceipt{}, fmt.Errorf("load state before directory transaction: %w", err)
	}
	beforeStateJSON, err := marshalComparableState(beforeState)
	if err != nil {
		return domain.MutationReceipt{}, fmt.Errorf("encode state before directory transaction: %w", err)
	}
	if operationIDExists(beforeState, mutation.OperationID) {
		return domain.MutationReceipt{}, fmt.Errorf("mutation operation id %q was already used", mutation.OperationID)
	}
	installationIndex, client, err := loadClientFromState(state, mutation.InstallationID, mutation.ClientBindingID)
	if err != nil {
		return domain.MutationReceipt{}, err
	}
	directoryReceipt, err := kernel.Directory.Apply(ctx, dirswap.Input{EnvironmentID: environmentID(mutation.EnvironmentAuthority), ProfileOwners: neutralOwners(owners),
		OperationID: mutation.OperationID, ClientBindingID: mutation.ClientBindingID, Sequence: mutation.Sequence,
		OwnedBase: mutation.OwnedBase, ActivePath: mutation.ActivePath, StagingPath: mutation.StagingPath,
		RequireAbsent: mutation.RequireAbsent, VerifyActive: mutation.VerifyBefore,
	})
	if err != nil {
		if directoryReceipt.OperationID != "" {
			if rollbackErr := kernel.Directory.Rollback(context.Background(), directoryReceipt); rollbackErr != nil {
				return domain.MutationReceipt{}, fmt.Errorf("%w; rollback failed: %w", err, rollbackErr)
			}
		}
		return domain.MutationReceipt{}, err
	}
	rollback := func(cause error) error {
		if err := kernel.Directory.Rollback(context.Background(), directoryReceipt); err != nil {
			return fmt.Errorf("%w; rollback failed: %w", cause, err)
		}
		return cause
	}
	if mutation.Verify != nil {
		if err := mutation.Verify(ctx, directoryReceipt.ActivePath); err != nil {
			return domain.MutationReceipt{}, rollback(fmt.Errorf("verify activated directory: %w", err))
		}
	}
	if err := kernel.Directory.VerifyPending(directoryReceipt); err != nil {
		return domain.MutationReceipt{}, rollback(err)
	}
	receipt := domain.MutationReceipt{DirectoryProof: directoryProof(directoryReceipt), ProfileOwners: copyOwners(owners), DataReceiptID: directoryReceipt.DataReceiptID,
		OperationID:          mutation.OperationID,
		OperationGroupID:     mutation.OperationID,
		Sequence:             mutation.Sequence,
		MutationType:         "directory_swap",
		ClientBindingID:      mutation.ClientBindingID,
		ActivePath:           directoryReceipt.ActivePath,
		StagingPath:          directoryReceipt.StagingPath,
		BackupPath:           directoryReceipt.BackupPath,
		BeforeDigest:         mutation.BeforeDigest,
		AfterDigest:          mutation.AfterDigest,
		EnvironmentAuthority: mutation.EnvironmentAuthority,
		Phase:                ReceiptPhaseStateCommitted,
	}
	client.Receipts = append(client.Receipts, receipt)
	client.NativeObjects = committedDirectoryObjects(beforeState, mutation.InstallationID, mutation.ClientBindingID, mutation.NativeObjects)
	client.Materialization = domain.MaterializationMaterialized
	client.Activation = mutation.Activation
	client.Authentication = mutation.Authentication
	client.Policy = mutation.Policy
	client.Verification = mutation.Verification
	installation := state.Installations[installationIndex]
	installation.OperationGroupID = mutation.OperationID
	installation.Clients[mutation.ClientBindingID] = client
	state.Installations[installationIndex] = installation
	if oldState, err := kernel.persistCommitDecision(state, beforeStateJSON); err != nil {
		if oldState {
			if rollbackErr := kernel.Directory.Rollback(context.Background(), directoryReceipt); rollbackErr != nil {
				return domain.MutationReceipt{}, fmt.Errorf("commit transaction state: %w; rollback failed: %w", err, rollbackErr)
			}
		}
		return domain.MutationReceipt{}, fmt.Errorf("commit transaction state: %w", err)
	}
	if err := kernel.Directory.Commit(ctx, directoryReceipt); err != nil {
		return receipt, fmt.Errorf("finalize committed directory mutation: %w", err)
	}
	receipt.Phase = ReceiptPhaseCommitted
	state.Installations[installationIndex].Clients[mutation.ClientBindingID] = replaceReceipt(client, receipt)
	if err := kernel.StateStore.Save(state); err != nil {
		return receipt, fmt.Errorf("finalize transaction receipt state: %w", err)
	}
	return receipt, nil
}

func (kernel Kernel) ApplyDirectoryGroup(ctx context.Context, group DirectoryGroup) ([]domain.MutationReceipt, error) {
	owners, err := kernel.mutationOwners(ctx, group.DesiredState, group.Mutations)
	if err != nil {
		return nil, err
	}
	kernel = kernel.guardOwners(ctx, owners)
	if kernel.StateStore == nil {
		return nil, fmt.Errorf("transaction state store is required")
	}
	if err := requireMutationReady(kernel.StateStore); err != nil {
		return nil, err
	}
	if len(group.Mutations) == 0 {
		return nil, fmt.Errorf("directory transaction group is empty")
	}
	if group.OperationGroupID == "" {
		return nil, fmt.Errorf("operation group id is required")
	}
	before, err := kernel.StateStore.Load()
	if err != nil {
		return nil, fmt.Errorf("load state before directory transaction group: %w", err)
	}
	beforeJSON, err := marshalComparableState(before)
	if err != nil {
		return nil, err
	}
	if operationGroupIDExists(before, group.OperationGroupID) {
		return nil, fmt.Errorf("operation group id %q was already used", group.OperationGroupID)
	}
	state := group.DesiredState
	seenOperations := map[string]struct{}{}
	seenPaths := map[string]struct{}{}
	for index, mutation := range group.Mutations {
		if mutation.Sequence < 1 || mutation.OperationID == "" {
			return nil, fmt.Errorf("mutation %d has incomplete identity", index)
		}
		if _, duplicate := seenOperations[mutation.OperationID]; duplicate || operationIDExists(before, mutation.OperationID) {
			return nil, fmt.Errorf("mutation operation id %q was already used", mutation.OperationID)
		}
		seenOperations[mutation.OperationID] = struct{}{}
		if _, duplicate := seenPaths[mutation.ActivePath]; duplicate {
			return nil, fmt.Errorf("physical backend %q occurs more than once in operation group", mutation.ActivePath)
		}
		seenPaths[mutation.ActivePath] = struct{}{}
		if _, _, err := loadClientFromState(state, mutation.InstallationID, mutation.ClientBindingID); err != nil {
			return nil, err
		}
	}
	seenOperations, seenPaths = map[string]struct{}{}, map[string]struct{}{}
	applied := make([]appliedGroupMutation, 0, len(group.Mutations))
	rollback := func() error {
		var rollbackErr error
		for index := len(applied) - 1; index >= 0; index-- {
			if err := kernel.Directory.Rollback(context.Background(), applied[index].receipt); err != nil {
				rollbackErr = errors.Join(rollbackErr, err)
			}
		}
		return rollbackErr
	}
	for index, mutation := range group.Mutations {
		if mutation.Sequence < 1 || mutation.OperationID == "" {
			return nil, fmt.Errorf("mutation %d has incomplete identity", index)
		}
		if _, duplicate := seenOperations[mutation.OperationID]; duplicate || operationIDExists(before, mutation.OperationID) {
			return nil, fmt.Errorf("mutation operation id %q was already used", mutation.OperationID)
		}
		seenOperations[mutation.OperationID] = struct{}{}
		if _, duplicate := seenPaths[mutation.ActivePath]; duplicate {
			return nil, fmt.Errorf("physical backend %q occurs more than once in operation group", mutation.ActivePath)
		}
		seenPaths[mutation.ActivePath] = struct{}{}
		installationIndex, client, err := loadClientFromState(state, mutation.InstallationID, mutation.ClientBindingID)
		if err != nil {
			return nil, err
		}
		directoryReceipt, err := kernel.Directory.Apply(ctx, dirswap.Input{EnvironmentID: environmentID(mutation.EnvironmentAuthority), ProfileOwners: neutralOwners(owners),
			OperationID: mutation.OperationID, ClientBindingID: mutation.ClientBindingID, Sequence: mutation.Sequence,
			OwnedBase: mutation.OwnedBase, ActivePath: mutation.ActivePath, StagingPath: mutation.StagingPath,
			RequireAbsent: mutation.RequireAbsent, VerifyActive: mutation.VerifyBefore,
		})
		if err != nil {
			if directoryReceipt.OperationID != "" {
				applied = append(applied, appliedGroupMutation{receipt: directoryReceipt})
			}
			if rollbackErr := rollback(); rollbackErr != nil {
				return nil, groupError(GroupFailureUnknown, fmt.Errorf("apply grouped directory mutation: %w; rollback failed: %w", err, rollbackErr))
			}
			if len(applied) > 0 {
				return nil, groupError(GroupFailureRolledBack, err)
			}
			return nil, groupError(GroupFailureUnchanged, err)
		}
		applied = append(applied, appliedGroupMutation{receipt: directoryReceipt})
		if mutation.Verify != nil {
			if err := mutation.Verify(ctx, directoryReceipt.ActivePath); err != nil {
				if rollbackErr := rollback(); rollbackErr != nil {
					return nil, groupError(GroupFailureUnknown, fmt.Errorf("verify grouped directory mutation: %w; rollback failed: %w", err, rollbackErr))
				}
				return nil, groupError(GroupFailureRolledBack, fmt.Errorf("verify grouped directory mutation: %w", err))
			}
		}
		receipt := domain.MutationReceipt{DirectoryProof: directoryProof(directoryReceipt), ProfileOwners: copyOwners(owners), DataReceiptID: directoryReceipt.DataReceiptID, OperationID: mutation.OperationID, OperationGroupID: group.OperationGroupID,
			Sequence: mutation.Sequence, MutationType: "directory_swap", ClientBindingID: mutation.ClientBindingID,
			ActivePath: directoryReceipt.ActivePath, StagingPath: directoryReceipt.StagingPath, BackupPath: directoryReceipt.BackupPath,
			BeforeDigest: mutation.BeforeDigest, AfterDigest: mutation.AfterDigest, Phase: ReceiptPhaseStateCommitted}
		receipt.EnvironmentAuthority = mutation.EnvironmentAuthority
		client.Receipts = append(client.Receipts, receipt)
		client.NativeObjects = committedDirectoryObjects(before, mutation.InstallationID, mutation.ClientBindingID, mutation.NativeObjects)
		client.Materialization, client.Activation, client.Authentication = domain.MaterializationMaterialized, mutation.Activation, mutation.Authentication
		client.Policy, client.Verification = mutation.Policy, mutation.Verification
		installation := state.Installations[installationIndex]
		installation.OperationGroupID = group.OperationGroupID
		installation.Clients[mutation.ClientBindingID] = client
		state.Installations[installationIndex] = installation
		applied[len(applied)-1].state = receipt
	}
	if group.PostApplyVerify != nil {
		if err := group.PostApplyVerify(ctx); err != nil {
			if rollbackErr := rollback(); rollbackErr != nil {
				return nil, groupError(GroupFailureUnknown, fmt.Errorf("post-apply group verification: %w; rollback failed: %w", err, rollbackErr))
			}
			return nil, groupError(GroupFailureRolledBack, fmt.Errorf("post-apply group verification: %w", err))
		}
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
				return nil, groupError(GroupFailureUnknown, fmt.Errorf("commit grouped transaction state: %w; rollback failed: %w", err, rollbackErr))
			}
			return nil, groupError(GroupFailureRolledBack, fmt.Errorf("commit grouped transaction state: %w", err))
		}
		if !old {
			return groupReceipts(applied), groupError(GroupFailureUnknown, fmt.Errorf("commit grouped transaction state: %w", err))
		}
	}
	for _, item := range applied {
		if err := kernel.Directory.Commit(ctx, item.receipt); err != nil {
			return groupReceipts(applied), groupError(GroupFailureCommitted, fmt.Errorf("finalize grouped directory mutation %s: %w", item.state.OperationID, err))
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
	if err := kernel.StateStore.Save(state); err != nil {
		return groupReceipts(applied), groupError(GroupFailureCommitted, fmt.Errorf("finalize grouped receipt state: %w", err))
	}
	result := groupReceipts(applied)
	for index := range result {
		result[index].Phase = ReceiptPhaseCommitted
	}
	return result, nil
}

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

func (kernel Kernel) Recover(ctx context.Context) error {
	if kernel.StateStore == nil {
		return fmt.Errorf("transaction state store is required")
	}
	state, err := kernel.StateStore.Load()
	if err != nil {
		return err
	}
	kernel.Directory.Namespace = kernel.Namespace
	open, err := kernel.Directory.ListOpen()
	if err != nil {
		return err
	}
	owners, err := kernel.pendingOwners(ctx, state, open)
	if err != nil {
		return err
	}
	kernel = kernel.guardOwners(ctx, owners)
	changed := false
	openOperations := make(map[string]struct{}, len(open))
	for _, directoryReceipt := range open {
		openOperations[directoryReceipt.OperationID] = struct{}{}
		installationIndex, clientKey, receiptIndex, stateCommitted := findReceipt(state, directoryReceipt)
		if stateCommitted {
			// A prior atomic rename may have become visible while its parent fsync
			// returned an error. Re-saving successfully makes the state commit
			// decision durable before native backup data can be deleted.
			if err := kernel.StateStore.Save(state); err != nil {
				return fmt.Errorf("make recovered state commit durable for %s: %w", directoryReceipt.OperationID, err)
			}
		}
		if err := kernel.Directory.Recover(ctx, directoryReceipt.OperationID, stateCommitted); err != nil {
			return fmt.Errorf("recover directory mutation %s: %w", directoryReceipt.OperationID, err)
		}
		if stateCommitted {
			if installationIndex < 0 {
				state.TransactionReceipts[receiptIndex].Phase = ReceiptPhaseCommitted
			} else {
				installation := state.Installations[installationIndex]
				client := installation.Clients[clientKey]
				client.Receipts[receiptIndex].Phase = ReceiptPhaseCommitted
				installation.Clients[clientKey] = client
				state.Installations[installationIndex] = installation
			}
			changed = true
		}
	}
	// A crash can occur after the directory journal is durably removed but
	// before the state receipt is advanced from state_committed to committed.
	// At that point the state receipt is the durable commit decision and there
	// is no native operation left to recover, so finalize only that receipt.
	for index := range state.TransactionReceipts {
		receipt := &state.TransactionReceipts[index]
		if receipt.Phase == ReceiptPhaseStateCommitted {
			if _, stillOpen := openOperations[receipt.OperationID]; !stillOpen {
				receipt.Phase = ReceiptPhaseCommitted
				changed = true
			}
		}
	}
	for installationIndex, installation := range state.Installations {
		for clientKey, client := range installation.Clients {
			clientChanged := false
			for receiptIndex := range client.Receipts {
				receipt := &client.Receipts[receiptIndex]
				if receipt.Phase != ReceiptPhaseStateCommitted {
					continue
				}
				if _, stillOpen := openOperations[receipt.OperationID]; stillOpen {
					continue
				}
				receipt.Phase = ReceiptPhaseCommitted
				clientChanged = true
			}
			if clientChanged {
				installation.Clients[clientKey] = client
				changed = true
			}
		}
		state.Installations[installationIndex] = installation
	}
	if changed {
		if err := kernel.StateStore.Save(state); err != nil {
			return err
		}
	}
	if kernel.NativeRecovery != nil {
		return kernel.NativeRecovery.Recover(ctx)
	}
	return nil
}

func (kernel Kernel) pendingOwners(ctx context.Context, state domain.StateFileV2, open []dirswap.Receipt) ([]domain.PhysicalProfileOwner, error) {
	var owners []domain.PhysicalProfileOwner
	add := func(scope []domain.PhysicalProfileOwner) error {
		if err := kernel.validateOwners(ctx, scope); err != nil {
			return err
		}
		owners = append(owners, scope...)
		return nil
	}
	for _, j := range open {
		if j.SchemaVersion == 5 || j.SchemaVersion == 7 {
			_, _, _, matched := findReceipt(state, j)
			switch j.Phase {
			case dirswap.PhaseIntent, dirswap.PhaseBackupPending, dirswap.PhaseOldBackedUp, dirswap.PhaseActivationPending, dirswap.PhaseRollbackPending, dirswap.PhaseRolledBack:
				if matched {
					return nil, fmt.Errorf("durable state references unrecoverable physical journal phase %q", j.Phase)
				}
			case dirswap.PhaseCommitPending, dirswap.PhaseCommitted:
				if !matched {
					return nil, fmt.Errorf("physical journal committed without durable state receipt")
				}
			case dirswap.PhaseActivated:
			default:
				return nil, fmt.Errorf("unsupported physical journal phase %q", j.Phase)
			}
			if !matched {
				var receipts []domain.MutationReceipt
				receipts = append(receipts, state.TransactionReceipts...)
				for _, i := range state.Installations {
					for _, b := range i.Clients {
						receipts = append(receipts, b.Receipts...)
					}
				}
				for _, r := range receipts {
					if r.OperationID == j.OperationID && (r.Phase == ReceiptPhaseStateCommitted || r.Phase == ReceiptPhaseCommitted) {
						return nil, fmt.Errorf("physical journal and durable state receipt disagree")
					}
				}
			}
		}
		var scope []domain.PhysicalProfileOwner
		for _, o := range j.ProfileOwners {
			token, err := profileauthority.FromNeutral(*o.Authority)
			if err != nil {
				return nil, err
			}
			scope = append(scope, domain.PhysicalProfileOwner{Namespace: o.Namespace, InstallationID: o.InstallationID, ClientID: o.ClientID, ClientBindingID: o.ClientBindingID, Authority: &token})
		}
		if err := add(scope); err != nil {
			return nil, err
		}
	}
	for _, r := range state.TransactionReceipts {
		if r.Phase == ReceiptPhaseStateCommitted {
			if err := kernel.verifyReceiptProof(r); err != nil {
				return nil, err
			}
			if err := add(r.ProfileOwners); err != nil {
				return nil, err
			}
		}
	}
	for _, installation := range state.Installations {
		for key, b := range installation.Clients {
			for _, r := range b.Receipts {
				if r.Phase != ReceiptPhaseStateCommitted {
					continue
				}
				{
					scope, err := kernel.bindingOwner(ctx, installation.InstallationID, key, b)
					if err != nil {
						return nil, err
					}
					if len(scope) > 0 {
						found := false
						for _, o := range r.ProfileOwners {
							if o.Namespace == b.ProfileNamespace && o.InstallationID == installation.InstallationID && o.ClientID == b.ClientID && o.ClientBindingID == key && domain.SameProfileAuthority(o.Authority, b.ProfileAuthority) {
								found = true
							}
						}
						if !found {
							return nil, fmt.Errorf("pending receipt differs from recorded physical owner")
						}
					}
					if err := kernel.verifyReceiptProof(r); err != nil {
						return nil, err
					}
					if b.ProfileAuthority != nil && len(r.ProfileOwners) == 0 {
						return nil, fmt.Errorf("pending opted receipt lacks physical owners")
					}
					if err := add(r.ProfileOwners); err != nil {
						return nil, err
					}
				}
			}
			for _, j := range open {
				if j.ClientBindingID == key {
					scope, err := kernel.bindingOwner(ctx, installation.InstallationID, key, b)
					if err != nil {
						return nil, err
					}
					if len(scope) > 0 {
						found := false
						for _, o := range j.ProfileOwners {
							if o.InstallationID == installation.InstallationID && o.ClientBindingID == key && o.ClientID == b.ClientID && o.Namespace == b.ProfileNamespace && o.Authority != nil {
								token, err := profileauthority.FromNeutral(*o.Authority)
								found = err == nil && token.Equal(*b.ProfileAuthority)
							}
						}
						if !found {
							return nil, fmt.Errorf("opted journal differs from recorded physical owner")
						}
					}
				}
			}
			if b.PendingNativeIntent != nil || b.NativeActivationAttempt != "" {
				if b.PendingNativeIntent != nil {
					if err := b.PendingNativeIntent.Validate(b); err != nil {
						return nil, err
					}
				}
				scope, err := kernel.bindingOwner(ctx, installation.InstallationID, key, b)
				if err != nil {
					return nil, err
				}
				if err := add(scope); err != nil {
					return nil, err
				}
			}
		}
	}
	// The same owner may appear in several pending decisions; retain one immutable scope.
	unique := map[[4]string]domain.PhysicalProfileOwner{}
	for _, o := range owners {
		key := [4]string{o.Namespace, o.InstallationID, o.ClientID, o.ClientBindingID}
		if old, ok := unique[key]; ok && !domain.SameProfileAuthority(old.Authority, o.Authority) {
			return nil, fmt.Errorf("pending owners disagree")
		}
		unique[key] = o
	}
	owners = nil
	for _, o := range unique {
		owners = append(owners, o)
	}
	return owners, nil
}
