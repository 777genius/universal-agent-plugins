package usecase

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type GroupInput struct {
	Targets             []AddInput
	CompatibilityChecks []AddInput
	OperationGroupID    string
	DryRun              bool
	Confirmed           bool
	Switch              bool
	Repair              bool
}

type GroupResult struct {
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

func (service Service) AddGroup(ctx context.Context, input GroupInput) (GroupResult, error) {
	return service.applyGroup(ctx, input, false)
}

func (service Service) UpdateGroup(ctx context.Context, input GroupInput) (GroupResult, error) {
	return service.applyGroup(ctx, input, true)
}

func (service Service) SwitchGroup(ctx context.Context, input GroupInput) (GroupResult, error) {
	input.Switch = true
	return service.applyGroup(ctx, input, true)
}

func (service Service) RepairGroup(ctx context.Context, input GroupInput) (GroupResult, error) {
	if service.StateStore == nil {
		return GroupResult{}, fmt.Errorf("state store is required")
	}
	state, err := service.StateStore.Load()
	if err != nil {
		return GroupResult{}, err
	}
	for _, target := range input.Targets {
		index, existing, _, err := findStickyInstallation(state, target, domain.ComputeSourceBindingID(target.Envelope.Source))
		if err != nil || !existing {
			if err != nil {
				return GroupResult{}, err
			}
			return GroupResult{}, fmt.Errorf("repair source is not installed")
		}
		installation := state.Installations[index]
		matched := false
		for _, client := range installation.Clients {
			if !sameNativeBackend(domain.ClientID(client.ClientID), target.Client.ClientID) || client.Scope != string(target.Scope) {
				continue
			}
			if !repairPackageRevisionMatches(client.PackageRevision, target.Envelope, installation.OriginMode == domain.OriginModeDirectory) {
				return GroupResult{}, fmt.Errorf("repair must use the exact applied package revision for %s", target.Client.ClientID)
			}
			if installation.OriginMode == domain.OriginModeDirectory && (target.DirectoryResolution == nil || client.PackageRevision.DistributionID != target.DirectoryResolution.DistributionID || client.PackageRevision.ReleaseSequence != target.DirectoryResolution.DesiredReleaseSequence) {
				return GroupResult{}, fmt.Errorf("repair must use the exact applied Directory release")
			}
			matched = true
			break
		}
		if !matched {
			return GroupResult{}, fmt.Errorf("repair target %s is not installed", target.Client.ClientID)
		}
	}
	input.Repair = true
	return service.applyGroup(ctx, input, true)
}

type plannedGroupTarget struct {
	input           AddInput
	plan            domain.DeliveryPlan
	resultIndexes   []int
	clientBindingID string
	managed         *domain.ClientBinding
	delivery        domain.StagedDelivery
	dataReceipt     domain.DataReceipt
	dataCreated     bool
	noChange        bool
	// recovering marks a confirmed repair target whose managed native object is
	// positively absent from the filesystem, proven without ever running native
	// client discovery. Its full native identity is deferred until the group's
	// directories are restored and is verified once, together, before commit.
	recovering bool
}

func (service Service) applyGroup(ctx context.Context, input GroupInput, replace bool) (GroupResult, error) {
	session := &groupSession{service: service, ctx: ctx, input: input, replace: replace}
	if err := session.validateGroupInput(); err != nil {
		return GroupResult{}, err
	}
	if err := session.ensureGroupID(); err != nil {
		return GroupResult{}, err
	}
	release, err := service.beginMutation(ctx, input.DryRun, input.Confirmed)
	if err != nil {
		return GroupResult{}, err
	}
	if release != nil {
		defer func() { _ = release() }()
	}
	if err := session.resolveGroupInstallation(); err != nil {
		return GroupResult{}, err
	}
	if err := session.planGroupTargets(); err != nil {
		return session.result, err
	}
	if err := session.preflightCompatibleBindings(); err != nil {
		return session.result, err
	}
	if session.input.DryRun || !session.input.Confirmed {
		return session.result, nil
	}
	if err := session.stageGroupDeliveries(); err != nil {
		return session.result, err
	}
	defer session.cleanupStaged()
	if err := session.reobserveGroupIdentity(); err != nil {
		return session.result, err
	}
	session.buildDesiredGroupState()
	if err := session.applyGroupKernel(); err != nil {
		return session.result, err
	}
	return session.activateGroupTargets()
}

func groupTargetFailureFromActivation(err error, outcome domain.ActivationOutcome) *GroupTargetFailure {
	stage := "activation"
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		stage = "canceled"
	case outcome.Authentication == domain.AuthenticationFailed &&
		outcome.Activation != domain.ActivationFailed &&
		outcome.Verification != domain.VerificationFailed:
		stage = "authentication"
	case outcome.Verification == domain.VerificationFailed &&
		outcome.Activation != domain.ActivationFailed:
		// Providers often set VerificationFailed together with ActivationFailed.
		// Prefer activation unless verification is the only failed dimension.
		stage = "verification"
	}
	message := "activation failed"
	if err != nil {
		message = err.Error()
	}
	return &GroupTargetFailure{Stage: stage, Message: message}
}

