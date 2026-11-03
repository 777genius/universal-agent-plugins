package usecase

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type GroupInput struct {
	Targets             []AddInput
	CompatibilityChecks []AddInput
	OperationGroupID    string
	Progress            func(GroupProgressEvent)
	OnlinePreview       bool
	DryRun              bool
	Confirmed           bool
	Switch              bool
	Repair              bool
}

type GroupProgressPhase string

const (
	GroupProgressPreparing   GroupProgressPhase = "preparing"
	GroupProgressConfiguring GroupProgressPhase = "configuring"
	GroupProgressConfigured  GroupProgressPhase = "configured"
	GroupProgressActivating  GroupProgressPhase = "activating"
	GroupProgressActivated   GroupProgressPhase = "activated"
)

// GroupProgressEvent reports observed checkpoints; it never grants consent or
// changes the outcome of an installation.
type GroupProgressEvent struct {
	ClientID domain.ClientID
	Phase    GroupProgressPhase
	Result   AddResult
}

type GroupResult struct {
	// PreparedClients carries immutable host snapshots to this operation's apply.
	PreparedClients  []domain.DetectedClient   `json:"-"`
	InstallationID   string                    `json:"installation_id"`
	OperationGroupID string                    `json:"operation_group_id,omitempty"`
	Targets          []AddResult               `json:"targets"`
	PluginData       domain.PluginDataDecision `json:"plugin_data"`
	Receipts         []domain.MutationReceipt  `json:"-"`
	Mutated          bool                      `json:"mutated"`
	Phase            GroupPhase                `json:"phase"`
}

type GroupPhase string

const (
	GroupPhasePlanned                 GroupPhase = "planned"
	GroupPhasePreparationFailed       GroupPhase = "preparation_failed"
	GroupPhaseManagedUnchanged        GroupPhase = "managed_unchanged"
	GroupPhaseManagedRolledBack       GroupPhase = "managed_rolled_back"
	GroupPhaseManagedCommitUnknown    GroupPhase = "managed_commit_unknown"
	GroupPhaseManagedCommitted        GroupPhase = "managed_committed"
	GroupPhaseManagedActivationFailed GroupPhase = "managed_committed_activation_failed"
	GroupPhaseExternalPartialFailure  GroupPhase = "external_partial_failure"
	GroupPhaseCompleted               GroupPhase = "completed"
)

type GroupTargetPhase string

const (
	GroupTargetPlanned              GroupTargetPhase = "planned"
	GroupTargetManagedRolledBack    GroupTargetPhase = "managed_rolled_back"
	GroupTargetManagedCommitted     GroupTargetPhase = "managed_committed"
	GroupTargetManagedUnknown       GroupTargetPhase = "managed_commit_unknown"
	GroupTargetExternalCompleted    GroupTargetPhase = "external_completed"
	GroupTargetExternalFailed       GroupTargetPhase = "external_failed"
	GroupTargetExternalPartial      GroupTargetPhase = "external_completed_managed_incomplete"
	GroupTargetExternalNotAttempted GroupTargetPhase = "external_not_attempted"
)

// GroupTargetFailure records why one logical surface failed or was skipped
// during external activation. Retry commands belong in CLI presentation.
type GroupTargetFailure struct {
	Stage   string `json:"stage"`
	Message string `json:"message"`
}
