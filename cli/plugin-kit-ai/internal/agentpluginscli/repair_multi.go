package agentpluginscli

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

const repairSourceResolutionTimeout = 2 * time.Minute

type repairTargetResult struct {
	Target     string           `json:"target"`
	Status     string           `json:"status"`
	NextAction string           `json:"next_action,omitempty"`
	Output     repairResultData `json:"output"`
}

type repairMultiResult struct {
	OperationID string               `json:"operation_id,omitempty"`
	Batch       bool                 `json:"batch"`
	Status      string               `json:"status"`
	Succeeded   int                  `json:"succeeded"`
	Failed      int                  `json:"failed"`
	Plugin      string               `json:"plugin"`
	DryRun      bool                 `json:"dry_run"`
	Targets     []repairTargetResult `json:"targets"`
}

type repairRevisionRequest struct {
	sequence uint64
	revision string
	source   string
	targets  []domain.ClientID
}

type repairManySession struct {
	ctx              context.Context
	cmd              *cobra.Command
	app              App
	opts             *options
	installation     domain.Installation
	targets          []domain.ClientID
	requests         map[string]*repairRevisionRequest
	targetRevision   map[domain.ClientID]string
	recordedBindings map[domain.ClientID]domain.ClientBinding
	keys             []string
	loadedByRevision map[string]loadedPackage
	detected         map[domain.ClientID]domain.DetectedClient
	inputs           []usecase.AddInput
	service          usecase.Service
	result           repairMultiResult
}

func runRepairMany(ctx context.Context, cmd *cobra.Command, app App, opts *options, selector string, targets []domain.ClientID) error {
	session := &repairManySession{ctx: ctx, cmd: cmd, app: app, opts: opts, targets: targets}
	if err := session.loadInstallation(selector); err != nil {
		return err
	}
	if err := session.collectRevisionRequests(); err != nil {
		return err
	}
	if err := session.preflight(); err != nil {
		return err
	}
	defer session.cleanupLoaded()
	if err := session.resolvePackages(); err != nil {
		return err
	}
	if err := session.authorizePackages(); err != nil {
		return err
	}
	if err := session.buildInputs(); err != nil {
		return err
	}
	return session.planAndApply()
}

func (session *repairManySession) loadInstallation(selector string) error {
	state, err := session.app.StateStore.Load()
	if err != nil {
		return err
	}
	installation, err := selectInstallation(state, selector)
	if err != nil {
		return err
	}
	if installation.NeedsRebind || installation.Package.LoaderKind != domain.LoaderKindAgentPlugins {
		return fmt.Errorf("repair requires a bound Agent Plugins installation")
	}
	session.installation = installation
	return nil
}

func (session *repairManySession) collectRevisionRequests() error {
	session.requests = map[string]*repairRevisionRequest{}
	session.targetRevision = make(map[domain.ClientID]string, len(session.targets))
	session.recordedBindings = make(map[domain.ClientID]domain.ClientBinding, len(session.targets))
	for _, target := range session.targets {
		if err := session.recordTargetRevision(target); err != nil {
			return err
		}
	}
	return nil
}

func (session *repairManySession) recordTargetRevision(target domain.ClientID) error {
	binding, err := selectedRepairBinding(session.installation, target, domain.ScopeUser)
	if err != nil {
		return fmt.Errorf("preflight target %s: %w; no target was changed", target, err)
	}
	if binding.PackageRevision == nil {
		return fmt.Errorf("repair target %s has no recorded package revision", target)
	}
	session.recordedBindings[target] = binding
	key, request, err := session.revisionKey(binding)
	if err != nil {
		return err
	}
	session.targetRevision[target] = key
	if existing := session.requests[key]; existing != nil {
		existing.targets = expandAffectedSurfaceTargets(append(existing.targets, bindingSurfaceTargets(binding)...))
		return nil
	}
	request.targets = expandAffectedSurfaceTargets(bindingSurfaceTargets(binding))
	session.requests[key] = &request
	return nil
}

