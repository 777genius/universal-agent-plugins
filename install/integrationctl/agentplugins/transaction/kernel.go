package transaction

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	ReceiptPhaseStateCommitted = "state_committed"
	ReceiptPhaseCommitted      = "committed"
)

type GroupFailurePhase string

const (
	GroupFailureUnchanged  GroupFailurePhase = "managed_unchanged"
	GroupFailureRolledBack GroupFailurePhase = "managed_rolled_back"
	GroupFailureUnknown    GroupFailurePhase = "managed_commit_unknown"
	GroupFailureCommitted  GroupFailurePhase = "managed_committed"
)

// GroupError carries the kernel's durable observation of a failed operation
// group. Callers must not infer rollback merely from an empty receipt slice:
// an empty slice can also mean that the first mutation never started.
type GroupError struct {
	Phase GroupFailurePhase
	Err   error
}

func (err *GroupError) Error() string { return err.Err.Error() }
func (err *GroupError) Unwrap() error { return err.Err }

func groupError(phase GroupFailurePhase, err error) error {
	return &GroupError{Phase: phase, Err: err}
}

func FailurePhase(err error) GroupFailurePhase {
	var groupErr *GroupError
	if errors.As(err, &groupErr) {
		return groupErr.Phase
	}
	return GroupFailureUnchanged
}

type StateStore interface {
	Load() (domain.StateFileV2, error)
	Save(domain.StateFileV2) error
}

type DirectoryMutation struct {
	OperationID     string
	InstallationID  string
	ClientBindingID string
	Sequence        int
	OwnedBase       string
	ActivePath      string
	StagingPath     string
	BeforeDigest    string
	AfterDigest     string
	NativeObjects   []domain.NativeObjectOwnership
	Activation      domain.ActivationState
	Authentication  domain.AuthenticationState
	Policy          domain.PolicyState
	Verification    domain.VerificationState
	// DesiredState is the fully validated state that becomes authoritative only
	// after the staged directory is active and verified. Keeping it in memory
	// closes the crash window that would otherwise persist "prepared" state
	// before a durable directory intent exists.
	DesiredState domain.StateFileV2
	Verify       func(context.Context, string) error
	// RequireAbsent rejects this mutation, before any journal write or
	// filesystem mutation, if ActivePath already exists. See dirswap.Input's
	// field of the same name.
	RequireAbsent bool
}

type DirectoryRemoval struct {
	OperationGroupID string
	OperationID      string
	InstallationID   string
	ClientBindingID  string
	Sequence         int
	OwnedBase        string
	ActivePath       string
	BeforeDigest     string
	// Standalone records a removal receipt at state-file level. This is used
	// when the same commit decision removes the owning binding/installation.
	Standalone bool
	Verify     func(context.Context, string) error
}

// DirectoryGroup is one commit decision for a fully preflighted set of
// physical backends. Every directory is activated and verified before state is
// made authoritative. A failure before that decision rolls all activated
// directories back in reverse order.
type DirectoryGroup struct {
	OperationGroupID string
	Mutations        []DirectoryMutation
	DesiredState     domain.StateFileV2
	// PostApplyVerify runs once every mutation in the group has been applied and
	// individually verified, and before the group's state commit decision. It
	// exists for verification that depends on the whole group's restored
	// filesystem state at once (for example, a native client registry that must
	// observe every reconstructed path together before any single one can be
	// confirmed). A returned error rolls back every applied mutation in the
	// group; no state or native side effect from this group is ever committed.
	PostApplyVerify func(context.Context) error
}

type DirectoryRemovalGroup struct {
	OperationGroupID string
	Removals         []DirectoryRemoval
	DesiredState     domain.StateFileV2
}

type appliedGroupMutation struct {
	receipt dirswap.Receipt
	state   domain.MutationReceipt
}

type Kernel struct {
	StateStore StateStore
	Directory  dirswap.Manager
}

