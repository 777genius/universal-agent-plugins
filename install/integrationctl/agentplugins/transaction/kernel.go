package transaction

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/profileauthority"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
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
	// VerifyBefore proves caller ownership at the active and moved backup paths.
	VerifyBefore func(context.Context, string) error
}

type DirectoryRemoval struct {
	DataReceiptID    string
	ProfileOwners    []domain.PhysicalProfileOwner
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
	Namespace         string
	PhysicalAuthority ports.PhysicalProfileAuthority
	StateStore        StateStore
	Directory         dirswap.Manager
}

// A directory commit confirms only the managed package bytes. External native
// receipts remain the last confirmed effect until the client adapter reports
// its own result after this transaction.
func committedDirectoryObjects(before domain.StateFileV2, installationID, bindingID string, projected []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	var objects []domain.NativeObjectOwnership
	for _, object := range projected {
		if object.Kind == "managed_package_directory" {
			objects = append(objects, object)
		}
	}
	for _, installation := range before.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		for _, object := range installation.Clients[bindingID].NativeObjects {
			if object.Kind != "managed_package_directory" {
				objects = append(objects, object)
			}
		}
		break
	}
	return objects
}

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
	directoryReceipt, err := kernel.Directory.Apply(ctx, dirswap.Input{ProfileOwners: neutralOwners(owners),
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
		directoryReceipt, err := kernel.Directory.Apply(ctx, dirswap.Input{ProfileOwners: neutralOwners(owners),
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
	directoryReceipt, err := kernel.Directory.Apply(ctx, dirswap.Input{ProfileOwners: neutralOwners(owners),
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
		OperationID:      removal.OperationID,
		OperationGroupID: firstNonEmpty(removal.OperationGroupID, removal.OperationID),
		Sequence:         removal.Sequence,
		MutationType:     "directory_remove",
		ClientBindingID:  directoryReceipt.ClientBindingID,
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
		directoryReceipt, err := kernel.Directory.Apply(ctx, dirswap.Input{ProfileOwners: neutralOwners(owners), DataReceiptID: removal.DataReceiptID, OperationID: removal.OperationID, ClientBindingID: dataBindingID(removal, owners),
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

// PersistStateDecision gives lifecycle state writes the same visibility and
// durability handling as directory commit decisions.
func (kernel Kernel) PersistStateDecision(before, desired domain.StateFileV2) error {
	beforeJSON, err := marshalComparableState(before)
	if err != nil {
		return err
	}
	_, err = kernel.persistCommitDecision(desired, beforeJSON)
	return err
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
		if sameDirectoryProof(receipt.DirectoryProof, directory) && sameReceiptOwners(receipt.ProfileOwners, directory.ProfileOwners) && receipt.DataReceiptID == directory.DataReceiptID && receipt.OperationID == directory.OperationID && receipt.ClientBindingID == directory.ClientBindingID &&
			receipt.Sequence == directory.Sequence && receipt.MutationType == mutationType &&
			receipt.ActivePath == directory.ActivePath && receipt.StagingPath == directory.StagingPath && receipt.BackupPath == directory.BackupPath &&
			(receipt.Phase == ReceiptPhaseStateCommitted || receipt.Phase == ReceiptPhaseCommitted) {
			return -1, "", receiptIndex, true
		}
	}
	for installationIndex, installation := range state.Installations {
		for clientKey, client := range installation.Clients {
			for receiptIndex, receipt := range client.Receipts {
				if sameDirectoryProof(receipt.DirectoryProof, directory) && sameReceiptOwners(receipt.ProfileOwners, directory.ProfileOwners) && receipt.DataReceiptID == directory.DataReceiptID && receipt.OperationID == directory.OperationID &&
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

func copyOwners(owners []domain.PhysicalProfileOwner) []domain.PhysicalProfileOwner {
	out := append([]domain.PhysicalProfileOwner(nil), owners...)
	for i := range out {
		out[i].Authority = domain.CloneProfileAuthority(out[i].Authority)
	}
	return out
}
func neutralOwners(owners []domain.PhysicalProfileOwner) []dirswap.ProfileOwner {
	out := make([]dirswap.ProfileOwner, 0, len(owners))
	for _, o := range owners {
		token, err := profileauthority.ToNeutral(*o.Authority)
		if err != nil {
			panic("validated physical owner cannot convert")
		}
		out = append(out, dirswap.ProfileOwner{Namespace: o.Namespace, InstallationID: o.InstallationID, ClientID: o.ClientID, ClientBindingID: o.ClientBindingID, Authority: &token})
	}
	return out
}
func sameReceiptOwners(owners []domain.PhysicalProfileOwner, journal []dirswap.ProfileOwner) bool {
	if len(owners) != len(journal) {
		return false
	}
	for i, o := range owners {
		j := journal[i]
		if o.Namespace != j.Namespace || o.InstallationID != j.InstallationID || o.ClientID != j.ClientID || o.ClientBindingID != j.ClientBindingID || o.Authority == nil || j.Authority == nil {
			return false
		}
		token, err := profileauthority.FromNeutral(*j.Authority)
		if err != nil || !token.Equal(*o.Authority) {
			return false
		}
	}
	return true
}
func (kernel Kernel) validateOwners(ctx context.Context, owners []domain.PhysicalProfileOwner) error {
	if len(owners) > 256 {
		return fmt.Errorf("profile owner scope limit")
	}
	seen := map[[4]string]bool{}
	for _, o := range owners {
		key := [4]string{o.Namespace, o.InstallationID, o.ClientID, o.ClientBindingID}
		if kernel.Namespace == "" || o.Namespace != kernel.Namespace || o.InstallationID == "" || o.ClientID == "" || o.ClientBindingID == "" || o.Authority == nil || o.Authority.IsZero() || seen[key] {
			return fmt.Errorf("invalid exact physical profile scope")
		}
		seen[key] = true
		if kernel.PhysicalAuthority == nil {
			return fmt.Errorf("physical profile verifier is required")
		}
		if err := kernel.PhysicalAuthority.RevalidateProfileAuthority(ctx, domain.ClientID(o.ClientID), *o.Authority); err != nil {
			return err
		}
	}
	return nil
}
func (kernel Kernel) bindingOwner(ctx context.Context, installationID, key string, b domain.ClientBinding) ([]domain.PhysicalProfileOwner, error) {
	if b.ProfileAuthority == nil {
		if kernel.PhysicalAuthority != nil {
			if err := kernel.PhysicalAuthority.RevalidateProfileAuthority(ctx, domain.ClientID(b.ClientID), domain.ProfileAuthority{}); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}
	if key != b.ClientBindingID || b.ProfileAuthority.IsZero() || b.NativeProfileRoot != b.ProfileAuthority.Facts().CanonicalRoot {
		return nil, fmt.Errorf("physical binding key or root differs from owner")
	}
	return []domain.PhysicalProfileOwner{{Namespace: b.ProfileNamespace, InstallationID: installationID, ClientID: b.ClientID, ClientBindingID: key, Authority: domain.CloneProfileAuthority(b.ProfileAuthority)}}, nil
}
func (kernel Kernel) mutationOwners(ctx context.Context, state domain.StateFileV2, mutations []DirectoryMutation) ([]domain.PhysicalProfileOwner, error) {
	before, err := kernel.StateStore.Load()
	if err != nil {
		return nil, err
	}
	var owners []domain.PhysicalProfileOwner
	for _, m := range mutations {
		_, b, err := loadClientFromState(state, m.InstallationID, m.ClientBindingID)
		if err != nil {
			return nil, err
		}
		if _, old, err := loadClientFromState(before, m.InstallationID, m.ClientBindingID); err == nil && (old.ProfileAuthority != nil || b.ProfileAuthority != nil) {
			if old.ClientID != b.ClientID || old.ProfileNamespace != b.ProfileNamespace || !domain.SameProfileAuthority(old.ProfileAuthority, b.ProfileAuthority) {
				return nil, fmt.Errorf("candidate differs from recorded physical owner")
			}
		}
		scope, err := kernel.bindingOwner(ctx, m.InstallationID, m.ClientBindingID, b)
		if err != nil {
			return nil, err
		}
		owners = append(owners, scope...)
		if b.DataReceiptID != "" {
			for _, installation := range state.Installations {
				if installation.InstallationID != m.InstallationID {
					continue
				}
				for key, peer := range installation.Clients {
					if key == m.ClientBindingID || peer.DataReceiptID != b.DataReceiptID {
						continue
					}
					relevant, err := kernel.bindingOwner(ctx, installation.InstallationID, key, peer)
					if err != nil {
						return nil, err
					}
					owners = append(owners, relevant...)
				}
			}
		}
	}
	unique := map[[4]string]domain.PhysicalProfileOwner{}
	for _, owner := range owners {
		unique[[4]string{owner.Namespace, owner.InstallationID, owner.ClientID, owner.ClientBindingID}] = owner
	}
	owners = nil
	for _, owner := range unique {
		owners = append(owners, owner)
	}
	sort.Slice(owners, func(i, j int) bool { return owners[i].ClientBindingID < owners[j].ClientBindingID })
	return owners, kernel.validateOwners(ctx, owners)
}
func (kernel Kernel) removalOwners(ctx context.Context, removals []DirectoryRemoval) ([]domain.PhysicalProfileOwner, error) {
	if kernel.StateStore == nil {
		return nil, fmt.Errorf("transaction state store is required")
	}
	state, err := kernel.StateStore.Load()
	if err != nil {
		return nil, err
	}
	var owners []domain.PhysicalProfileOwner
	seen := map[[4]string]bool{}
	add := func(items []domain.PhysicalProfileOwner) {
		for _, o := range items {
			key := [4]string{o.Namespace, o.InstallationID, o.ClientID, o.ClientBindingID}
			if !seen[key] {
				seen[key] = true
				owners = append(owners, o)
			}
		}
	}
	for _, r := range removals {
		for _, o := range r.ProfileOwners {
			_, b, err := loadClientFromState(state, o.InstallationID, o.ClientBindingID)
			if err != nil || o.Namespace != b.ProfileNamespace || o.ClientID != b.ClientID || !domain.SameProfileAuthority(o.Authority, b.ProfileAuthority) {
				return nil, fmt.Errorf("removal scope differs from recorded owner")
			}
		}
		add(copyOwners(r.ProfileOwners))
		for _, installation := range state.Installations {
			if installation.InstallationID != r.InstallationID {
				continue
			}
			for key, b := range installation.Clients {
				if (r.DataReceiptID == "" && key != r.ClientBindingID) || (r.DataReceiptID != "" && b.DataReceiptID != r.DataReceiptID) {
					continue
				}
				scope, err := kernel.bindingOwner(ctx, installation.InstallationID, key, b)
				if err != nil {
					return nil, err
				}
				add(scope)
			}
		}
	}
	sort.Slice(owners, func(i, j int) bool { return owners[i].ClientBindingID < owners[j].ClientBindingID })
	return owners, kernel.validateOwners(ctx, owners)
}

type guardedStateStore struct {
	StateStore
	check func() error
}

func (s guardedStateStore) Save(state domain.StateFileV2) error {
	if err := s.check(); err != nil {
		return err
	}
	return s.StateStore.Save(state)
}
func (s guardedStateStore) RequireMutationReady() error { return requireMutationReady(s.StateStore) }
func (kernel Kernel) guardOwners(ctx context.Context, owners []domain.PhysicalProfileOwner) Kernel {
	frozen := copyOwners(owners)
	kernel.Directory.Namespace = kernel.Namespace
	kernel.Directory.RequiredOwners = neutralOwners(frozen)
	if len(frozen) > 0 {
		kernel.Directory.CheckOpen = true
		base := kernel
		kernel.StateStore = guardedStateStore{StateStore: kernel.StateStore, check: func() error {
			if err := base.validateOwners(ctx, frozen); err != nil {
				return err
			}
			return base.PrevalidateRecovery(ctx)
		}}
	}
	return kernel
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
		if j.SchemaVersion == 5 {
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

// PrevalidateRecovery is inert and covers the entire pending durability decision.
func (kernel Kernel) PrevalidateRecovery(ctx context.Context) error {
	if kernel.StateStore == nil {
		return fmt.Errorf("transaction state store is required")
	}
	state, err := kernel.StateStore.Load()
	if err != nil {
		return err
	}
	if kernel.Directory.JournalDir == "" {
		owners, err := kernel.pendingOwners(ctx, state, nil)
		if err != nil {
			return err
		}
		if len(owners) != 0 || len(kernel.Directory.RequiredOwners) != 0 {
			return fmt.Errorf("physical recovery journal dir is required")
		}
		for _, installation := range state.Installations {
			for _, binding := range installation.Clients {
				if binding.ProfileAuthority != nil {
					return fmt.Errorf("physical recovery journal dir is required")
				}
			}
		}
		return nil
	}
	kernel.Directory.Namespace = kernel.Namespace
	open, err := kernel.Directory.ListOpen()
	if err != nil {
		return err
	}
	_, err = kernel.pendingOwners(ctx, state, open)
	return err
}

func dataBindingID(r DirectoryRemoval, owners []domain.PhysicalProfileOwner) string {
	if r.DataReceiptID != "" && len(owners) > 0 {
		return ""
	}
	return r.ClientBindingID
}

func directoryProof(r dirswap.Receipt) json.RawMessage {
	if r.SchemaVersion != 5 {
		return nil
	}
	raw, _ := json.Marshal(r)
	return raw
}
func sameDirectoryProof(raw json.RawMessage, r dirswap.Receipt) bool {
	if r.SchemaVersion != 5 {
		return len(raw) == 0
	}
	var recorded dirswap.Receipt
	if json.Unmarshal(raw, &recorded) != nil {
		return false
	}
	recorded.Phase = r.Phase
	a, _ := json.Marshal(recorded)
	b, _ := json.Marshal(r)
	return bytes.Equal(a, b)
}
func (kernel Kernel) verifyReceiptProof(r domain.MutationReceipt) error {
	if len(r.ProfileOwners) == 0 {
		if len(r.DirectoryProof) != 0 {
			return fmt.Errorf("physical proof lost owner scope")
		}
		return nil
	}
	var proof dirswap.Receipt
	if len(r.DirectoryProof) == 0 || json.Unmarshal(r.DirectoryProof, &proof) != nil || proof.SchemaVersion != 5 || proof.OperationID != r.OperationID || proof.Sequence != r.Sequence || proof.ClientBindingID != r.ClientBindingID || proof.DataReceiptID != r.DataReceiptID || proof.ActivePath != r.ActivePath || proof.StagingPath != r.StagingPath || proof.BackupPath != r.BackupPath || !sameReceiptOwners(r.ProfileOwners, proof.ProfileOwners) {
		return fmt.Errorf("pending receipt physical role proof is incomplete")
	}
	manager := kernel.Directory
	manager.Namespace = kernel.Namespace
	current, err := manager.Load(r.OperationID)
	if err == nil {
		if !sameDirectoryProof(r.DirectoryProof, current) {
			return fmt.Errorf("pending receipt and current physical journal disagree")
		}
		proof = current
	} else if os.IsNotExist(err) {
		// The state commit can outlive journal removal. Verify final roles.
		proof.Phase = dirswap.PhaseCommitted
	} else {
		return err
	}
	return manager.VerifyRecorded(proof)
}