func (session *repairManySession) revisionKey(binding domain.ClientBinding) (string, repairRevisionRequest, error) {
	request := repairRevisionRequest{}
	var key string
	if session.installation.OriginMode == domain.OriginModeDirectory {
		request.sequence = binding.PackageRevision.ReleaseSequence
		request.revision = binding.PackageRevision.ResolvedRevision
		key = "directory:" + binding.PackageRevision.DistributionID + ":" + strconv.FormatUint(request.sequence, 10)
	} else {
		source, err := repairSource(session.installation, binding)
		if err != nil {
			return "", request, err
		}
		request.source = source
		key = "direct:" + source
	}
	key += ":" + binding.PackageRevision.ResolvedRevision + ":" + binding.PackageRevision.TreeDigest + ":" + binding.PackageRevision.ManifestDigest
	if binding.PackageRevision.CatalogEvidence != nil {
		key += ":" + binding.PackageRevision.CatalogEvidence.Digest
	}
	return key, request, nil
}

func (session *repairManySession) preflight() error {
	_, detected, err := preflightSelectedTargets(
		session.ctx, session.app, session.targets, nil,
		!session.opts.dryRun && session.installation.OriginMode == domain.OriginModeDirectory,
		lifecycleInstallIntents(session.installation, session.opts.scope, nil),
	)
	if err != nil {
		return err
	}
	session.detected = detected
	return nil
}

func (session *repairManySession) resolvePackages() error {
	writeProgress(session.app, session.opts.format, "Resolving and validating each unique exact installed package revision once...")
	keys := make([]string, 0, len(session.requests))
	for key := range session.requests {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	session.keys = keys
	session.loadedByRevision = make(map[string]loadedPackage, len(keys))
	for _, key := range keys {
		loaded, err := session.resolveRevision(key)
		if err != nil {
			return err
		}
		session.loadedByRevision[key] = loaded
	}
	return nil
}

func (session *repairManySession) resolveRevision(key string) (loadedPackage, error) {
	request := session.requests[key]
	return resolveRepairSource(session.ctx, repairSourceResolutionTimeout, func(resolutionCtx context.Context) (loadedPackage, error) {
		if session.installation.OriginMode == domain.OriginModeDirectory {
			return session.app.loadInstalledPackage(resolutionCtx, session.installation, request.targets, domain.DirectoryRepair, request.sequence, request.revision, session.detected)
		}
		return session.app.loadPackageFor(resolutionCtx, request.source, packageResolutionRequest{
			Targets: request.targets, Operation: domain.DirectoryRepair, Clients: session.detected,
		})
	})
}

func (session *repairManySession) cleanupLoaded() {
	for index := len(session.keys) - 1; index >= 0; index-- {
		loaded, ok := session.loadedByRevision[session.keys[index]]
		if !ok || loaded.cleanup == nil {
			continue
		}
		_ = loaded.cleanup()
	}
}

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
	client := session.detected[target]
	return usecase.AddInput{
		Envelope: clientPackage.envelope, Client: client, Scope: domain.ScopeUser,
		Interactive: false, Hints: clientPackage.hints,
		InstallationID:    session.installation.InstallationID,
		BackendExecutable: backendExecutable(client, session.detected),
		OriginMode:        loaded.origin, DirectoryResolution: cloneDirectoryOrigin(loaded.directory),
		DistributionSuspended: loaded.distributionSuspended, ReleaseRevoked: loaded.releaseRevoked,
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
	planned, err := session.service.RepairGroup(session.ctx, usecase.GroupInput{Targets: session.inputs, OperationGroupID: operationID, DryRun: true, Repair: true})
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

func renderRepairMultiResult(cmd *cobra.Command, opts *options, result repairMultiResult) error {
	if opts.format == "json" {
		overall := "success"
		if result.Failed > 0 {
			overall = "failure"
		}
		return writeJSONResult(cmd.OutOrStdout(), "repair", overall, result)
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Repair %s: %s\n", result.Plugin, result.Status); err != nil {
		return err
	}
	if err := renderRepairTargets(cmd, result); err != nil {
		return err
	}
	if result.Status == "completed" {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Managed package repaired from the exact installed revision and its package digest verified; external client activation and authentication were not reverified."); err != nil {
			return err
		}
	}
	return nil
}

func renderRepairTargets(cmd *cobra.Command, result repairMultiResult) error {
	for _, target := range result.Targets {
		if err := renderOpenCodeRuntimeNotice(cmd.OutOrStdout(), target.Output.Result); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", target.Target, target.Status); err != nil {
			return err
		}
		if target.NextAction != "" {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "    Next: %s\n", localTargetLifecycleAction(target.Output.Result, target.NextAction)); err != nil {
				return err
			}
		}
	}
	return nil
}
