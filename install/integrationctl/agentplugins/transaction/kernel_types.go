package transaction

// Core transaction types and durable operation states.

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
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
	VerifyBefore         func(context.Context, string) error
	EnvironmentAuthority *domain.EnvironmentAuthority
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
	Standalone           bool
	Verify               func(context.Context, string) error
	EnvironmentAuthority *domain.EnvironmentAuthority
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
	NativeRecovery    ports.NativeTransitionRecovery
	StateStore        StateStore
	Directory         dirswap.Manager
}
