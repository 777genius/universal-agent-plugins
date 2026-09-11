package agentpluginscli

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	"github.com/spf13/cobra"
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

func runRepairMany(ctx context.Context, cmd *cobra.Command, app App, opts *options, selector string, targets []domain.ClientID) error {
	state, err := app.StateStore.Load()
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
	type revisionRequest struct {
		sequence uint64
		revision string
		source   string
		targets  []domain.ClientID
	}
	requests := map[string]*revisionRequest{}
	targetRevision := make(map[domain.ClientID]string, len(targets))
	recordedBindings := make(map[domain.ClientID]domain.ClientBinding, len(targets))
	for _, target := range targets {
		binding, err := selectedRepairBinding(installation, target, domain.ScopeUser)
		if err != nil {
			return fmt.Errorf("preflight target %s: %w; no target was changed", target, err)
		}
		if binding.PackageRevision == nil {
			return fmt.Errorf("repair target %s has no recorded package revision", target)
		}
		recordedBindings[target] = binding
		key := ""
		request := revisionRequest{}
		if installation.OriginMode == domain.OriginModeDirectory {
			request.sequence = binding.PackageRevision.ReleaseSequence
			request.revision = binding.PackageRevision.ResolvedRevision
			key = "directory:" + binding.PackageRevision.DistributionID + ":" + strconv.FormatUint(request.sequence, 10)
		} else {
			source, err := repairSource(installation, binding)
			if err != nil {
				return err
			}
			request.source = source
			key = "direct:" + source
		}
		key += ":" + binding.PackageRevision.ResolvedRevision + ":" + binding.PackageRevision.TreeDigest + ":" + binding.PackageRevision.ManifestDigest
		if binding.PackageRevision.CatalogEvidence != nil {
			key += ":" + binding.PackageRevision.CatalogEvidence.Digest
		}
		targetRevision[target] = key
		if existing := requests[key]; existing != nil {
			existing.targets = expandAffectedSurfaceTargets(append(existing.targets, bindingSurfaceTargets(binding)...))
		} else {
			request.targets = expandAffectedSurfaceTargets(bindingSurfaceTargets(binding))
			requests[key] = &request
		}
	}
	_, detected, err := preflightSelectedTargets(ctx, app, targets, nil, !opts.dryRun && installation.OriginMode == domain.OriginModeDirectory, lifecycleInstallIntents(installation, opts.scope, nil))
	if err != nil {
		return err
	}
	writeProgress(app, opts.format, "Resolving and validating each unique exact installed package revision once...")
	keys := make([]string, 0, len(requests))
	for key := range requests {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	loadedByRevision := make(map[string]loadedPackage, len(keys))
	for _, key := range keys {
		request := requests[key]
		var loaded loadedPackage
		loaded, err = resolveRepairSource(ctx, repairSourceResolutionTimeout, func(resolutionCtx context.Context) (loadedPackage, error) {
			if installation.OriginMode == domain.OriginModeDirectory {
				return app.loadInstalledPackage(resolutionCtx, installation, request.targets, domain.DirectoryRepair, request.sequence, request.revision, detected)
			}
			return app.loadPackageFor(resolutionCtx, request.source, packageResolutionRequest{Targets: request.targets, Operation: domain.DirectoryRepair, Clients: detected})
		})
		if err != nil {
			return err
		}
		loadedByRevision[key] = loaded
		if loaded.cleanup != nil {
			defer loaded.cleanup()
		}
	}
	for _, key := range keys {
		loaded := loadedByRevision[key]
		if err := authorizeSecurityAssessment(cmd, app, opts, &loaded); err != nil {
			return err
		}
		loadedByRevision[key] = loaded
	}
	service := lifecycleService(app, detected)
	inputs := make([]usecase.AddInput, 0, len(targets))
	result := repairMultiResult{Batch: true, Status: "planned", Plugin: installation.DeclaredName, DryRun: opts.dryRun, Targets: make([]repairTargetResult, 0, len(targets))}
	for _, target := range targets {
		loaded := loadedByRevision[targetRevision[target]]
		clientPackage := cloneLoadedPackage(loaded)
		restoreCatalogEvidence(&clientPackage, recordedBindings[target])
		if err := prepareLoadedPackageForClient(&clientPackage, target); err != nil {
			return fmt.Errorf("preflight target %s: %w; no target was changed", target, err)
		}
		client := detected[target]
		input := usecase.AddInput{
			Envelope: clientPackage.envelope, Client: client, Scope: domain.ScopeUser,
			Interactive: false, Hints: clientPackage.hints,
			InstallationID:    installation.InstallationID,
			BackendExecutable: backendExecutable(client, detected),
			OriginMode:        loaded.origin, DirectoryResolution: cloneDirectoryOrigin(loaded.directory),
			DistributionSuspended: loaded.distributionSuspended, ReleaseRevoked: loaded.releaseRevoked,
		}
		inputs = append(inputs, input)
	}
	operationID, err := newOperationGroupID()
	if err != nil {
		return err
	}
	result.OperationID = operationID
	planned, err := service.RepairGroup(ctx, usecase.GroupInput{Targets: inputs, OperationGroupID: operationID, DryRun: true, Repair: true})
	for index, targetResult := range planned.Targets {
		output := newRepairResultData(installation, targetResult, true)
		output.OperationID = operationID
		result.Targets = append(result.Targets, repairTargetResult{Target: string(targets[index]), Status: string(targetResult.Plan.Status), NextAction: output.NextAction, Output: output})
	}
	result.Succeeded = len(planned.Targets)
	if err != nil {
		result.Status, result.Failed, result.Succeeded = "preflight_failed", len(inputs), 0
		_ = renderRepairMultiResult(cmd, opts, result)
		return fmt.Errorf("group repair preflight failed; no target was changed: %w", err)
	}
	if opts.dryRun {
		return renderRepairMultiResult(cmd, opts, result)
	}
	result.Status = "applying"
	result.Succeeded = 0
	result.Failed = 0
	result.Targets = result.Targets[:0]
	appliedGroup, groupErr := service.RepairGroup(ctx, usecase.GroupInput{Targets: inputs, OperationGroupID: operationID, Confirmed: true, Repair: true})
	appliedResults := appliedGroup.Targets
	if len(appliedResults) > len(inputs) || len(appliedResults) != len(inputs) && groupErr == nil {
		return fmt.Errorf("install engine returned %d repair results for %d targets", len(appliedResults), len(inputs))
	}
	for index, applied := range appliedResults {
		output := newRepairResultData(installation, applied, false)
		output.OperationID = operationID
		result.Targets = append(result.Targets, repairTargetResult{Target: string(targets[index]), Status: groupTargetStatus(applied), NextAction: output.NextAction, Output: output})
		if applied.GroupPhase == usecase.GroupTargetExternalCompleted {
			result.Succeeded++
		}
	}
	if groupErr != nil {
		result.Status, result.Failed = groupFailureStatus(appliedGroup.Phase), len(inputs)-result.Succeeded
		_ = renderRepairMultiResult(cmd, opts, result)
		return groupErr
	}
	result.Status = string(appliedGroup.Phase)
	return renderRepairMultiResult(cmd, opts, result)
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
	if result.Status == "completed" {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Managed package repaired from the exact installed revision and its package digest verified; external client activation and authentication were not reverified."); err != nil {
			return err
		}
	}
	return nil
}
