package agentpluginscli

import (
	"context"
	"fmt"
	"time"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func (session *repairManySession) authorizePackages() error {
	for _, key := range session.keys {
		loaded := session.loadedByRevision[key]
		if err := authorizeSecurityAssessment(session.cmd, session.app, session.opts, &loaded); err != nil {
			return err
		}
		session.loadedByRevision[key] = loaded
	}
	return nil
}

func (session *repairManySession) buildInputs() error {
	session.service = lifecycleService(session.app, session.detected)
	inputs := make([]usecase.AddInput, 0, len(session.targets))
	session.result = repairMultiResult{
		Batch: true, Status: "planned", Plugin: session.installation.DeclaredName,
		DryRun: session.opts.dryRun, Targets: make([]repairTargetResult, 0, len(session.targets)),
	}
	for _, target := range session.targets {
		input, err := session.targetInput(target)
		if err != nil {
			return err
		}
		inputs = append(inputs, input)
	}
	session.inputs = inputs
	return nil
}

func (session *repairManySession) targetInput(target domain.ClientID) (usecase.AddInput, error) {
	loaded := session.loadedByRevision[session.targetRevision[target]]
	clientPackage := cloneLoadedPackage(loaded)
	restoreCatalogEvidence(&clientPackage, session.recordedBindings[target])
	if err := prepareLoadedPackageForClient(&clientPackage, target); err != nil {
		return usecase.AddInput{}, fmt.Errorf("preflight target %s: %w; no target was changed", target, err)
	}
	state, err := session.app.StateStore.Load()
	if err != nil {
		return usecase.AddInput{}, err
	}
	authority, err := environmentAuthorityForBinding(state, session.installation.InstallationID, target, "repair")
	if err != nil {
		return usecase.AddInput{}, err
	}
	if authority != nil {
		binding := session.recordedBindings[target]
		if binding.PackageRevision == nil || clientPackage.envelope.TreeDigest != binding.PackageRevision.TreeDigest || clientPackage.envelope.Source.ResolvedRevision != binding.PackageRevision.ResolvedRevision {
			return usecase.AddInput{}, fmt.Errorf("repair must restore the exact recorded dependency pin")
		}
		clientPackage.envelope.Source.SourceBindingHint = "uap-dependency-git"
		if loaded.origin == domain.OriginModeDirectory {
			clientPackage.envelope.Source.SourceBindingHint = "uap-dependency-registry"
		}
	}
	client := session.detected[target]
	return usecase.AddInput{
		Envelope: clientPackage.envelope, Client: client, Scope: session.app.scope(),
		Interactive: false, Hints: clientPackage.hints,
		InstallationID:    session.installation.InstallationID,
		BackendExecutable: backendExecutable(client, session.detected, session.app.scope()),
		OriginMode:        loaded.origin, DirectoryResolution: cloneDirectoryOrigin(loaded.directory),
		DistributionSuspended: loaded.distributionSuspended, ReleaseRevoked: loaded.releaseRevoked,
		EnvironmentAuthority: authority,
	}, nil
}

func (session *repairManySession) planAndApply() error {
	operationID, err := newOperationGroupID()
	if err != nil {
		return err
	}
	session.result.OperationID = operationID
	if err := session.plan(operationID); err != nil {
		return err
	}
	if session.opts.dryRun {
		return renderRepairMultiResult(session.cmd, session.opts, session.result)
	}
	return session.apply()
}

func (session *repairManySession) plan(operationID string) error {
	planned, err := session.service.RepairGroup(session.ctx, usecase.GroupInput{Targets: session.inputs, OperationGroupID: operationID, DryRun: true, OnlinePreview: !session.opts.dryRun, Repair: true})
	retainPreparedClients(session.inputs, planned.PreparedClients)
	for index, targetResult := range planned.Targets {
		output := newRepairResultData(session.installation, targetResult, true)
		output.OperationID = operationID
		session.result.Targets = append(session.result.Targets, repairTargetResult{
			Target: string(session.targets[index]), Status: string(targetResult.Plan.Status),
			NextAction: output.NextAction, Output: output,
		})
	}
	session.result.Succeeded = len(planned.Targets)
	if err != nil {
		session.result.Status, session.result.Failed, session.result.Succeeded = "preflight_failed", len(session.inputs), 0
		_ = renderRepairMultiResult(session.cmd, session.opts, session.result)
		return fmt.Errorf("group repair preflight failed; no target was changed: %w", err)
	}
	return nil
}

func (session *repairManySession) apply() error {
	session.result.Status = "applying"
	session.result.Succeeded = 0
	session.result.Failed = 0
	session.result.Targets = session.result.Targets[:0]
	appliedGroup, groupErr := session.service.RepairGroup(session.ctx, usecase.GroupInput{
		Targets: session.inputs, OperationGroupID: session.result.OperationID, Confirmed: true, Repair: true,
	})
	appliedResults := appliedGroup.Targets
	if len(appliedResults) > len(session.inputs) || len(appliedResults) != len(session.inputs) && groupErr == nil {
		return fmt.Errorf("install engine returned %d repair results for %d targets", len(appliedResults), len(session.inputs))
	}
	for index, applied := range appliedResults {
		output := newRepairResultData(session.installation, applied, false)
		output.OperationID = session.result.OperationID
		session.result.Targets = append(session.result.Targets, repairTargetResult{
			Target: string(session.targets[index]), Status: groupTargetStatus(applied),
			NextAction: output.NextAction, Output: output,
		})
		if applied.GroupPhase == usecase.GroupTargetExternalCompleted {
			session.result.Succeeded++
		}
	}
	if groupErr != nil {
		session.result.Status, session.result.Failed = groupFailureStatus(appliedGroup.Phase), len(session.inputs)-session.result.Succeeded
		_ = renderRepairMultiResult(session.cmd, session.opts, session.result)
		return groupErr
	}
	session.result.Status = string(appliedGroup.Phase)
	return renderRepairMultiResult(session.cmd, session.opts, session.result)
}

func resolveRepairSource(ctx context.Context, timeout time.Duration, resolve func(context.Context) (loadedPackage, error)) (loadedPackage, error) {
	bounded, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return resolve(bounded)
}
