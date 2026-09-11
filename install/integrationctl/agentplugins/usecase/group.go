package usecase

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
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
	for _, target := range append(append([]AddInput(nil), input.Targets...), input.CompatibilityChecks...) {
		if err := target.InstallIntent.Validate(target.Client.ClientID); err != nil {
			return GroupResult{}, err
		}
	}
	if len(input.Targets) == 0 {
		return GroupResult{}, fmt.Errorf("at least one target is required")
	}
	if service.StateStore == nil || service.Planner == nil || service.Stager == nil || service.Activator == nil {
		return GroupResult{}, fmt.Errorf("agentplugins group dependencies are incomplete")
	}
	groupID := strings.TrimSpace(input.OperationGroupID)
	if groupID == "" {
		var err error
		groupID, err = newOperationID()
		if err != nil {
			return GroupResult{}, err
		}
	}
	release, err := service.beginMutation(ctx, input.DryRun, input.Confirmed)
	if err != nil {
		return GroupResult{}, err
	}
	if release != nil {
		defer func() { _ = release() }()
	}
	state, err := service.StateStore.Load()
	if err != nil {
		return GroupResult{}, err
	}
	first := input.Targets[0]
	if first.OriginMode == "" && first.DirectoryResolution != nil {
		first.OriginMode = domain.OriginModeDirectory
		input.Targets[0] = first
	}
	if err := validateOperationOrigin(first.OriginMode, first.DirectoryResolution); err != nil {
		return GroupResult{}, err
	}
	if first.Envelope.LoaderKind != domain.LoaderKindAgentPlugins {
		return GroupResult{}, fmt.Errorf("group accepts only standard Agent Plugins packages")
	}
	sourceID := domain.ComputeSourceBindingID(first.Envelope.Source)
	installationIndex, existing, sticky, err := findStickyInstallation(state, first, sourceID)
	if err != nil {
		return GroupResult{}, err
	}
	installationID := strings.TrimSpace(first.InstallationID)
	var lifecycleBaseline *domain.Installation
	if existing {
		installation := state.Installations[installationIndex]
		baseline := installation
		lifecycleBaseline = &baseline
		installationID = installation.InstallationID
		if sticky && installation.Source.SourceBindingID != sourceID && !input.Switch {
			sameDistribution := installation.OriginMode == domain.OriginModeDirectory && installation.Directory != nil && first.DirectoryResolution != nil && installation.Directory.DistributionID == first.DirectoryResolution.DistributionID
			if !sameDistribution {
				return GroupResult{}, fmt.Errorf("installation is source-sticky; use switch")
			}
		}
		if installation.NeedsRebind {
			return GroupResult{}, fmt.Errorf("installation requires explicit rebind")
		}
		if input.Switch {
			if installation.DeclaredName != first.Envelope.Manifest.Name {
				return GroupResult{}, fmt.Errorf("switch must preserve manifest identity")
			}
			if installation.OriginMode == domain.OriginModeDirectory && normalizedOriginMode(first.OriginMode) == domain.OriginModeDirectory && installation.Directory != nil && first.DirectoryResolution != nil && installation.Directory.ProductID != first.DirectoryResolution.ProductID {
				return GroupResult{}, fmt.Errorf("switch must remain within one Directory product")
			}
			for otherIndex, other := range state.Installations {
				if otherIndex != installationIndex && other.Source.SourceBindingID == sourceID {
					return GroupResult{}, fmt.Errorf("switch source is already bound to installation %s", other.InstallationID)
				}
			}
		} else if !input.Repair {
			if err := validateDirectoryTransition(installation, first); err != nil {
				return GroupResult{}, err
			}
		}
		if replace && !input.Switch && !input.Repair && installation.OriginMode == domain.OriginModeDirect && immutableDirectGit(installation.Source) {
			return GroupResult{}, fmt.Errorf("direct full-SHA installations require explicit switch")
		}
		if !replace && installation.OriginMode == domain.OriginModeDirectory && installation.Directory != nil && first.DirectoryResolution != nil && first.DirectoryResolution.DesiredReleaseSequence != installation.Directory.DesiredReleaseSequence {
			return GroupResult{}, fmt.Errorf("adding targets must retain recorded release sequence %d; update separately", installation.Directory.DesiredReleaseSequence)
		}
		if !replace && installation.Source.TreeDigest != first.Envelope.TreeDigest {
			return GroupResult{}, fmt.Errorf("adding targets must use recorded desired package bytes; update separately")
		}
	} else if replace {
		return GroupResult{}, fmt.Errorf("update requires an existing installation")
	}
	if installationID == "" {
		installationID, err = domain.NewInstallationID()
		if err != nil {
			return GroupResult{}, err
		}
	}
	result := GroupResult{InstallationID: installationID, OperationGroupID: groupID, Targets: make([]AddResult, len(input.Targets)), Phase: GroupPhasePlanned}
	if input.Switch && lifecycleBaseline != nil {
		result.PluginData = switchPluginDataDecision(*lifecycleBaseline)
	}
	physical := map[string]int{}
	planned := make([]plannedGroupTarget, 0, len(input.Targets))
	for targetIndex, target := range input.Targets {
		if target.OriginMode == "" && target.DirectoryResolution != nil {
			target.OriginMode = domain.OriginModeDirectory
			input.Targets[targetIndex] = target
		}
		if err := validateOperationOrigin(target.OriginMode, target.DirectoryResolution); err != nil {
			return result, err
		}
		if !input.Repair && (target.Envelope.TreeDigest != first.Envelope.TreeDigest || target.Envelope.ManifestDigest != first.Envelope.ManifestDigest || domain.ComputeSourceBindingID(target.Envelope.Source) != sourceID) {
			return result, fmt.Errorf("all targets in one operation group must use one immutable package")
		}
		if target.Scope != domain.ScopeUser {
			return result, fmt.Errorf("%s scope is not proven; group mutation supports user scope only", target.Scope)
		}
		if target.ReleaseRevoked && normalizedOriginMode(target.OriginMode) == domain.OriginModeDirectory {
			return result, fmt.Errorf("revoked release cannot be exposed, updated, or repaired")
		}
		if target.DistributionSuspended && normalizedOriginMode(target.OriginMode) == domain.OriginModeDirectory && !input.Repair {
			return result, fmt.Errorf("suspended distribution blocks group add/update")
		}
		plan, err := service.planInstall(ctx, &target, domain.ComputePhysicalArtifactID(target.Envelope.Manifest.Name, installationID), installationIfExisting(state, installationIndex, existing))
		if err != nil {
			return result, err
		}
		if openAIOAuthApplies(target.Client.ClientID, target.Envelope, target.Hints) {
			plan.Authentication = domain.AuthenticationPending
		}
		result.Targets[targetIndex] = AddResult{InstallationID: installationID, Plan: plan}
		if target.ReleaseRevoked && normalizedOriginMode(target.OriginMode) == domain.OriginModeDirect {
			plan.Warnings = append(plan.Warnings, "direct_source_digest_matches_known_revoked_directory_release")
			result.Targets[targetIndex].Plan = plan
		}
		if plan.Status == domain.PlanUnsupported {
			return result, fmt.Errorf("target %s is unsupported; group preflight caused no mutation", target.Client.ClientID)
		}
		if err := service.preflightActivation(target, plan); err != nil {
			return result, err
		}
		if err := service.preflightTargetComponents(ctx, target, &plan, installationIfExisting(state, installationIndex, existing), input.Repair, replace); err != nil {
			result.Targets[targetIndex].Plan = plan
			return result, err
		}
		result.Targets[targetIndex].Plan = plan
		key := plan.ActivePath
		if sameNativeBackend(target.Client.ClientID, domain.ClientCopilot) {
			key = "shared-copilot-vscode:" + plan.PhysicalArtifactID
		} else if existing {
			for _, binding := range state.Installations[installationIndex].Clients {
				if binding.PhysicalArtifact == plan.PhysicalArtifactID && sameNativeBackend(domain.ClientID(binding.ClientID), target.Client.ClientID) && binding.Materialization != domain.MaterializationAbsent {
					key = binding.TargetLocator
					break
				}
			}
		}
		if priorIndex, ok := physical[key]; ok {
			prior := &planned[priorIndex]
			if !sameNativeBackend(prior.input.Client.ClientID, target.Client.ClientID) {
				return result, fmt.Errorf("targets collide on physical backend %s", key)
			}
			if prior.noChange {
				result.Targets[targetIndex].NoChange = true
				result.Targets[targetIndex].Activation = result.Targets[prior.resultIndexes[0]].Activation
			}
			prior.resultIndexes = append(prior.resultIndexes, targetIndex)
			result.Targets[targetIndex].Plan = prior.plan
			continue
		}
		clientID := domain.ComputeClientBindingID(installationID, string(target.Client.ClientID), string(target.Scope), plan.ActivePath)
		var managed *domain.ClientBinding
		if existing {
			if binding, ok := state.Installations[installationIndex].Clients[clientID]; ok {
				copy := binding
				managed = &copy
			} else if sameNativeBackend(target.Client.ClientID, domain.ClientCopilot) {
				for _, binding := range state.Installations[installationIndex].Clients {
					if binding.PhysicalArtifact == plan.PhysicalArtifactID && sameNativeBackend(domain.ClientID(binding.ClientID), target.Client.ClientID) && binding.Materialization != domain.MaterializationAbsent {
						copy := binding
						managed = &copy
						clientID = binding.ClientBindingID
						plan.ActivePath = binding.TargetLocator
						plan.TargetRoot = filepath.Dir(binding.TargetLocator)
						break
					}
				}
			}
		}
		if replace {
			describeMCPRemovals(&plan, managed)
		}
		if replace && managed == nil {
			return result, fmt.Errorf("update target %s is not installed", target.Client.ClientID)
		}
		result.Targets[targetIndex].Plan = plan
		recovering := false
		if input.DryRun {
			if err := service.observeGroupPreparedIdentity(ctx, target.Client, plan, managed, input.Repair); err != nil {
				return result, err
			}
			if managed != nil && !input.Repair {
				if err := service.verifyManagedTarget(ctx, target.Client, target.Scope, *managed, "group dry-run"); err != nil {
					return result, err
				}
			}
		} else if input.Repair && managed != nil && service.observeGroupRecoveryEligibility(ctx, target.Client, plan, managed) {
			recovering = true
		} else if err := service.observeGroupNativeIdentity(ctx, target.Client, plan, managed, input.Repair); err != nil {
			return result, err
		}
		noChange := !requiresComponentRemoval(plan) && managed != nil && !input.Repair && !input.Switch && groupPackageUnchanged(*managed, target) && containsSurface(managed.AffectedSurfaces, string(target.Client.ClientID))
		if noChange {
			result.Targets[targetIndex].NoChange = true
			result.Targets[targetIndex].Activation = domain.ActivationOutcome{Activation: managed.Activation, Authentication: managed.Authentication, Policy: managed.Policy, Verification: managed.Verification}
		}
		physical[key] = len(planned)
		planned = append(planned, plannedGroupTarget{input: target, plan: plan, resultIndexes: []int{targetIndex}, clientBindingID: clientID, managed: managed, noChange: noChange, recovering: recovering})
	}
	if replace && existing && !input.Repair {
		compatibleBindings := map[string]bool{}
		for _, target := range planned {
			if target.managed != nil {
				compatibleBindings[target.managed.ClientBindingID] = true
			}
		}
		checks := input.CompatibilityChecks
		if input.Switch {
			checks = nil
		}
		for _, check := range checks {
			if check.Envelope.TreeDigest != first.Envelope.TreeDigest || check.Envelope.ManifestDigest != first.Envelope.ManifestDigest {
				return result, fmt.Errorf("compatibility preflight must use the update candidate bytes")
			}
			plan, err := service.planInstall(ctx, &check, domain.ComputePhysicalArtifactID(check.Envelope.Manifest.Name, installationID), installationIfExisting(state, installationIndex, existing))
			if err != nil {
				return result, err
			}
			if plan.Status == domain.PlanUnsupported {
				return result, fmt.Errorf("update candidate is incompatible with installed binding %s", check.Client.ClientID)
			}
			if err := service.preflightActivation(check, plan); err != nil {
				return result, err
			}
			if err := service.preflightTargetComponents(ctx, check, &plan, &state.Installations[installationIndex], false, true); err != nil {
				return result, err
			}
			for _, binding := range state.Installations[installationIndex].Clients {
				if sameNativeBackend(domain.ClientID(binding.ClientID), check.Client.ClientID) && binding.Scope == string(check.Scope) {
					compatibleBindings[binding.ClientBindingID] = true
				}
			}
		}
		for _, binding := range state.Installations[installationIndex].Clients {
			if binding.Materialization == domain.MaterializationAbsent {
				continue
			}
			if !compatibleBindings[binding.ClientBindingID] {
				return result, fmt.Errorf("update group must preflight every installed physical binding; %s is missing", binding.ClientID)
			}
		}
	}
	if input.DryRun || !input.Confirmed {
		return result, nil
	}
	cleanup := func() {
		for _, target := range planned {
			if target.delivery.StagingPath != "" {
				_ = service.Stager.Discard(context.Background(), target.delivery)
			}
			if target.dataCreated {
				_ = service.PluginData.PurgeData(context.Background(), target.dataReceipt)
			}
		}
	}
	for targetIndex := range planned {
		target := &planned[targetIndex]
		if target.noChange {
			continue
		}
		operationID := fmt.Sprintf("%s-%03d", groupID, targetIndex+1)
		if packageNeedsPluginData(target.input.Envelope, target.plan) {
			if service.PluginData == nil {
				cleanup()
				return result, fmt.Errorf("PLUGIN_DATA manager is required for stdio MCP packages")
			}
			receipt, created, err := service.PluginData.EnsureData(ctx, installationID, target.plan.PhysicalArtifactID, string(target.input.Scope))
			if err != nil {
				cleanup()
				return result, err
			}
			target.dataReceipt, target.dataCreated = receipt, created
		}
		delivery, err := service.stagePackage(ctx, target.input.Envelope, target.plan, operationID, target.input.Hints, target.dataReceipt.Locator)
		if err != nil {
			cleanup()
			return result, err
		}
		delivery, err = bindStagedDeliveryToPhysicalOwner(delivery, target.plan, target.managed)
		if err != nil {
			_ = service.Stager.Discard(context.Background(), delivery)
			cleanup()
			return result, err
		}
		target.delivery = delivery
	}
	defer cleanup()
	for _, target := range planned {
		if target.recovering {
			// The recovering target's directory is still absent at this point; its
			// native registry is expected to keep failing until the group's
			// directories are actually restored. Re-confirm, from the filesystem
			// alone, that nothing has since occupied the target: a genuine native
			// verification happens once, after restoration, in PostApplyVerify.
			if !service.observeGroupRecoveryEligibility(ctx, target.input.Client, target.plan, target.managed) {
				return result, fmt.Errorf("native identity changed before group commit: recorded absent target %s is no longer eligible for recovery", target.input.Client.ClientID)
			}
			// The staged reconstruction must reproduce the exact prior receipt
			// digest, not merely a new, self-consistent build: repair only ever
			// authorizes restoring what was already recorded as owned.
			if expected := managedDigest(*target.managed); expected == "" || target.delivery.ArtifactDigest != expected {
				return result, fmt.Errorf("staged reconstruction for %s does not match the recorded package digest", target.input.Client.ClientID)
			}
			continue
		}
		if err := service.observeGroupNativeIdentity(ctx, target.input.Client, target.plan, target.managed, input.Repair); err != nil {
			return result, fmt.Errorf("native identity changed before group commit: %w", err)
		}
	}
	desired := state
	for _, target := range planned {
		if target.noChange {
			continue
		}
		desired, installationIndex = upsertPreparedInstallation(desired, installationIndex, existing, target.input, target.plan, installationID, target.clientBindingID, sourceID, service.now())
		existing = true
		installation := desired.Installations[installationIndex]
		client := installation.Clients[target.clientBindingID]
		if target.managed != nil {
			// Addressing a shared backend through another logical surface must not
			// rewrite the persisted physical binding identity (and thereby make its
			// deterministic binding ID inconsistent).
			client.ClientID = target.managed.ClientID
			client.AffectedSurfaces = append(client.AffectedSurfaces, target.managed.AffectedSurfaces...)
			client.AffectedSurfaces = append(client.AffectedSurfaces, target.managed.ClientID)
		}
		if sameNativeBackend(target.input.Client.ClientID, domain.ClientCopilot) {
			client.AffectedSurfaces = append(client.AffectedSurfaces, string(domain.ClientCopilot), string(domain.ClientVSCode))
		}
		for _, resultIndex := range target.resultIndexes {
			client.AffectedSurfaces = append(client.AffectedSurfaces, string(input.Targets[resultIndex].Client.ClientID))
		}
		client.AffectedSurfaces = uniqueSortedSurfaces(client.AffectedSurfaces)
		if input.Repair && target.managed != nil {
			// A repair commit becomes authoritative before directory/receipt
			// finalization. Preserve prior authentication evidence in that commit
			// decision so a late failure cannot reset completed authentication.
			client.Authentication = preservedGroupAuthentication(target.plan.Authentication, target.managed.Authentication)
		}
		if target.dataReceipt.DataReceiptID != "" {
			if installation.DataReceipts == nil {
				installation.DataReceipts = map[string]domain.DataReceipt{}
			}
			installation.DataReceipts[target.dataReceipt.DataReceiptID] = target.dataReceipt
			client.DataReceiptID = target.dataReceipt.DataReceiptID
		} else if input.Switch && target.managed != nil {
			// A destination that does not consume PLUGIN_DATA must not sever the
			// retained receipt from the active binding. A later reverse switch
			// can therefore recover the same owned directory.
			client.DataReceiptID = target.managed.DataReceiptID
		}
		installation.DataRetained = false
		installation.OperationGroupID = groupID
		installation.Clients[target.clientBindingID] = client
		desired.Installations[installationIndex] = installation
	}
	if input.Repair && lifecycleBaseline != nil {
		// Repair restores each binding's own recorded projection. It must not let
		// iteration order move the installation-wide desired release/source back
		// to whichever heterogeneous binding happened to be repaired last.
		installation := desired.Installations[installationIndex]
		installation.DeclaredName = lifecycleBaseline.DeclaredName
		installation.Source = lifecycleBaseline.Source
		installation.Package = lifecycleBaseline.Package
		installation.OriginMode = lifecycleBaseline.OriginMode
		installation.Directory = cloneDirectoryOrigin(lifecycleBaseline.Directory)
		desired.Installations[installationIndex] = installation
	}
	if input.Switch {
		installation := desired.Installations[installationIndex]
		installation.Source = domain.SourceBinding{SourceBindingID: sourceID, RequestedSource: first.Envelope.Source.RequestedSource,
			CanonicalSource: first.Envelope.Source.CanonicalSource, Repository: first.Envelope.Source.Repository, PackageSubpath: first.Envelope.Source.PackageSubpath,
			ResolvedRevision: first.Envelope.Source.ResolvedRevision, TreeDigest: first.Envelope.TreeDigest}
		installation.OriginMode = normalizedOriginMode(first.OriginMode)
		installation.Directory = cloneDirectoryOrigin(first.DirectoryResolution)
		desired.Installations[installationIndex] = installation
	} else if input.Repair && lifecycleBaseline != nil {
		installation := desired.Installations[installationIndex]
		installation.Source = lifecycleBaseline.Source
		installation.Package = lifecycleBaseline.Package
		installation.OriginMode = lifecycleBaseline.OriginMode
		installation.Directory = cloneDirectoryOrigin(lifecycleBaseline.Directory)
		desired.Installations[installationIndex] = installation
	}
	mutations := make([]transaction.DirectoryMutation, 0, len(planned))
	for targetIndex, target := range planned {
		if target.noChange {
			continue
		}
		client := desired.Installations[installationIndex].Clients[target.clientBindingID]
		before := ""
		if target.managed != nil && !target.recovering {
			// A recovering target's active path is positively absent right now; its
			// recorded receipt digest describes what was there before it disappeared,
			// not the current (absent) state this mutation actually observed.
			before = managedDigest(*target.managed)
		}
		operationID := fmt.Sprintf("%s-%03d", groupID, targetIndex+1)
		delivery := target.delivery
		authentication := target.plan.Authentication
		if input.Repair && target.managed != nil {
			authentication = preservedGroupAuthentication(authentication, target.managed.Authentication)
		}
		mutations = append(mutations, transaction.DirectoryMutation{OperationID: operationID, InstallationID: installationID, ClientBindingID: target.clientBindingID,
			Sequence: nextSequence(client), OwnedBase: delivery.OwnedBase, ActivePath: delivery.ActivePath, StagingPath: delivery.StagingPath,
			BeforeDigest: before, AfterDigest: delivery.ArtifactDigest, NativeObjects: delivery.NativeObjects, Activation: target.plan.Activation,
			Authentication: authentication, Policy: domain.PolicyAllowed, Verification: target.plan.Verification, RequireAbsent: target.recovering,
			Verify: func(verifyContext context.Context, activePath string) error {
				return service.Stager.Verify(verifyContext, activePath, delivery.ArtifactDigest)
			}})
	}
	kernel := service.Kernel
	kernel.StateStore = service.StateStore
	postApplyVerify := service.groupRecoveryPostApplyVerify(planned)
	var receipts []domain.MutationReceipt
	if len(mutations) > 0 {
		receipts, err = kernel.ApplyDirectoryGroup(ctx, transaction.DirectoryGroup{OperationGroupID: groupID, Mutations: mutations, DesiredState: desired, PostApplyVerify: postApplyVerify})
		if err != nil {
			result.Receipts = receipts
			assignGroupReceipts(result.Targets, planned, receipts)
			failure := transaction.FailurePhase(err)
			switch failure {
			case transaction.GroupFailureRolledBack:
				result.Phase = GroupPhaseManagedRolledBack
				for index := range result.Targets {
					result.Targets[index].GroupPhase = GroupTargetManagedRolledBack
				}
			case transaction.GroupFailureCommitted:
				result.Phase = GroupPhaseManagedCommitted
			case transaction.GroupFailureUnknown:
				result.Phase = GroupPhaseManagedCommitUnknown
				for index := range result.Targets {
					result.Targets[index].GroupPhase = GroupTargetManagedUnknown
				}
			default:
				result.Phase = GroupPhaseManagedUnchanged
			}
			if failure == transaction.GroupFailureCommitted {
				receipted := make(map[string]bool, len(receipts))
				for _, receipt := range receipts {
					receipted[receipt.ClientBindingID] = true
				}
				for _, target := range planned {
					if !receipted[target.clientBindingID] {
						continue
					}
					for _, resultIndex := range target.resultIndexes {
						result.Targets[resultIndex].GroupPhase = GroupTargetManagedCommitted
					}
				}
			}
			if failure == transaction.GroupFailureCommitted || failure == transaction.GroupFailureUnknown {
				for index := range planned {
					planned[index].dataCreated = false
				}
			}
			return result, err
		}
		result.Receipts, result.Mutated = receipts, true
		assignGroupReceipts(result.Targets, planned, receipts)
		result.Phase = GroupPhaseManagedCommitted
		for index := range result.Targets {
			result.Targets[index].GroupPhase = GroupTargetManagedCommitted
		}
	}
	for index := range planned {
		planned[index].dataCreated = false
	}
	externalCompleted := 0
	externalFailed := 0
	logicalTotal := len(result.Targets)
	var firstActivationErr error
	markRemainingNotAttempted := func(fromIndex int, stage, message string) {
		for index := fromIndex; index < len(planned); index++ {
			target := planned[index]
			for _, resultIndex := range target.resultIndexes {
				result.Targets[resultIndex].GroupPhase = GroupTargetExternalNotAttempted
				result.Targets[resultIndex].Failure = &GroupTargetFailure{Stage: stage, Message: message}
			}
		}
	}
	classifyActivationFailure := func() {
		if externalCompleted > 0 {
			result.Phase = GroupPhaseExternalPartialFailure
			return
		}
		result.Phase = GroupPhaseManagedActivationFailed
	}
	for plannedIndex, target := range planned {
		if err := ctx.Err(); err != nil {
			classifyActivationFailure()
			markRemainingNotAttempted(plannedIndex, "canceled", "processing stopped because the operation was canceled before remaining clients could be activated safely")
			return result, fmt.Errorf("%d of %d client activations failed: %w", externalFailed+countNotAttempted(result.Targets), logicalTotal, err)
		}
		delivery := target.delivery
		if target.noChange && target.managed != nil {
			delivery = domain.StagedDelivery{
				ClientID: target.input.Client.ClientID, OwnedBase: target.plan.TargetRoot,
				ActivePath: target.managed.TargetLocator, ArtifactDigest: managedDigest(*target.managed),
				NativeObjects: append([]domain.NativeObjectOwnership(nil), target.managed.NativeObjects...),
			}
		}
		outcome, activationErr := service.Activator.Activate(ctx, domain.ActivationRequest{Client: target.input.Client, Plan: target.plan, Delivery: delivery,
			DeclaredName: target.input.Envelope.Manifest.Name, Replacing: replace, Interactive: target.input.Interactive, BackendExecutable: target.input.BackendExecutable,
			PreviousNativeObjects: func() []domain.NativeObjectOwnership {
				if target.managed == nil {
					return nil
				}
				return append([]domain.NativeObjectOwnership(nil), target.managed.NativeObjects...)
			}(),
			VerifyOnly: target.noChange, ActivationComplete: target.input.ActivationComplete})
		if input.Repair && target.managed != nil {
			outcome = preserveManagedAuthentication(outcome, target.managed.Authentication)
		}
		if activationErr == nil && outcome.Activation == "" {
			if err := ctx.Err(); err != nil {
				activationErr = err
			} else {
				activationErr = fmt.Errorf("activator returned an empty activation outcome")
			}
		}
		if activationErr == nil && (outcome.Activation == domain.ActivationFailed || outcome.Verification == domain.VerificationFailed || outcome.Authentication == domain.AuthenticationFailed) {
			activationErr = fmt.Errorf("activator reported a failed activation outcome without an error")
		}
		if activationErr != nil && outcome.Activation == "" {
			outcome = domain.ActivationOutcome{
				Activation: domain.ActivationFailed, Authentication: target.plan.Authentication,
				Policy: domain.PolicyAllowed, Verification: domain.VerificationFailed,
			}
		}
		if target.noChange && target.managed != nil {
			if activationErr == nil && !clientVerifierAvailable(target.input, target.plan) && target.managed.Activation == domain.ActivationActive && target.managed.Verification == domain.VerificationInstalled {
				outcome.Activation = target.managed.Activation
				outcome.Verification = target.managed.Verification
			}
			if (target.managed.Authentication == domain.AuthenticationPending || target.managed.Authentication == domain.AuthenticationNotChecked) && target.input.AuthComplete {
				outcome.Authentication = domain.AuthenticationComplete
				outcome.AuthenticationAttested = true
			} else if target.managed.Authentication != "" {
				outcome.Authentication = target.managed.Authentication
			}
		}
		previousNativeObjects := []domain.NativeObjectOwnership(nil)
		if target.managed != nil {
			previousNativeObjects = target.managed.NativeObjects
		}
		lifecycleChanged, persistErr := service.updateActivationResult(installationID, target.clientBindingID, outcome, activationErr, previousNativeObjects)
		if lifecycleChanged {
			result.Mutated = true
		}
		for _, resultIndex := range target.resultIndexes {
			result.Targets[resultIndex].Activation = outcome
			result.Targets[resultIndex].Mutated = !target.noChange || lifecycleChanged
			if lifecycleChanged {
				result.Targets[resultIndex].NoChange = false
			}
			switch {
			case persistErr != nil:
				result.Targets[resultIndex].GroupPhase = GroupTargetExternalFailed
				result.Targets[resultIndex].Failure = &GroupTargetFailure{Stage: "persist", Message: persistErr.Error()}
			case activationErr != nil:
				result.Targets[resultIndex].GroupPhase = GroupTargetExternalFailed
				result.Targets[resultIndex].Failure = groupTargetFailureFromActivation(activationErr, outcome)
			default:
				result.Targets[resultIndex].GroupPhase = GroupTargetExternalCompleted
			}
		}
		if persistErr != nil {
			externalFailed += len(target.resultIndexes)
			classifyActivationFailure()
			markRemainingNotAttempted(plannedIndex+1, "persist", "processing stopped because installation state could not be saved safely")
			return result, fmt.Errorf("%d of %d client activations failed: %w", externalFailed+countNotAttempted(result.Targets), logicalTotal, persistErr)
		}
		if activationErr != nil {
			externalFailed += len(target.resultIndexes)
			if firstActivationErr == nil {
				firstActivationErr = activationErr
			}
			if errors.Is(activationErr, context.Canceled) || errors.Is(activationErr, context.DeadlineExceeded) || ctx.Err() != nil {
				classifyActivationFailure()
				markRemainingNotAttempted(plannedIndex+1, "canceled", "processing stopped because the operation was canceled before remaining clients could be activated safely")
				cause := activationErr
				if ctxErr := ctx.Err(); ctxErr != nil && !errors.Is(activationErr, context.Canceled) && !errors.Is(activationErr, context.DeadlineExceeded) {
					cause = ctxErr
				}
				return result, fmt.Errorf("%d of %d client activations failed: %w", externalFailed+countNotAttempted(result.Targets), logicalTotal, cause)
			}
			continue
		}
		externalCompleted += len(target.resultIndexes)
	}
	if externalFailed > 0 {
		classifyActivationFailure()
		if firstActivationErr != nil {
			return result, fmt.Errorf("%d of %d client activations failed: %w", externalFailed, logicalTotal, firstActivationErr)
		}
		return result, fmt.Errorf("%d of %d client activations failed", externalFailed, logicalTotal)
	}
	result.Phase = GroupPhaseCompleted
	return result, nil
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
// This is deliberately restricted to Codex: it is the only client with
// evidence (run05) that its native registry command fails outright while the
// target it would report on is absent. Other clients' registry commands have
// not been shown to share that failure mode, so they keep going through the
// ordinary, immediate CLI-inclusive check.
func (service Service) observeGroupRecoveryEligibility(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) bool {
	if client.ClientID != domain.ClientCodex || managed == nil || managedDigest(*managed) == "" || service.NativeObserver == nil {
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
