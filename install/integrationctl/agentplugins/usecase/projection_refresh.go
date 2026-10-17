package usecase

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// ErrProjectionRefreshRequired means an intact package with installer-rendered
// components would be staged differently today than the recorded projection.
// Repair, a repeated add and a resumed activation keep the installed tree;
// only update applies the current rules. It is domain.ErrProjectionRefreshRequired,
// which the stager also returns.
var ErrProjectionRefreshRequired = domain.ErrProjectionRefreshRequired

// ErrProjectionPinChangeRequired is what a saved dependency gets instead of
// ErrProjectionRefreshRequired: an update cannot refresh it, so the new rules
// wait for its next pin change.
var ErrProjectionPinChangeRequired = domain.ErrProjectionPinChangeRequired

// forSavedDependency keeps a saved dependency from being told to run an update
// that cannot help it.
func forSavedDependency(err error, authority *domain.EnvironmentAuthority) error {
	if authority == nil || !errors.Is(err, ErrProjectionRefreshRequired) {
		return err
	}
	return ErrProjectionPinChangeRequired
}

// groupEnvironmentAuthority is the saved-dependency authority of a group, if
// any of its targets carries one.
func groupEnvironmentAuthority(input GroupInput) *domain.EnvironmentAuthority {
	for _, target := range input.Targets {
		if target.EnvironmentAuthority != nil {
			return target.EnvironmentAuthority
		}
	}
	return nil
}

// Update applies a new package revision. For an unchanged revision whose
// delivered tree also depends on installer rendering, it first refreshes the
// projection, so newer rules reach the installed tree; the ordinary update
// then runs as before, including runtime preparation and resume. Its preview
// is never reported as unchanged.
func (service Service) Update(ctx context.Context, input AddInput) (AddResult, error) {
	result, err := service.update(ctx, input)
	return result, forSavedDependency(err, input.EnvironmentAuthority)
}

func (service Service) update(ctx context.Context, input AddInput) (AddResult, error) {
	installationID, eligible, err := service.projectionRefreshTarget(input)
	if err != nil {
		return AddResult{}, err
	}
	if !eligible {
		return service.apply(ctx, input, true)
	}
	refreshed, err := service.refreshBeforeUpdate(ctx, input, installationID, refreshOperationID(input.OperationID, "-refresh"))
	if err != nil || refreshed.RequiresConfirmation {
		return refreshed, err
	}
	applied, err := service.apply(ctx, input, true)
	if input.DryRun {
		markRefreshPending(&applied)
		return applied, err
	}
	applied.Mutated = applied.Mutated || refreshed.Mutated
	applied.NoChange = applied.NoChange && !refreshed.Mutated
	return applied, err
}

// markRefreshPending keeps the preview of an eligible target from reporting
// the package as current. Only staging shows whether the current rules change
// the tree, and a preview never stages, so callers that skip unchanged
// previews must still run this update.
func markRefreshPending(result *AddResult) {
	result.NoChange = false
	result.RequiresConfirmation = true
}

// UpdateGroup refreshes every eligible target in turn before the group
// update. A preview marks those targets as needing confirmation and never as
// unchanged, and a refresh failure stops before the group update starts.
func (service Service) UpdateGroup(ctx context.Context, input GroupInput) (GroupResult, error) {
	result, err := service.updateGroup(ctx, input)
	return result, forSavedDependency(err, groupEnvironmentAuthority(input))
}

func (service Service) updateGroup(ctx context.Context, input GroupInput) (GroupResult, error) {
	eligible := map[int]string{}
	for index, target := range input.Targets {
		installationID, ok, err := service.projectionRefreshTarget(target)
		if err != nil {
			return GroupResult{}, err
		}
		if ok {
			eligible[index] = installationID
		}
	}
	if len(eligible) == 0 {
		return service.applyGroup(ctx, input, true)
	}
	if input.DryRun || !input.Confirmed {
		result, err := service.applyGroup(ctx, input, true)
		for index := range eligible {
			if index < len(result.Targets) {
				markRefreshPending(&result.Targets[index])
			}
		}
		return result, err
	}
	refreshed, err := service.refreshGroupTargets(ctx, input, eligible)
	if err != nil {
		return refreshed, err
	}
	result, err := service.applyGroup(ctx, input, true)
	return mergeRefreshedGroup(result, refreshed, err), err
}

// mergeRefreshedGroup folds the refreshes into the group update's result. A
// refresh that changed a target stays reported when the update then fails:
// unless the update names a later stage itself, the group ends as a partial
// failure rather than as a failure that changed nothing.
func mergeRefreshedGroup(result, refreshed GroupResult, err error) GroupResult {
	if len(result.Targets) == 0 {
		result.InstallationID = refreshed.InstallationID
		result.OperationGroupID = refreshed.OperationGroupID
		result.Targets = refreshed.Targets
	}
	for index, target := range refreshed.Targets {
		if target.Mutated && index < len(result.Targets) {
			result.Targets[index].Mutated = true
			result.Targets[index].NoChange = false
		}
	}
	result.Mutated = result.Mutated || refreshed.Mutated
	if err != nil && refreshed.Mutated {
		switch result.Phase {
		case "", GroupPhasePlanned, GroupPhasePreparationFailed, GroupPhaseManagedUnchanged, GroupPhaseManagedRolledBack:
			result.Phase = GroupPhaseExternalPartialFailure
		}
	}
	return result
}