func countNotAttempted(targets []AddResult) int {
	count := 0
	for _, target := range targets {
		if target.GroupPhase == GroupTargetExternalNotAttempted {
			count++
		}
	}
	return count
}

func preservedGroupAuthentication(planned, current domain.AuthenticationState) domain.AuthenticationState {
	if current == domain.AuthenticationComplete || planned == "" || planned == domain.AuthenticationNotChecked {
		return current
	}
	return planned
}

func switchPluginDataDecision(installation domain.Installation) domain.PluginDataDecision {
	decision := domain.PluginDataDecision{
		Disposition: domain.PluginDataNone, Ownership: domain.PluginDataOwnershipNone,
		Compatibility: domain.PluginDataCompatibilityNotApplicable,
	}
	if len(installation.DataReceipts) == 0 {
		return decision
	}
	decision.Disposition = domain.PluginDataRetained
	decision.Present = true
	decision.ReceiptCount = len(installation.DataReceipts)
	decision.Ownership = domain.PluginDataOwnershipOwned
	decision.Compatibility = domain.PluginDataCompatibilityNotProven
	decision.Warning = domain.PluginDataCompatibilityWarning
	for _, receipt := range installation.DataReceipts {
		if receipt.State != domain.DataReceiptOwned || receipt.OwnershipDigest == "" {
			decision.Ownership = domain.PluginDataOwnershipIndeterminate
			break
		}
	}
	return decision
}

func assignGroupReceipts(targets []AddResult, planned []plannedGroupTarget, receipts []domain.MutationReceipt) {
	byBinding := make(map[string]domain.MutationReceipt, len(receipts))
	for _, receipt := range receipts {
		byBinding[receipt.ClientBindingID] = receipt
	}
	for _, target := range planned {
		receipt, ok := byBinding[target.clientBindingID]
		if !ok {
			continue
		}
		for _, resultIndex := range target.resultIndexes {
			targets[resultIndex].Receipt = receipt
		}
	}
}

