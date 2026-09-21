package usecase

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (session *groupSession) planGroupTargets() error {
	session.physical = map[string]int{}
	session.planned = make([]plannedGroupTarget, 0, len(session.input.Targets))
	for targetIndex, target := range session.input.Targets {
		if err := session.planOneGroupTarget(targetIndex, target); err != nil {
			return err
		}
	}
	return nil
}

func (session *groupSession) planOneGroupTarget(targetIndex int, target AddInput) error {
	if err := session.validateGroupTarget(targetIndex, &target); err != nil {
		return err
	}
	plan, err := session.planAndPreflightGroupTarget(targetIndex, &target)
	if err != nil {
		return err
	}
	collided, err := session.collideGroupTarget(targetIndex, target, plan)
	if err != nil {
		return err
	}
	if collided {
		return nil
	}
	return session.recordGroupTarget(targetIndex, target, plan)
}

func (session *groupSession) validateGroupTarget(targetIndex int, target *AddInput) error {
	if target.OriginMode == "" && target.DirectoryResolution != nil {
		target.OriginMode = domain.OriginModeDirectory
		session.input.Targets[targetIndex] = *target
	}
	if err := validateOperationOrigin(target.OriginMode, target.DirectoryResolution); err != nil {
		return err
	}
	if !session.input.Repair && (target.Envelope.TreeDigest != session.first.Envelope.TreeDigest || target.Envelope.ManifestDigest != session.first.Envelope.ManifestDigest || domain.ComputeSourceBindingID(target.Envelope.Source) != session.sourceID) {
		return fmt.Errorf("all targets in one operation group must use one immutable package")
	}
	if target.Scope != domain.ScopeUser {
		return fmt.Errorf("%s scope is not proven; group mutation supports user scope only", target.Scope)
	}
	if target.ReleaseRevoked && normalizedOriginMode(target.OriginMode) == domain.OriginModeDirectory {
		return fmt.Errorf("revoked release cannot be exposed, updated, or repaired")
	}
	if target.DistributionSuspended && normalizedOriginMode(target.OriginMode) == domain.OriginModeDirectory && !session.input.Repair {
		return fmt.Errorf("suspended distribution blocks group add/update")
	}
	return nil
}

func (session *groupSession) planAndPreflightGroupTarget(targetIndex int, target *AddInput) (domain.DeliveryPlan, error) {
	plan, err := session.service.planInstall(session.ctx, target, domain.ComputePhysicalArtifactID(target.Envelope.Manifest.Name, session.installationID), installationIfExisting(session.state, session.installationIndex, session.existing))
	if err != nil {
		return domain.DeliveryPlan{}, err
	}
	if openAIOAuthApplies(target.Client.ClientID, target.Envelope, target.Hints) {
		plan.Authentication = domain.AuthenticationPending
	}
	session.result.Targets[targetIndex] = AddResult{InstallationID: session.installationID, Plan: plan}
	if target.ReleaseRevoked && normalizedOriginMode(target.OriginMode) == domain.OriginModeDirect {
		plan.Warnings = append(plan.Warnings, "direct_source_digest_matches_known_revoked_directory_release")
		session.result.Targets[targetIndex].Plan = plan
	}
	if plan.Status == domain.PlanUnsupported {
		return domain.DeliveryPlan{}, fmt.Errorf("target %s is unsupported; group preflight caused no mutation", target.Client.ClientID)
	}
	if err := session.service.preflightActivation(*target, plan); err != nil {
		return domain.DeliveryPlan{}, err
	}
	if err := session.service.preflightTargetComponents(session.ctx, *target, &plan, installationIfExisting(session.state, session.installationIndex, session.existing), session.input.Repair, session.replace); err != nil {
		session.result.Targets[targetIndex].Plan = plan
		return domain.DeliveryPlan{}, err
	}
	session.result.Targets[targetIndex].Plan = plan
	return plan, nil
}

func (session *groupSession) collideGroupTarget(targetIndex int, target AddInput, plan domain.DeliveryPlan) (bool, error) {
	key := plan.ActivePath
	if sharesPhysicalBackend(target.Client.ClientID) {
		definition, _ := domain.ClientDefinitionFor(target.Client.ClientID)
		key = "shared-backend:" + definition.BackendFamily + ":" + plan.PhysicalArtifactID
	} else if session.existing {
		for _, binding := range session.state.Installations[session.installationIndex].Clients {
			if binding.PhysicalArtifact == plan.PhysicalArtifactID && sameNativeBackend(domain.ClientID(binding.ClientID), target.Client.ClientID) && binding.Materialization != domain.MaterializationAbsent {
				key = binding.TargetLocator
				break
			}
		}
	}
	priorIndex, ok := session.physical[key]
	if !ok {
		session.collisionKey = key
		return false, nil
	}
	prior := &session.planned[priorIndex]
	if !sameNativeBackend(prior.input.Client.ClientID, target.Client.ClientID) {
		return false, fmt.Errorf("targets collide on physical backend %s", key)
	}
	if prior.noChange {
		session.result.Targets[targetIndex].NoChange = true
		session.result.Targets[targetIndex].Activation = session.result.Targets[prior.resultIndexes[0]].Activation
	}
	prior.resultIndexes = append(prior.resultIndexes, targetIndex)
	session.result.Targets[targetIndex].Plan = prior.plan
	return true, nil
}