// refreshGroupTargets refreshes the eligible targets in order and stops at the
// first failure, marking every target of the partial result.
func (service Service) refreshGroupTargets(ctx context.Context, input GroupInput, eligible map[int]string) (GroupResult, error) {
	indexes := make([]int, 0, len(eligible))
	for index := range eligible {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	partial := GroupResult{OperationGroupID: input.OperationGroupID, Targets: make([]AddResult, len(input.Targets))}
	attempted := map[int]bool{}
	for _, index := range indexes {
		target := input.Targets[index]
		target.Confirmed, target.DryRun = true, false
		attempted[index] = true
		result, err := service.refreshBeforeUpdate(ctx, target, eligible[index], refreshOperationID(input.OperationGroupID, "-refresh-"+string(target.Client.ClientID)))
		partial.InstallationID = eligible[index]
		partial.Targets[index] = result
		partial.Mutated = partial.Mutated || result.Mutated
		if err != nil {
			partial.Phase = GroupPhasePreparationFailed
			if partial.Mutated {
				partial.Phase = GroupPhaseExternalPartialFailure
			}
			markStoppedRefresh(&partial, input, index, attempted, err)
			return partial, err
		}
	}
	return partial, nil
}

// markStoppedRefresh gives every target of a stopped group refresh its client,
// scope and phase, which hosts need to report it: completed for an earlier
// refresh, failed for the one that stopped, not attempted for the rest.
func markStoppedRefresh(partial *GroupResult, input GroupInput, failed int, attempted map[int]bool, err error) {
	for index := range partial.Targets {
		target := &partial.Targets[index]
		if target.Plan.ClientID == "" {
			target.Plan.ClientID, target.Plan.Scope = input.Targets[index].Client.ClientID, input.Targets[index].Scope
		}
		switch {
		case index == failed:
			target.GroupPhase = GroupTargetExternalFailed
			target.Failure = &GroupTargetFailure{Stage: "refresh", Message: err.Error()}
		case attempted[index]:
			target.GroupPhase = GroupTargetExternalCompleted
		default:
			target.GroupPhase = GroupTargetExternalNotAttempted
			target.Failure = &GroupTargetFailure{Stage: "refresh", Message: "processing stopped because an earlier projection refresh failed"}
		}
	}
}

// projectionRefreshTarget reports the installation to refresh before this
// update: one without environment authority (RefreshProjection authorizes
// only "repair", which a saved-dependency sync never grants), of a package
// with installer-rendered components, whose binding for this client and
// scope is materialized at exactly this revision. It only reads state;
// RefreshProjection checks the exact binding again under its own lock.
func (service Service) projectionRefreshTarget(input AddInput) (string, bool, error) {
	if input.EnvironmentAuthority != nil || !rendersCommands(input.Envelope, input.Client.ClientID) || service.StateStore == nil {
		return "", false, nil
	}
	state, err := service.StateStore.Load()
	if err != nil {
		return "", false, err
	}
	index, existing := findSourceInstallation(state, domain.ComputeSourceBindingID(input.Envelope.Source))
	if !existing {
		return "", false, nil
	}
	installation := state.Installations[index]
	for _, binding := range installation.Clients {
		if binding.ClientID == string(input.Client.ClientID) && binding.Scope == string(input.Scope) &&
			binding.Materialization == domain.MaterializationMaterialized && packageRevisionMatches(binding.PackageRevision, input.Envelope) &&
			binding.PackageRevision.ResolvedRevision == input.Envelope.Source.ResolvedRevision {
			return installation.InstallationID, true, nil
		}
	}
	return "", false, nil
}

// rendersCommands reports whether command rules shape this client's tree:
// the package has command entries and the client can receive commands. A
// client whose registry entry withholds them is left as for a package
// without commands.
func rendersCommands(envelope domain.PackageEnvelope, client domain.ClientID) bool {
	definition, _ := domain.ClientDefinitionFor(client)
	return envelope.HasInstallerRenderedComponents() && domain.ReceivesCommands(definition)
}

func (service Service) refreshBeforeUpdate(ctx context.Context, input AddInput, installationID, operationID string) (AddResult, error) {
	input.InstallationID = installationID
	input.OperationID = operationID
	input.refreshChangedProjectionOnly = true
	return service.RefreshProjection(ctx, input)
}

// refreshOperationID derives the refresh's own operation id, which has to
// stay a valid leaf id however long the host's id is. An empty base leaves it
// empty, so the refresh generates one.
func refreshOperationID(base, suffix string) string {
	if strings.TrimSpace(base) == "" {
		return ""
	}
	return domain.DeriveLeafID(strings.TrimSpace(base), suffix)
}
