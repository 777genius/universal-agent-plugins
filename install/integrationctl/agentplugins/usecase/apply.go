package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// applySession carries the locals apply() used to thread through one add or
// update. Extract-method keeps call order; the struct is the phase context,
// not a new policy layer.
type applySession struct {
	service               Service
	ctx                   context.Context
	input                 AddInput
	replace               bool
	result                AddResult
	state                 domain.StateFileV2
	sourceBindingID       string
	installationIndex     int
	existing              bool
	stickyMatch           bool
	installationID        string
	plan                  domain.DeliveryPlan
	clientBindingID       string
	isMaterialized        bool
	registrationMigration bool
	managedBinding        *domain.ClientBinding
}

func (session *applySession) validateApplyInput() error {
	if err := session.input.InstallIntent.Validate(session.input.Client.ClientID); err != nil {
		return err
	}
	if session.input.OriginMode == "" && session.input.DirectoryResolution != nil {
		session.input.OriginMode = domain.OriginModeDirectory
	}
	if err := validateOperationOrigin(session.input.OriginMode, session.input.DirectoryResolution); err != nil {
		return err
	}
	if session.service.StateStore == nil || session.service.Paths == nil || session.service.Planner == nil || session.service.Stager == nil || session.service.Activator == nil {
		return fmt.Errorf("agentplugins service dependencies are incomplete")
	}
	if session.input.Envelope.LoaderKind != domain.LoaderKindAgentPlugins {
		return fmt.Errorf("agentplugins add accepts only standard Agent Plugins packages")
	}
	return nil
}

func (session *applySession) resolveInstallation() error {
	state, err := session.service.StateStore.Load()
	if err != nil {
		return err
	}
	session.state = state
	session.sourceBindingID = domain.ComputeSourceBindingID(session.input.Envelope.Source)
	session.installationIndex, session.existing, session.stickyMatch, err = findStickyInstallation(state, session.input, session.sourceBindingID)
	if err != nil {
		return err
	}
	session.installationID = strings.TrimSpace(session.input.InstallationID)
	if session.existing {
		installation := state.Installations[session.installationIndex]
		if installation.NeedsRebind {
			return fmt.Errorf("installation %s requires explicit rebind", installation.InstallationID)
		}
		if session.input.Envelope.Manifest.Name != installation.DeclaredName {
			return fmt.Errorf("refuse package identity change from %q to %q; remove all targets and use explicit rebind", installation.DeclaredName, session.input.Envelope.Manifest.Name)
		}
		if installation.Package.LoaderKind != session.input.Envelope.LoaderKind || installation.Package.FormatID != session.input.Envelope.FormatID {
			return fmt.Errorf("source is bound to a different package format; use migrate-format")
		}
		session.installationID = installation.InstallationID
		if session.stickyMatch && installation.Source.SourceBindingID != session.sourceBindingID {
			sameDistribution := installation.OriginMode == domain.OriginModeDirectory && installation.Directory != nil && session.input.DirectoryResolution != nil && installation.Directory.DistributionID == session.input.DirectoryResolution.DistributionID
			if !sameDistribution {
				return fmt.Errorf("installation %s is sticky to %s; use switch to change source", installation.InstallationID, installation.Source.CanonicalSource)
			}
		}
	}
	if session.installationID == "" {
		session.installationID, err = domain.NewInstallationID()
		if err != nil {
			return err
		}
	}
	if session.replace && !session.existing {
		return fmt.Errorf("update source is not bound to an existing installation; use add or rebind")
	}
	return nil
}

func (session *applySession) validateApplyTransitions() error {
	if err := session.validateExistingPackage(); err != nil {
		return err
	}
	return session.validateReleasePolicy()
}

func (session *applySession) validateExistingPackage() error {
	if !session.existing {
		return nil
	}
	// Identity/release transitions must fail before the more general "use
	// update" diagnostic. A same-release digest conflict is a supply-chain
	// failure regardless of whether the caller happened to be adding a target.
	installation := session.state.Installations[session.installationIndex]
	if err := validatePackageTransition(installation, session.input.Envelope); err != nil {
		return err
	}
	if err := validateDirectoryTransition(installation, session.input); err != nil {
		return err
	}
	if session.replace {
		if installation.OriginMode == domain.OriginModeDirect && immutableDirectGit(installation.Source) {
			return fmt.Errorf("direct full-SHA installations have no update channel; use switch --to with a new full SHA")
		}
		return nil
	}
	if installation.OriginMode == domain.OriginModeDirectory && installation.Directory != nil && session.input.DirectoryResolution != nil && session.input.DirectoryResolution.DesiredReleaseSequence != installation.Directory.DesiredReleaseSequence {
		return fmt.Errorf("adding a target must use recorded release sequence %d; run update separately", installation.Directory.DesiredReleaseSequence)
	}
	if installation.Source.TreeDigest != session.input.Envelope.TreeDigest {
		return fmt.Errorf("adding a target must use the recorded desired package bytes; run update separately")
	}
	return nil
}

