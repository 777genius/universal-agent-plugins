package usecase

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type repairSession struct {
	service        Service
	ctx            context.Context
	input          AddInput
	result         AddResult
	state          domain.StateFileV2
	index          int
	installation   domain.Installation
	plan           domain.DeliveryPlan
	clientKey      string
	client         domain.ClientBinding
	target         domain.DeliveryTarget
	expectedDigest string
	verifyErr      error
}

func (session *repairSession) validateRepairInput() error {
	if err := session.input.InstallIntent.Validate(session.input.Client.ClientID); err != nil {
		return err
	}
	if session.input.OriginMode == "" && session.input.DirectoryResolution != nil {
		session.input.OriginMode = domain.OriginModeDirectory
	}
	if err := validateOperationOrigin(session.input.OriginMode, session.input.DirectoryResolution); err != nil {
		return err
	}
	if session.service.StateStore == nil || session.service.Paths == nil || session.service.Planner == nil || session.service.Targets == nil || session.service.Stager == nil {
		return fmt.Errorf("agentplugins repair dependencies are incomplete")
	}
	if session.input.ReleaseRevoked && normalizedOriginMode(session.input.OriginMode) == domain.OriginModeDirectory {
		return fmt.Errorf("revoked release cannot be repaired or rematerialized; remove or update to a safe release")
	}
	return nil
}

func (session *repairSession) loadRepairTarget() error {
	state, err := session.service.StateStore.Load()
	if err != nil {
		return err
	}
	session.state = state
	index, existing := findSourceInstallation(state, domain.ComputeSourceBindingID(session.input.Envelope.Source))
	if !existing {
		return fmt.Errorf("resolved source is not bound to an installation")
	}
	installation := state.Installations[index]
	if installation.InstallationID != session.input.InstallationID || installation.DeclaredName != session.input.Envelope.Manifest.Name {
		return fmt.Errorf("resolved repair source identity does not match the selected installation")
	}
	physicalID := domain.ComputePhysicalArtifactID(installation.DeclaredName, installation.InstallationID)
	plan, err := session.service.planInstall(session.ctx, &session.input, physicalID, &installation)
	if err != nil {
		return err
	}
	session.index = index
	session.installation = installation
	session.plan = plan
	session.result = AddResult{InstallationID: installation.InstallationID, Plan: plan}
	if err := session.service.preflightActivation(session.input, plan); err != nil {
		return err
	}
	if err := session.service.preflightTargetComponents(session.ctx, session.input, &plan, &installation, true, false); err != nil {
		session.plan = plan
		session.result.Plan = plan
		return err
	}
	session.plan = plan
	session.result.Plan = plan
	return session.bindRepairClient(physicalID)
}

func (session *repairSession) bindRepairClient(physicalID string) error {
	session.clientKey = domain.ComputeClientBindingID(session.installation.InstallationID, string(session.input.Client.ClientID), string(session.input.Scope), session.plan.ActivePath)
	client, ok := session.installation.Clients[session.clientKey]
	if !ok && sharesPhysicalBackend(session.input.Client.ClientID) {
		for key, binding := range session.installation.Clients {
			if binding.Scope != string(session.input.Scope) || binding.Materialization == domain.MaterializationAbsent ||
				binding.PhysicalArtifact != session.plan.PhysicalArtifactID || !sameNativeBackend(domain.ClientID(binding.ClientID), session.input.Client.ClientID) {
				continue
			}
			session.clientKey, client, ok = key, binding, true
			session.plan.ActivePath = binding.TargetLocator
			session.plan.TargetRoot = filepath.Dir(binding.TargetLocator)
			session.result.Plan = session.plan
			break
		}
	}
	if !ok || (client.Materialization != domain.MaterializationMaterialized && client.Materialization != domain.MaterializationDegraded) {
		return fmt.Errorf("plugin is not materialized for %s", session.input.Client.ClientID)
	}
	if client.ClientBindingID != session.clientKey || !sameNativeBackend(domain.ClientID(client.ClientID), session.input.Client.ClientID) ||
		client.Scope != string(session.input.Scope) || client.PhysicalArtifact != physicalID {
		return fmt.Errorf("managed repair target identity does not match the selected binding")
	}
	session.client = client
	session.result.Activation = lifecycleOutcome(client)
	return session.validateRepairRevision()
}

func (session *repairSession) validateRepairRevision() error {
	if !repairPackageRevisionMatches(session.client.PackageRevision, session.input.Envelope, session.installation.OriginMode == domain.OriginModeDirectory) {
		return fmt.Errorf("resolved repair package differs from the installed revision; use update")
	}
	if session.installation.OriginMode == domain.OriginModeDirectory {
		if session.installation.Directory == nil || session.input.DirectoryResolution == nil || session.client.PackageRevision == nil ||
			session.client.PackageRevision.DistributionID != session.installation.Directory.DistributionID || session.client.PackageRevision.ReleaseSequence < 1 ||
			session.input.DirectoryResolution.DistributionID != session.client.PackageRevision.DistributionID || session.input.DirectoryResolution.DesiredReleaseSequence != session.client.PackageRevision.ReleaseSequence {
			return fmt.Errorf("repair requires the exact recorded Directory release sequence")
		}
	}
	if session.client.PackageRevision == nil || strings.TrimSpace(session.client.PackageRevision.ResolvedRevision) != strings.TrimSpace(session.input.Envelope.Source.ResolvedRevision) {
		return fmt.Errorf("resolved repair package does not match the exact installed revision")
	}
	return nil
}

func (session *repairSession) verifyRepairPreconditions() error {
	targetClient := session.input.Client
	targetClient.ClientID = domain.ClientID(session.client.ClientID)
	target, err := session.service.Targets.ResolveTarget(session.ctx, targetClient, session.input.Scope, session.client.PhysicalArtifact)
	if err != nil {
		return fmt.Errorf("resolve managed repair target: %w", err)
	}
	if err := session.service.Paths.RequireExactPath(target.ActivePath, session.client.TargetLocator); err != nil {
		return fmt.Errorf("refuse repair of untrusted persisted target: %w", err)
	}
	session.target = target
	session.expectedDigest = managedDigest(session.client)
	if session.expectedDigest == "" {
		return fmt.Errorf("managed package digest is missing; refusing repair")
	}
	session.verifyErr = session.service.Stager.Verify(session.ctx, target.ActivePath, session.expectedDigest)
	return nil
}

func (session *repairSession) persistRepair(client domain.ClientBinding) error {
	client.UpdatedAt = session.service.now().Format("2006-01-02T15:04:05.999999999Z07:00")
	session.installation.Clients[session.clientKey] = client
	session.installation.UpdatedAt = client.UpdatedAt
	session.state.Installations[session.index] = session.installation
	return session.service.StateStore.Save(session.state)
}

func repairMismatchKind(err error) (*ports.VerificationError, bool) {
	var verification *ports.VerificationError
	if !errors.As(err, &verification) {
		return nil, false
	}
	return verification, verification.Kind == ports.VerificationAbsent || verification.Kind == ports.VerificationDigestMismatch
}