func (session *groupSession) recordGroupTarget(targetIndex int, target AddInput, plan domain.DeliveryPlan) error {
	clientID, managed := session.resolveGroupManagedBinding(target, &plan)
	if session.replace {
		describeMCPRemovals(&plan, managed)
	}
	if session.replace && managed == nil {
		return fmt.Errorf("update target %s is not installed", target.Client.ClientID)
	}
	session.result.Targets[targetIndex].Plan = plan
	recovering, err := session.observePlannedGroupTarget(target, plan, managed)
	if err != nil {
		return fmt.Errorf("target %s identity preflight: %w", target.Client.ClientID, err)
	}
	noChange := !requiresComponentRemoval(plan) && managed != nil && !session.input.Repair && !session.input.Switch && groupPackageUnchanged(*managed, target) && containsSurface(managed.AffectedSurfaces, string(target.Client.ClientID))
	if noChange {
		session.result.Targets[targetIndex].NoChange = true
		session.result.Targets[targetIndex].Activation = domain.ActivationOutcome{
			Activation: managed.Activation, Authentication: managed.Authentication, Policy: managed.Policy, Verification: managed.Verification,
		}
	}
	session.physical[session.collisionKey] = len(session.planned)
	session.planned = append(session.planned, plannedGroupTarget{
		input: target, plan: plan, resultIndexes: []int{targetIndex}, clientBindingID: clientID, managed: managed, noChange: noChange, recovering: recovering,
	})
	return nil
}

func (session *groupSession) resolveGroupManagedBinding(target AddInput, plan *domain.DeliveryPlan) (string, *domain.ClientBinding) {
	clientID := domain.ComputeClientBindingID(session.installationID, string(target.Client.ClientID), string(target.Scope), plan.ActivePath)
	if !session.existing {
		return clientID, nil
	}
	if binding, ok := session.state.Installations[session.installationIndex].Clients[clientID]; ok {
		owned := binding
		return clientID, &owned
	}
	if !sharesPhysicalBackend(target.Client.ClientID) {
		return clientID, nil
	}
	for _, binding := range session.state.Installations[session.installationIndex].Clients {
		if binding.PhysicalArtifact == plan.PhysicalArtifactID && sameNativeBackend(domain.ClientID(binding.ClientID), target.Client.ClientID) && binding.Materialization != domain.MaterializationAbsent {
			owned := binding
			plan.ActivePath = binding.TargetLocator
			plan.TargetRoot = filepath.Dir(binding.TargetLocator)
			return binding.ClientBindingID, &owned
		}
	}
	return clientID, nil
}

func (session *groupSession) observePlannedGroupTarget(target AddInput, plan domain.DeliveryPlan, managed *domain.ClientBinding) (bool, error) {
	if session.input.DryRun {
		if err := session.service.observeGroupPreparedIdentity(session.ctx, target.Client, plan, managed, session.input.Repair); err != nil {
			return false, err
		}
		if managed != nil && !session.input.Repair {
			if err := session.service.verifyManagedTarget(session.ctx, target.Client, target.Scope, *managed, "group dry-run"); err != nil {
				return false, err
			}
		}
		return false, nil
	}
	if session.input.Repair && managed != nil && session.service.observeGroupRecoveryEligibility(session.ctx, target.Client, plan, managed) {
		return true, nil
	}
	if err := session.service.observeGroupNativeIdentity(session.ctx, target.Client, plan, managed, session.input.Repair); err != nil {
		return false, err
	}
	return false, nil
}

func (session *groupSession) preflightCompatibleBindings() error {
	if !session.replace || !session.existing || session.input.Repair {
		return nil
	}
	compatibleBindings := map[string]bool{}
	for _, target := range session.planned {
		if target.managed != nil {
			compatibleBindings[target.managed.ClientBindingID] = true
		}
	}
	checks := session.input.CompatibilityChecks
	if session.input.Switch {
		checks = nil
	}
	for _, check := range checks {
		if err := session.preflightOneCompatibleBinding(check, compatibleBindings); err != nil {
			return err
		}
	}
	for _, binding := range session.state.Installations[session.installationIndex].Clients {
		if binding.Materialization == domain.MaterializationAbsent {
			continue
		}
		if !compatibleBindings[binding.ClientBindingID] {
			return fmt.Errorf("update group must preflight every installed physical binding; %s is missing", binding.ClientID)
		}
	}
	return nil
}

func (session *groupSession) preflightOneCompatibleBinding(check AddInput, compatibleBindings map[string]bool) error {
	if check.Envelope.TreeDigest != session.first.Envelope.TreeDigest || check.Envelope.ManifestDigest != session.first.Envelope.ManifestDigest {
		return fmt.Errorf("compatibility preflight must use the update candidate bytes")
	}
	plan, err := session.service.planInstall(session.ctx, &check, domain.ComputePhysicalArtifactID(check.Envelope.Manifest.Name, session.installationID), installationIfExisting(session.state, session.installationIndex, session.existing))
	if err != nil {
		return err
	}
	if plan.Status == domain.PlanUnsupported {
		return fmt.Errorf("update candidate is incompatible with installed binding %s", check.Client.ClientID)
	}
	if err := session.service.preflightActivation(check, plan); err != nil {
		return err
	}
	if err := session.service.preflightTargetComponents(session.ctx, check, &plan, &session.state.Installations[session.installationIndex], false, true); err != nil {
		return err
	}
	for _, binding := range session.state.Installations[session.installationIndex].Clients {
		if sameNativeBackend(domain.ClientID(binding.ClientID), check.Client.ClientID) && binding.Scope == string(check.Scope) {
			compatibleBindings[binding.ClientBindingID] = true
		}
	}
	return nil
}