func (session *applySession) validateReleasePolicy() error {
	if session.input.ReleaseRevoked && normalizedOriginMode(session.input.OriginMode) == domain.OriginModeDirectory {
		return fmt.Errorf("release is revoked; new exposure, update, and repair are blocked while removal remains available")
	}
	if session.input.DistributionSuspended && normalizedOriginMode(session.input.OriginMode) == domain.OriginModeDirectory && (!session.existing || session.replace) {
		return fmt.Errorf("distribution is suspended; new installs, new targets, and updates are blocked")
	}
	return nil
}

func (session *applySession) planAndPreflight() error {
	physicalID := domain.ComputePhysicalArtifactID(session.input.Envelope.Manifest.Name, session.installationID)
	plan, err := session.service.planInstall(session.ctx, &session.input, physicalID, installationIfExisting(session.state, session.installationIndex, session.existing))
	if err != nil {
		return err
	}
	if openAIOAuthApplies(session.input.Client.ClientID, session.input.Envelope, session.input.Hints) {
		plan.Authentication = domain.AuthenticationPending
	}
	session.plan = plan
	session.result = AddResult{InstallationID: session.installationID, Plan: plan}
	if session.input.ReleaseRevoked && normalizedOriginMode(session.input.OriginMode) == domain.OriginModeDirect {
		plan.Warnings = append(plan.Warnings, "direct_source_digest_matches_known_revoked_directory_release")
		session.plan = plan
		session.result.Plan = plan
	}
	if plan.Status == domain.PlanUnsupported {
		action := strings.Join(plan.UserActions, "; ")
		if action != "" {
			return fmt.Errorf("delivery plan for %s is unsupported. Next: %s", plan.ClientID, action)
		}
		return fmt.Errorf("delivery plan for %s is unsupported", plan.ClientID)
	}
	if err := session.service.preflightActivation(session.input, plan); err != nil {
		return err
	}
	if err := session.service.preflightTargetComponents(session.ctx, session.input, &plan, installationIfExisting(session.state, session.installationIndex, session.existing), false, session.replace); err != nil {
		session.plan = plan
		session.result.Plan = plan
		return err
	}
	session.plan = plan
	session.result.Plan = plan
	return rejectNativeNameCollision(session.state, session.installationID, session.input.Envelope.Manifest.Name, session.input.Client.ClientID)
}

func (session *applySession) resolveBinding() error {
	session.clientBindingID = domain.ComputeClientBindingID(session.installationID, string(session.input.Client.ClientID), string(session.input.Scope), session.plan.ActivePath)
	session.adoptSharedBinding()
	session.isMaterialized = session.existing && materializedClient(session.state.Installations[session.installationIndex], session.clientBindingID)
	session.registrationMigration = session.existing && session.input.Envelope.LocalChatGPTMapping != nil && session.state.Installations[session.installationIndex].LocalChatGPTMapping != nil && session.state.Installations[session.installationIndex].LocalChatGPTMapping.IsLegacyContext7Registration() && *session.state.Installations[session.installationIndex].LocalChatGPTMapping != *session.input.Envelope.LocalChatGPTMapping
	if session.isMaterialized {
		binding := session.state.Installations[session.installationIndex].Clients[session.clientBindingID]
		session.managedBinding = &binding
	}
	if session.replace {
		describeMCPRemovals(&session.plan, session.managedBinding)
		session.result.Plan = session.plan
	}
	if err := session.rejectBlockedTarget(); err != nil {
		return err
	}
	if err := session.service.checkMCPNamespace(session.ctx, session.input.Client, &session.plan, session.managedBinding); err != nil {
		return err
	}
	session.result.Plan = session.plan
	return nil
}

func (session *applySession) adoptSharedBinding() {
	if !session.existing || !sharesPhysicalBackend(session.input.Client.ClientID) {
		return
	}
	for key, binding := range session.state.Installations[session.installationIndex].Clients {
		if binding.Scope != string(session.input.Scope) || binding.Materialization == domain.MaterializationAbsent || binding.PhysicalArtifact != session.plan.PhysicalArtifactID || !sameNativeBackend(domain.ClientID(binding.ClientID), session.input.Client.ClientID) {
			continue
		}
		session.clientBindingID = key
		session.plan.ActivePath = binding.TargetLocator
		session.plan.TargetRoot = filepath.Dir(binding.TargetLocator)
		session.result.Plan = session.plan
		return
	}
}

func (session *applySession) rejectBlockedTarget() error {
	if session.input.DistributionSuspended && normalizedOriginMode(session.input.OriginMode) == domain.OriginModeDirectory && !session.isMaterialized {
		return fmt.Errorf("distribution is suspended; adding a target is blocked")
	}
	if !session.isMaterialized && (session.input.ActivationComplete || session.input.AuthComplete) {
		return fmt.Errorf("lifecycle completion flags require an already materialized package; run add first, then rerun add with the completion flags")
	}
	return nil
}