func containsSurface(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func uniqueSortedSurfaces(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func groupPackageUnchanged(binding domain.ClientBinding, input AddInput) bool {
	revision := binding.PackageRevision
	if !packageRevisionMatches(revision, input.Envelope) || revision.ResolvedRevision != input.Envelope.Source.ResolvedRevision {
		return false
	}
	if normalizedOriginMode(input.OriginMode) == domain.OriginModeDirectory {
		return input.DirectoryResolution != nil && revision.DistributionID == input.DirectoryResolution.DistributionID && revision.ReleaseSequence == input.DirectoryResolution.DesiredReleaseSequence
	}
	return true
}

// Repair is the one lifecycle operation where an owned native object may be
// absent: recreating that exact object is its purpose. A foreign or
// indeterminate object remains blocking, and an existing managed object still
// has to match its recorded ownership digest.
func (service Service) observeGroupNativeIdentity(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding, repair bool) error {
	if !repair || service.NativeObserver == nil {
		return service.observeNativeIdentity(ctx, client, plan, managed)
	}
	observation, err := service.NativeObserver.ObserveNativeIdentity(ctx, client, plan, managed)
	if err != nil {
		return fmt.Errorf("observe native identity for %s: %w", client.ClientID, err)
	}
	return validateGroupNativeIdentityObservation(observation, managed)
}

func (service Service) observeGroupPreparedIdentity(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding, repair bool) error {
	if !repair {
		return service.observePreparedIdentity(ctx, client, plan, managed)
	}
	observation, err := service.preparedIdentityObservation(ctx, client, plan, managed)
	if err != nil {
		return fmt.Errorf("observe prepared identity for %s: %w", client.ClientID, err)
	}
	return validateGroupNativeIdentityObservation(observation, managed)
}

func validateGroupNativeIdentityObservation(observation domain.NativeIdentityObservation, managed *domain.ClientBinding) error {
	if observation.State == domain.NativeIdentityAbsent && managed != nil {
		return nil
	}
	switch observation.State {
	case domain.NativeIdentityManaged:
		if managed == nil {
			return fmt.Errorf("native identity already exists without matching agentplugins ownership; automatic adoption is disabled")
		}
		expected := managedDigest(*managed)
		if expected == "" || observation.Digest == "" || expected != observation.Digest {
			return fmt.Errorf("native identity ownership digest is stale or does not match")
		}
		return nil
	case domain.NativeIdentityUnmanaged:
		return fmt.Errorf("native identity is unmanaged; automatic adoption is disabled")
	case domain.NativeIdentityIndeterminate:
		return fmt.Errorf("native identity ownership is indeterminate; refusing repair")
	case domain.NativeIdentityAbsent:
		return fmt.Errorf("native identity is absent without a recorded managed binding")
	default:
		return fmt.Errorf("native identity observer returned an unknown state")
	}
}

// observeGroupRecoveryEligibility determines whether a confirmed repair target
// qualifies for the absent-managed-object recovery path: an intact prior
// managed ownership receipt with a nonempty digest, a native observer capable
// of the later post-restoration verification, and the active path positively
// confirmed absent through filesystem-only observation. It deliberately never
// runs native client discovery, so a failed native registry command can never
// be mistaken for proof of absence. At its first call site (the per-target
// preflight loop) a false result simply falls through to the ordinary,
// CLI-inclusive identity check; at its second call site (the pre-commit
// recheck of an already-committed recovering target) a false result is a hard
// refusal instead, since the target's own recovery path was already chosen.
//
// This is deliberately restricted to clients that declare
// SupportsPreparedRecovery: Codex is the only one with evidence (run05) that
// its native registry command fails outright while the target it would report
// on is absent. Other clients' registry commands have not been shown to share
// that failure mode, so they keep going through the ordinary, immediate
// CLI-inclusive check.
func (service Service) observeGroupRecoveryEligibility(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) bool {
	if !domain.ClientTraitsFor(client.ClientID).SupportsPreparedRecovery || managed == nil || managedDigest(*managed) == "" || service.NativeObserver == nil {
		return false
	}
	observation, err := service.preparedIdentityObservation(ctx, client, plan, managed)
	if err != nil {
		return false
	}
	return observation.State == domain.NativeIdentityAbsent
}

// groupRecoveryPostApplyVerify builds the transaction.DirectoryGroup's
// PostApplyVerify hook for a group containing at least one recovering target,
// or returns nil when the group has none. It runs full, native-registry-
// inclusive identity discovery exactly once per recovering target, only after
// every directory in the group has already been restored, and requires the
// client to now confirm exact managed ownership reconciled against the same
// recorded digest. A returned error rolls the whole group back before any
// state commit or external activation.
//
// Known residual limitations, not fixed by this contract:
//
//   - Absent swaps use exclusive publication and persisted root identity/tree
//     fingerprints for rollback and recovery. Changed or replaced provisional
//     objects retain their journal and fail closed. A noncooperating writer
//     changing the quarantine after its final fingerprint check and before
//     removal remains an unresolved filesystem concurrency limitation.
//   - Codex's own registry command is global across every installation's
//     marketplace entries in its config.toml, not scoped to one installation:
//     one installation has at most one Codex client binding, so a single
//     RepairGroup call can never itself carry two recovering targets, but a
//     wiped managed root can still leave two separate installations each with
//     their own absent Codex-owned directory. Repairing installation A
//     restores its directory, but this hook's `codex plugin list --json` call
//     still fails because installation B's marketplace entry is missing, so
//     the repair of A rolls back too; repairing B is symmetric. Neither gets
//     fixed until an operator restores or de-registers the other missing
//     source. This is reachable, not unreached -- but it fails closed (a
//     rollback, never silent adoption) and is no worse than before this fix
//     (a preflight refusal instead of a rollback), so it is left as a
//     documented limitation rather than solved here.
func (service Service) groupRecoveryPostApplyVerify(planned []plannedGroupTarget) func(context.Context) error {
	recovering := make([]plannedGroupTarget, 0, len(planned))
	for _, target := range planned {
		if target.recovering {
			recovering = append(recovering, target)
		}
	}
	if len(recovering) == 0 {
		return nil
	}
	return func(ctx context.Context) error {
		for _, target := range recovering {
			// Eligibility already required a non-nil NativeObserver; recovering
			// can only be true when that held at preflight time.
			observation, err := service.NativeObserver.ObserveNativeIdentity(ctx, target.input.Client, target.plan, target.managed)
			if err != nil {
				return fmt.Errorf("verify restored native identity for %s: %w", target.input.Client.ClientID, err)
			}
			if err := validateGroupRecoveryVerification(observation, target.managed); err != nil {
				return fmt.Errorf("restored native identity for %s: %w", target.input.Client.ClientID, err)
			}
		}
		return nil
	}
}

// validateGroupRecoveryVerification requires the same managed-identity proof
// the ordinary repair gate (validateGroupNativeIdentityObservation) already
// accepts as sufficient: a Managed finding whose digest matches the recorded
// receipt. Unlike the ordinary gate, an absent finding is no longer
// acceptable here -- the entire purpose of this check is confirming the
// reconstructed directory's digest, not merely its absence.
//
// Known residual gap, shared with every other Managed check in this
// codebase (not unique to recovery): a Managed finding with a matching digest
// can, for a real provider, be reached through the filesystem/digest fallback
// in observeIdentity even when the native registry itself reported the
// package as not found (registryClear) rather than confirming it -- see
// NativeIdentityObservation.NativeDiscoveryReconciled. Requiring that field
// here was tried and reverted: it is not part of the documented
// NativeIdentityObserver contract, so any other implementation of that
// interface (including this package's own test fakes and this repo's shared
// CLI test fixture) can validly omit it, and requiring it broke unrelated
// tests without a corresponding real-world exploit demonstrated. This check
// therefore proves exact digest reconstruction, not that the native registry
// itself already reports the package as installed; it does not by itself
// prove the package is activated or otherwise in active use.
func validateGroupRecoveryVerification(observation domain.NativeIdentityObservation, managed *domain.ClientBinding) error {
	if managed == nil {
		return fmt.Errorf("recovery verification requires a recorded managed binding")
	}
	if observation.State != domain.NativeIdentityManaged {
		return fmt.Errorf("state is %q, want managed", observation.State)
	}
	expected := managedDigest(*managed)
	if expected == "" || observation.Digest == "" || expected != observation.Digest {
		return fmt.Errorf("restored digest does not match the recorded receipt")
	}
	return nil
}
