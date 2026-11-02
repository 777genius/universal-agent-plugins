package transaction

// Directory mutation orchestration; state, ownership and proof helpers live in focused files.

import (
	"context"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
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