func (kernel Kernel) ApplyDirectory(ctx context.Context, mutation DirectoryMutation) (domain.MutationReceipt, error) {
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
	directoryReceipt, err := kernel.Directory.Apply(ctx, dirswap.Input{
		OperationID: mutation.OperationID, ClientBindingID: mutation.ClientBindingID, Sequence: mutation.Sequence,
		OwnedBase: mutation.OwnedBase, ActivePath: mutation.ActivePath, StagingPath: mutation.StagingPath,
		RequireAbsent: mutation.RequireAbsent,
	})
	if err != nil {
		if directoryReceipt.OperationID != "" {
			_ = kernel.Directory.Rollback(context.Background(), directoryReceipt)
		}
		return domain.MutationReceipt{}, err
	}
	rollback := func() { _ = kernel.Directory.Rollback(context.Background(), directoryReceipt) }
	if mutation.Verify != nil {
		if err := mutation.Verify(ctx, directoryReceipt.ActivePath); err != nil {
			rollback()
			return domain.MutationReceipt{}, fmt.Errorf("verify activated directory: %w", err)
		}
	}
	receipt := domain.MutationReceipt{
		OperationID:      mutation.OperationID,
		OperationGroupID: mutation.OperationID,
		Sequence:         mutation.Sequence,
		MutationType:     "directory_swap",
		ClientBindingID:  mutation.ClientBindingID,
		ActivePath:       directoryReceipt.ActivePath,
		StagingPath:      directoryReceipt.StagingPath,
		BackupPath:       directoryReceipt.BackupPath,
		BeforeDigest:     mutation.BeforeDigest,
		AfterDigest:      mutation.AfterDigest,
		Phase:            ReceiptPhaseStateCommitted,
	}
	client.Receipts = append(client.Receipts, receipt)
	client.NativeObjects = append([]domain.NativeObjectOwnership(nil), mutation.NativeObjects...)
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
				return domain.MutationReceipt{}, fmt.Errorf("commit transaction state: %v; rollback failed: %w", err, rollbackErr)
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
			if err := kernel.Directory.Rollback(context.Background(), applied[index].receipt); err != nil && rollbackErr == nil {
				rollbackErr = err
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
		directoryReceipt, err := kernel.Directory.Apply(ctx, dirswap.Input{
			OperationID: mutation.OperationID, ClientBindingID: mutation.ClientBindingID, Sequence: mutation.Sequence,
			OwnedBase: mutation.OwnedBase, ActivePath: mutation.ActivePath, StagingPath: mutation.StagingPath,
			RequireAbsent: mutation.RequireAbsent,
		})
		if err != nil {
			if directoryReceipt.OperationID != "" {
				applied = append(applied, appliedGroupMutation{receipt: directoryReceipt})
			}
			if rollbackErr := rollback(); rollbackErr != nil {
				return nil, groupError(GroupFailureUnknown, fmt.Errorf("apply grouped directory mutation: %v; rollback failed: %w", err, rollbackErr))
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
					return nil, groupError(GroupFailureUnknown, fmt.Errorf("verify grouped directory mutation: %v; rollback failed: %w", err, rollbackErr))
				}
				return nil, groupError(GroupFailureRolledBack, fmt.Errorf("verify grouped directory mutation: %w", err))
			}
		}
		receipt := domain.MutationReceipt{OperationID: mutation.OperationID, OperationGroupID: group.OperationGroupID,
			Sequence: mutation.Sequence, MutationType: "directory_swap", ClientBindingID: mutation.ClientBindingID,
			ActivePath: directoryReceipt.ActivePath, StagingPath: directoryReceipt.StagingPath, BackupPath: directoryReceipt.BackupPath,
			BeforeDigest: mutation.BeforeDigest, AfterDigest: mutation.AfterDigest, Phase: ReceiptPhaseStateCommitted}
		client.Receipts = append(client.Receipts, receipt)
		client.NativeObjects = append([]domain.NativeObjectOwnership(nil), mutation.NativeObjects...)
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
				return nil, groupError(GroupFailureUnknown, fmt.Errorf("post-apply group verification: %v; rollback failed: %w", err, rollbackErr))
			}
			return nil, groupError(GroupFailureRolledBack, fmt.Errorf("post-apply group verification: %w", err))
		}
	}
	if oldState, err := kernel.persistCommitDecision(state, beforeJSON); err != nil {
		if oldState {
			if rollbackErr := rollback(); rollbackErr != nil {
				return nil, groupError(GroupFailureUnknown, fmt.Errorf("commit grouped transaction state: %v; rollback failed: %w", err, rollbackErr))
			}
			return nil, groupError(GroupFailureRolledBack, fmt.Errorf("commit grouped transaction state: %w", err))
		}
		if !oldState {
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

func groupReceipts(items []appliedGroupMutation) []domain.MutationReceipt {
	result := make([]domain.MutationReceipt, 0, len(items))
	for _, item := range items {
		if item.state.OperationID != "" {
			result = append(result, item.state)
		}
	}
	return result
}

func (kernel Kernel) RemoveDirectory(ctx context.Context, removal DirectoryRemoval) (domain.MutationReceipt, error) {
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
	directoryReceipt, err := kernel.Directory.Apply(ctx, dirswap.Input{
		OperationID: removal.OperationID, ClientBindingID: removal.ClientBindingID, Sequence: removal.Sequence,
		OwnedBase: removal.OwnedBase, ActivePath: removal.ActivePath, Remove: true,
	})
	if err != nil {
		if directoryReceipt.OperationID != "" {
			_ = kernel.Directory.Rollback(context.Background(), directoryReceipt)
		}
		return domain.MutationReceipt{}, err
	}
	receipt := domain.MutationReceipt{
		OperationID:      removal.OperationID,
		OperationGroupID: firstNonEmpty(removal.OperationGroupID, removal.OperationID),
		Sequence:         removal.Sequence,
		MutationType:     "directory_remove",
		ClientBindingID:  removal.ClientBindingID,
		ActivePath:       directoryReceipt.ActivePath,
		BackupPath:       directoryReceipt.BackupPath,
		BeforeDigest:     removal.BeforeDigest,
		Phase:            ReceiptPhaseStateCommitted,
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
				return domain.MutationReceipt{}, fmt.Errorf("commit removal state: %v; rollback failed: %w", err, rollbackErr)
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
			if err := kernel.Directory.Rollback(context.Background(), applied[index].receipt); err != nil && result == nil {
				result = err
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
		directoryReceipt, err := kernel.Directory.Apply(ctx, dirswap.Input{OperationID: removal.OperationID, ClientBindingID: removal.ClientBindingID,
			Sequence: removal.Sequence, OwnedBase: removal.OwnedBase, ActivePath: removal.ActivePath, Remove: true})
		if err != nil {
			if directoryReceipt.OperationID != "" {
				applied = append(applied, appliedGroupMutation{receipt: directoryReceipt})
			}
			if rollbackErr := rollback(); rollbackErr != nil {
				return nil, groupError(GroupFailureUnknown, fmt.Errorf("apply grouped removal: %v; rollback failed: %w", err, rollbackErr))
			}
			if len(applied) > 0 {
				return nil, groupError(GroupFailureRolledBack, err)
			}
			return nil, groupError(GroupFailureUnchanged, err)
		}
		receipt := domain.MutationReceipt{OperationID: removal.OperationID, OperationGroupID: group.OperationGroupID,
			Sequence: removal.Sequence, MutationType: "directory_remove", ClientBindingID: removal.ClientBindingID,
			ActivePath: directoryReceipt.ActivePath, BackupPath: directoryReceipt.BackupPath, BeforeDigest: removal.BeforeDigest,
			Phase: ReceiptPhaseStateCommitted}
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
	if old, err := kernel.persistCommitDecision(state, beforeJSON); err != nil {
		if old {
			if rollbackErr := rollback(); rollbackErr != nil {
				return nil, groupError(GroupFailureUnknown, fmt.Errorf("commit grouped removal: %v; rollback failed: %w", err, rollbackErr))
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
	open, err := kernel.Directory.ListOpen()
	if err != nil {
		return err
	}
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
		return kernel.StateStore.Save(state)
	}
	return nil
}

// persistCommitDecision handles the ambiguous failure window where an atomic
// state rename is visible but syncing its parent directory reports an error.
// The caller may roll back only when a reload proves the exact old state is
// still authoritative. A visible desired state is re-saved successfully before
// the native directory transaction is allowed to commit. All other outcomes
// leave the directory journal and backup intact for recovery.
func (kernel Kernel) persistCommitDecision(desired domain.StateFileV2, beforeJSON []byte) (oldState bool, err error) {
	if err := kernel.StateStore.Save(desired); err == nil {
		return false, nil
	} else {
		initialErr := err
		observed, loadErr := kernel.StateStore.Load()
		if loadErr != nil {
			return false, fmt.Errorf("state save failed (%v) and commit visibility is unknown: %w", initialErr, loadErr)
		}
		observedJSON, marshalErr := marshalComparableState(observed)
		if marshalErr != nil {
			return false, fmt.Errorf("state save failed (%v) and observed state cannot be compared: %w", initialErr, marshalErr)
		}
		desiredJSON, marshalErr := marshalComparableState(desired)
		if marshalErr != nil {
			return false, fmt.Errorf("state save failed (%v) and desired state cannot be compared: %w", initialErr, marshalErr)
		}
		if bytes.Equal(observedJSON, desiredJSON) {
			if retryErr := kernel.StateStore.Save(desired); retryErr != nil {
				return false, fmt.Errorf("state became visible after save error (%v), but durability retry failed: %w", initialErr, retryErr)
			}
			return false, nil
		}
		if bytes.Equal(observedJSON, beforeJSON) {
			return true, initialErr
		}
		return false, fmt.Errorf("state save failed and reload matched neither exact old nor desired state: %w", initialErr)
	}
}

func marshalComparableState(state domain.StateFileV2) ([]byte, error) {
	state.Installations = append([]domain.Installation(nil), state.Installations...)
	sort.Slice(state.Installations, func(i, j int) bool {
		return state.Installations[i].InstallationID < state.Installations[j].InstallationID
	})
	return json.Marshal(state)
}

func loadClientFromState(state domain.StateFileV2, installationID, clientBindingID string) (int, domain.ClientBinding, error) {
	if state.SchemaVersion != domain.StateSchemaVersion {
		return -1, domain.ClientBinding{}, fmt.Errorf("desired transaction state schema_version must be %d", domain.StateSchemaVersion)
	}
	for index, installation := range state.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		client, ok := installation.Clients[clientBindingID]
		if !ok {
			return -1, domain.ClientBinding{}, fmt.Errorf("client binding %q not found in desired transaction state", clientBindingID)
		}
		return index, client, nil
	}
	return -1, domain.ClientBinding{}, fmt.Errorf("installation %q not found in desired transaction state", installationID)
}

func (kernel Kernel) loadClient(installationID, clientBindingID string) (domain.StateFileV2, int, domain.ClientBinding, error) {
	state, err := kernel.StateStore.Load()
	if err != nil {
		return domain.StateFileV2{}, -1, domain.ClientBinding{}, err
	}
	for index, installation := range state.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		client, ok := installation.Clients[clientBindingID]
		if !ok {
			return domain.StateFileV2{}, -1, domain.ClientBinding{}, fmt.Errorf("client binding %q not found", clientBindingID)
		}
		return state, index, client, nil
	}
	return domain.StateFileV2{}, -1, domain.ClientBinding{}, fmt.Errorf("installation %q not found", installationID)
}

func replaceReceipt(client domain.ClientBinding, receipt domain.MutationReceipt) domain.ClientBinding {
	for index := range client.Receipts {
		if client.Receipts[index].OperationID == receipt.OperationID {
			client.Receipts[index] = receipt
			return client
		}
	}
	client.Receipts = append(client.Receipts, receipt)
	return client
}

func findReceipt(state domain.StateFileV2, directory dirswap.Receipt) (int, string, int, bool) {
	mutationType := "directory_swap"
	if directory.Operation == dirswap.OperationRemove {
		mutationType = "directory_remove"
	}
	for receiptIndex, receipt := range state.TransactionReceipts {
		if receipt.OperationID == directory.OperationID && receipt.ClientBindingID == directory.ClientBindingID &&
			receipt.Sequence == directory.Sequence && receipt.MutationType == mutationType &&
			receipt.ActivePath == directory.ActivePath && receipt.StagingPath == directory.StagingPath && receipt.BackupPath == directory.BackupPath &&
			(receipt.Phase == ReceiptPhaseStateCommitted || receipt.Phase == ReceiptPhaseCommitted) {
			return -1, "", receiptIndex, true
		}
	}
	for installationIndex, installation := range state.Installations {
		for clientKey, client := range installation.Clients {
			for receiptIndex, receipt := range client.Receipts {
				if receipt.OperationID == directory.OperationID &&
					receipt.ClientBindingID == directory.ClientBindingID && clientKey == directory.ClientBindingID &&
					receipt.Sequence == directory.Sequence && receipt.MutationType == mutationType &&
					receipt.ActivePath == directory.ActivePath && receipt.StagingPath == directory.StagingPath && receipt.BackupPath == directory.BackupPath &&
					(receipt.Phase == ReceiptPhaseStateCommitted || receipt.Phase == ReceiptPhaseCommitted) {
					return installationIndex, clientKey, receiptIndex, true
				}
			}
		}
	}
	return -1, "", -1, false
}

func operationIDExists(state domain.StateFileV2, operationID string) bool {
	for _, receipt := range state.TransactionReceipts {
		if receipt.OperationID == operationID {
			return true
		}
	}
	for _, installation := range state.Installations {
		for _, client := range installation.Clients {
			for _, receipt := range client.Receipts {
				if receipt.OperationID == operationID {
					return true
				}
			}
		}
	}
	return false
}

func requireMutationReady(store StateStore) error {
	if guard, ok := store.(interface{ RequireMutationReady() error }); ok {
		return guard.RequireMutationReady()
	}
	return nil
}

func operationGroupIDExists(state domain.StateFileV2, operationGroupID string) bool {
	for _, receipt := range state.TransactionReceipts {
		if receipt.OperationGroupID == operationGroupID {
			return true
		}
	}
	for _, installation := range state.Installations {
		if installation.OperationGroupID == operationGroupID {
			return true
		}
		for _, client := range installation.Clients {
			for _, receipt := range client.Receipts {
				if receipt.OperationGroupID == operationGroupID {
					return true
				}
			}
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