func (session *applySession) dryRunPath() (AddResult, error) {
	if err := session.service.observePreparedIdentity(session.ctx, session.input.Client, session.plan, session.managedBinding); err != nil {
		return session.result, err
	}
	if session.replace && !session.isMaterialized {
		return session.result, fmt.Errorf("plugin is not materialized for %s; use add", session.input.Client.ClientID)
	}
	if session.isMaterialized {
		current := session.state.Installations[session.installationIndex].Clients[session.clientBindingID]
		session.result.Activation = lifecycleOutcome(current)
		if err := session.service.verifyManagedTarget(session.ctx, session.input.Client, session.input.Scope, current, "dry-run"); err != nil {
			return session.result, err
		}
		if !session.replace && !packageRevisionMatches(current.PackageRevision, session.input.Envelope) {
			return session.result, fmt.Errorf("plugin is already materialized for %s at a different revision; use update", session.input.Client.ClientID)
		}
		session.result.NoChange = !session.replace && !session.registrationMigration && lifecycleConverged(current)
	}
	return session.result, nil
}

func (session *applySession) noChangeOrResume() (bool, AddResult, error) {
	if session.managedBinding == nil {
		if err := session.service.observeNativeIdentity(session.ctx, session.input.Client, session.plan, nil); err != nil {
			return true, session.result, err
		}
	}
	if session.isMaterialized && !session.replace && !session.registrationMigration {
		current := session.state.Installations[session.installationIndex].Clients[session.clientBindingID]
		done, result, err := session.finishExistingLifecycle(current, "no-change check")
		return done, result, err
	}
	if session.replace && !session.isMaterialized {
		return true, session.result, fmt.Errorf("plugin is not materialized for %s; use add", session.input.Client.ClientID)
	}
	if session.replace {
		previousClient := session.state.Installations[session.installationIndex].Clients[session.clientBindingID]
		session.result.Activation = lifecycleOutcome(previousClient)
		if err := session.service.verifyManagedTarget(session.ctx, session.input.Client, session.input.Scope, previousClient, "update"); err != nil {
			return true, session.result, err
		}
		if err := session.service.observeNativeIdentity(session.ctx, session.input.Client, session.plan, session.managedBinding); err != nil {
			return true, session.result, err
		}
		if !requiresComponentRemoval(session.plan) && packageRevisionMatches(previousClient.PackageRevision, session.input.Envelope) && previousClient.PackageRevision.ResolvedRevision == session.input.Envelope.Source.ResolvedRevision {
			return session.finishExistingLifecycle(previousClient, "")
		}
	}
	return false, AddResult{}, nil
}

func (session *applySession) finishExistingLifecycle(current domain.ClientBinding, verifyLabel string) (bool, AddResult, error) {
	session.result.Activation = lifecycleOutcome(current)
	if verifyLabel == "no-change check" && !packageRevisionMatches(current.PackageRevision, session.input.Envelope) {
		return true, session.result, fmt.Errorf("plugin is already materialized for %s at a different revision; use update", session.input.Client.ClientID)
	}
	if session.input.Confirmed {
		installation := session.state.Installations[session.installationIndex]
		if err := session.service.prepareExistingRuntime(session.ctx, session.input.Envelope, session.plan, installation, current); err != nil {
			return true, session.result, fmt.Errorf("prepare installed MCP runtime: %w", err)
		}
	}
	if !lifecycleConverged(current) {
		result, err := session.service.resume(session.ctx, session.input, session.result, session.installationID, session.clientBindingID, current)
		return true, result, err
	}
	if verifyLabel != "" {
		if err := session.service.verifyManagedTarget(session.ctx, session.input.Client, session.input.Scope, current, verifyLabel); err != nil {
			return true, session.result, err
		}
	}
	return session.persistReadOnlyObservation(current)
}

func (session *applySession) persistReadOnlyObservation(current domain.ClientBinding) (bool, AddResult, error) {
	verified, verifyErr := session.service.verifyClientReadOnly(session.ctx, session.input, session.result, current)
	if verifyErr != nil {
		if verified.Activation != "" && !session.input.DryRun && (session.input.Confirmed || session.input.PersistAuthoritativeObservations && verified.AuthoritativeObservation) {
			session.result.Activation = verified
			changed, updateErr := session.service.persistAuthoritativeObservation(session.ctx, session.input, session.installationID, session.clientBindingID, current, verified)
			session.result.Mutated = changed
			if updateErr != nil {
				return true, session.result, fmt.Errorf("client verification failed: %w; persist negative verification evidence: %w", verifyErr, updateErr)
			}
		}
		return true, session.result, verifyErr
	}
	if verified.Activation != "" && !sameLifecycleOutcome(verified, lifecycleOutcome(current)) {
		result, err := session.service.persistObservedLifecycle(session.input, session.result, session.installationID, session.clientBindingID, verified)
		return true, result, err
	}
	session.result.NoChange = true
	return true, session.result, nil
}
