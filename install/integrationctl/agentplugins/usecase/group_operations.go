package usecase

import (
	"context"
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

func (service Service) AddGroup(ctx context.Context, input GroupInput) (GroupResult, error) {
	return service.applyGroup(ctx, input, false)
}

func (service Service) SwitchGroup(ctx context.Context, input GroupInput) (GroupResult, error) {
	input.Switch = true
	return service.applyGroup(ctx, input, true)
}

func (service Service) RepairGroup(ctx context.Context, input GroupInput) (GroupResult, error) {
	result, err := service.repairGroup(ctx, input)
	return result, forSavedDependency(err, groupEnvironmentAuthority(input))
}

func (service Service) repairGroup(ctx context.Context, input GroupInput) (GroupResult, error) {
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
			if !sameNativeBackend(domain.ClientID(client.ClientID), target.Client.ClientID, target.Scope) || client.Scope != string(target.Scope) {
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
	// requireAbsent records the filesystem precondition independently of native recovery.
	requireAbsent bool
}

func (session *groupSession) reportGroupProgress(target plannedGroupTarget, phase GroupProgressPhase) error {
	if session.input.Progress == nil {
		return nil
	}
	for _, index := range target.resultIndexes {
		if session.service.profileCheck != nil {
			if err := session.service.checkProfiles(session.ctx); err != nil {
				return err
			}
		}
		result := session.result.Targets[index]
		session.input.Progress(GroupProgressEvent{ClientID: result.Plan.ClientID, Phase: phase, Result: result})
		if session.service.profileCheck != nil {
			if err := session.service.checkProfiles(session.ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func (service Service) applyGroup(ctx context.Context, input GroupInput, replace bool) (GroupResult, error) {
	session := &groupSession{service: service, ctx: ctx, input: input, replace: replace}
	if err := session.prepareGroupInputs(); err != nil {
		return GroupResult{}, err
	}
	service = session.service
	release, err := service.beginEnvironmentMutation(ctx, input.DryRun, input.Confirmed, session.first.EnvironmentAuthority)
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
	if err := session.preflightUnchangedProjections(); err != nil {
		return session.result, err
	}
	if session.input.DryRun || !session.input.Confirmed {
		return session.result, nil
	}
	if err := session.revalidateHosts(); err != nil {
		return session.result, err
	}
	if err := session.stageGroupDeliveries(); err != nil {
		// Staging can prepare a locked runtime after the read-only preflight.
		// No managed package or client was committed, but this is an apply-time
		// preparation failure, not a failed preflight check.
		session.result.Phase = GroupPhasePreparationFailed
		return session.result, err
	}
	defer session.cleanupStaged()
	if err := session.revalidateHosts(); err != nil {
		return session.result, err
	}
	for _, target := range session.planned {
		if err := session.reportGroupProgress(target, GroupProgressConfiguring); err != nil {
			return session.result, err
		}
	}
	// Callers can change any target from a progress callback. Reobserve every
	// identity after the final callback, before building or committing state.
	if err := session.reobserveGroupIdentity(); err != nil {
		return session.result, err
	}
	session.buildDesiredGroupState()
	for _, target := range session.planned {
		if target.input.EnvironmentAuthority != nil {
			_ = transaction.UpsertEnvironmentReceipt(&session.desired, target.input.EnvironmentAuthority,
				session.installationID, target.clientBindingID, firstEnvironmentSourceKey(target.input.EnvironmentAuthority, session.sourceID),
				target.input.Envelope.TreeDigest, target.plan.ProfileAuthority(), "committed")
		}
	}
	if err := session.applyGroupKernel(); err != nil {
		return session.result, err
	}
	return session.activateGroupTargets()
}

func (session *groupSession) prepareGroupInputs() error {
	if err := session.validateGroupInput(); err != nil {
		return err
	}
	targets, err := session.service.prepareHostInputs(session.ctx, session.input.Targets, session.input.DryRun)
	if err != nil {
		return err
	}
	session.input.Targets = targets
	checks, err := session.service.prepareHostInputs(session.ctx, session.input.CompatibilityChecks, session.input.DryRun)
	if err != nil {
		return err
	}
	session.input.CompatibilityChecks = checks
	if err := session.ensureGroupID(); err != nil {
		return err
	}
	if err := session.resolveGroupInstallation(); err != nil {
		return err
	}
	return session.freezeGroupProfiles()
}
