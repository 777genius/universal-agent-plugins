package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	"github.com/spf13/cobra"
)

type updateTargetResult struct {
	Target     string        `json:"target"`
	Selected   bool          `json:"selected"`
	Status     string        `json:"status"`
	NextAction string        `json:"next_action,omitempty"`
	Output     addResultData `json:"output"`
}

type updateMultiResult struct {
	OperationID string               `json:"operation_id,omitempty"`
	Batch       bool                 `json:"batch"`
	Status      string               `json:"status"`
	Succeeded   int                  `json:"succeeded"`
	Failed      int                  `json:"failed"`
	Plugin      string               `json:"plugin"`
	Version     string               `json:"version,omitempty"`
	Source      string               `json:"source"`
	Revision    string               `json:"revision,omitempty"`
	TreeDigest  string               `json:"tree_digest,omitempty"`
	DryRun      bool                 `json:"dry_run"`
	Targets     []updateTargetResult `json:"targets"`
}

type preparedUpdateMany struct {
	loaded        loadedPackage
	service       usecase.Service
	inputs        []usecase.AddInput
	compatibility []usecase.AddInput
	selected      []domain.ClientID
	result        updateMultiResult
	noChange      bool
}

func runUpdateMany(ctx context.Context, cmd *cobra.Command, app App, opts *options, selector string, targets []domain.ClientID) error {
	state, err := app.StateStore.Load()
	if err != nil {
		return err
	}
	installation, err := selectInstallation(state, selector)
	if err != nil {
		return err
	}
	prepared, err := prepareUpdateMany(ctx, app, opts, installation, targets, !opts.dryRun)
	if prepared != nil {
		defer prepared.cleanup()
	}
	if err != nil {
		if prepared != nil {
			_ = renderUpdateMultiResult(cmd, opts, prepared.result)
			return fmt.Errorf("group update preflight failed; no target was changed: %w%s", err, groupNextAction(prepared.result.Targets))
		}
		return err
	}
	if err := authorizeSecurityAssessment(cmd, app, opts, &prepared.loaded); err != nil {
		return err
	}
	if opts.dryRun {
		return renderUpdateMultiResult(cmd, opts, prepared.result)
	}
	result, applyErr := applyPreparedUpdate(ctx, prepared)
	if renderErr := renderUpdateMultiResult(cmd, opts, result); renderErr != nil && applyErr == nil {
		applyErr = renderErr
	}
	return applyErr
}

func prepareUpdateMany(ctx context.Context, app App, opts *options, installation domain.Installation, targets []domain.ClientID, probeVersion bool) (*preparedUpdateMany, error) {
	if installation.NeedsRebind || installation.Package.LoaderKind != domain.LoaderKindAgentPlugins {
		return nil, fmt.Errorf("update requires a bound Agent Plugins installation")
	}
	for _, target := range targets {
		if !installationHasTarget(installation, target, opts.scope) {
			return nil, fmt.Errorf("plugin is not installed for target %q in %s scope; no target was changed", target, opts.scope)
		}
	}
	allTargets := installationTargets(installation, opts.scope)
	_, detected, err := preflightSelectedTargets(ctx, app, targets, nil, probeVersion && installation.OriginMode == domain.OriginModeDirectory, lifecycleInstallIntents(installation, opts.scope, nil))
	if err != nil {
		return nil, err
	}
	installedClients, err := preflightInstalledBindings(installedBindingTargets(installation, opts.scope), detected)
	if err != nil {
		return nil, err
	}
	writeProgress(app, opts.format, "Resolving and validating one updated Agent Plugin package for every selected target...")
	operation := domain.DirectoryUpdate
	if selectedTargetsNeedDesiredRelease(installation, targets, opts.scope) {
		// A prior partial rollout already accepted the installation-wide desired
		// release. Resolve that exact release so remaining bindings converge to it
		// instead of incorrectly requiring a still newer release.
		operation = domain.DirectoryNewTarget
	}
	loaded, err := app.loadInstalledPackage(ctx, installation, allTargets, operation, 0, installation.Source.ResolvedRevision, detected)
	if operation == domain.DirectoryUpdate && errors.Is(err, domain.ErrDirectoryNoSafeUpdate) {
		// "No later release" means the tracked immutable release is already the
		// newest safe choice, not that update itself is invalid. Resolve that
		// exact recorded release again so the lifecycle can report an idempotent
		// no-change result while retaining revocation and integrity checks.
		loaded, err = app.loadInstalledPackage(ctx, installation, allTargets, domain.DirectoryNewTarget, 0, installation.Source.ResolvedRevision, detected)
	}
	if err != nil {
		return nil, err
	}
	prepared := &preparedUpdateMany{loaded: loaded}
	fail := func(err error) (*preparedUpdateMany, error) {
		prepared.cleanup()
		return nil, err
	}
	if installation.OriginMode != domain.OriginModeDirectory && domain.ComputeSourceBindingID(loaded.envelope.Source) != installation.Source.SourceBindingID {
		return fail(fmt.Errorf("resolved source identity changed; use agentplugins switch after reviewing provenance"))
	}
	service := lifecycleService(app, detected)
	inputs := make([]usecase.AddInput, 0, len(targets))
	compatibility := make([]usecase.AddInput, 0, len(installedClients))
	selected := make([]domain.ClientID, 0, len(targets))
	result := updateMultiResult{
		Batch: true, Status: "planned", Plugin: loaded.envelope.Manifest.Name,
		Version: loaded.envelope.Manifest.Version, Source: publicPackageSource(loaded.envelope.Source),
		Revision: loaded.envelope.Source.ResolvedRevision, TreeDigest: loaded.envelope.TreeDigest,
		DryRun: opts.dryRun, Targets: make([]updateTargetResult, 0, len(targets)),
	}
	for _, client := range installedClients {
		target := client.ClientID
		clientPackage := cloneLoadedPackage(loaded)
		if err := prepareLoadedPackageForClient(&clientPackage, target); err != nil {
			return fail(fmt.Errorf("preflight target %s: %w; no target was changed", target, err))
		}
		input := usecase.AddInput{
			Envelope: clientPackage.envelope, Client: client, Scope: domain.ScopeUser,
			Interactive: false, Hints: clientPackage.hints, BackendExecutable: backendExecutable(client, detected),
			OriginMode: loaded.origin, DirectoryResolution: cloneDirectoryOrigin(loaded.directory),
			DistributionSuspended: loaded.distributionSuspended, ReleaseRevoked: loaded.releaseRevoked,
		}
		compatibility = append(compatibility, input)
	}
	for _, target := range targets {
		client := detected[target]
		clientPackage := cloneLoadedPackage(loaded)
		if err := prepareLoadedPackageForClient(&clientPackage, target); err != nil {
			return fail(fmt.Errorf("preflight target %s: %w; no target was changed", target, err))
		}
		inputs = append(inputs, usecase.AddInput{
			Envelope: clientPackage.envelope, Client: client, Scope: domain.ScopeUser,
			Interactive: false, Hints: clientPackage.hints, BackendExecutable: backendExecutable(client, detected),
			OriginMode: loaded.origin, DirectoryResolution: cloneDirectoryOrigin(loaded.directory),
			DistributionSuspended: loaded.distributionSuspended, ReleaseRevoked: loaded.releaseRevoked,
		})
		selected = append(selected, target)
	}
	operationID, err := newOperationGroupID()
	if err != nil {
		return fail(err)
	}
	result.OperationID = operationID
	planned, err := service.UpdateGroup(ctx, usecase.GroupInput{Targets: inputs, CompatibilityChecks: compatibility, OperationGroupID: operationID, DryRun: true})
	for index, targetResult := range planned.Targets {
		output := newAddResultData(inputs[index].Envelope, targetResult, true)
		output.OperationID = operationID
		result.Targets = append(result.Targets, updateTargetResult{Target: string(selected[index]), Selected: true, Status: groupTargetStatus(targetResult), Output: output, NextAction: nextLifecycleAction(targetResult)})
	}
	result.Succeeded = len(planned.Targets)
	prepared.service, prepared.inputs, prepared.compatibility = service, inputs, compatibility
	prepared.selected, prepared.result = selected, result
	prepared.noChange = len(planned.Targets) > 0
	for _, targetResult := range planned.Targets {
		if !targetResult.NoChange {
			prepared.noChange = false
			break
		}
	}
	if err != nil {
		result.Status, result.Failed, result.Succeeded = "preflight_failed", len(inputs), 0
		prepared.result = result
		return prepared, err
	}
	return prepared, nil
}

func applyPreparedUpdate(ctx context.Context, prepared *preparedUpdateMany) (updateMultiResult, error) {
	result := prepared.result
	applied, groupErr := prepared.service.UpdateGroup(ctx, usecase.GroupInput{Targets: prepared.inputs, CompatibilityChecks: prepared.compatibility, OperationGroupID: result.OperationID, Confirmed: true})
	result.Status, result.Targets, result.Succeeded = string(applied.Phase), result.Targets[:0], 0
	for index, targetResult := range applied.Targets {
		output := newAddResultData(prepared.inputs[index].Envelope, targetResult, false)
		output.OperationID = result.OperationID
		result.Targets = append(result.Targets, updateTargetResult{Target: string(prepared.selected[index]), Selected: true, Status: groupTargetStatus(targetResult), Output: output, NextAction: nextLifecycleAction(targetResult)})
		if targetResult.GroupPhase == usecase.GroupTargetExternalCompleted {
			result.Succeeded++
		}
	}
	if groupErr != nil {
		result.Status, result.Failed = groupFailureStatus(applied.Phase), len(prepared.inputs)-result.Succeeded
		return result, groupErr
	}
	return result, nil
}

func (prepared *preparedUpdateMany) cleanup() {
	if prepared == nil || prepared.loaded.cleanup == nil {
		return
	}
	_ = prepared.loaded.cleanup()
	prepared.loaded.cleanup = nil
}

func installationHasTarget(installation domain.Installation, target domain.ClientID, scope string) bool {
	for _, binding := range installation.Clients {
		if binding.Scope == scope && binding.Materialization != domain.MaterializationAbsent && bindingAffectsTarget(binding, target) {
			return true
		}
	}
	return false
}

func installationTargets(installation domain.Installation, scope string) []domain.ClientID {
	seen := make(map[domain.ClientID]struct{}, len(installation.Clients))
	result := make([]domain.ClientID, 0, len(installation.Clients))
	for _, binding := range installation.Clients {
		if binding.Scope != scope || binding.Materialization == domain.MaterializationAbsent {
			continue
		}
		for _, target := range bindingSurfaceTargets(binding) {
			if !supportedTarget(target) {
				continue
			}
			if _, duplicate := seen[target]; duplicate {
				continue
			}
			seen[target] = struct{}{}
			result = append(result, target)
		}
	}
	sortTargets(result)
	return result
}

func installedBindingTargets(installation domain.Installation, scope string) []domain.ClientID {
	seen := make(map[domain.ClientID]struct{}, len(installation.Clients))
	result := make([]domain.ClientID, 0, len(installation.Clients))
	for _, binding := range installation.Clients {
		target := domain.ClientID(binding.ClientID)
		if binding.Scope != scope || binding.Materialization == domain.MaterializationAbsent || !supportedTarget(target) {
			continue
		}
		if _, duplicate := seen[target]; duplicate {
			continue
		}
		seen[target] = struct{}{}
		result = append(result, target)
	}
	sortTargets(result)
	return result
}

func bindingAffectsTarget(binding domain.ClientBinding, target domain.ClientID) bool {
	for _, surface := range bindingSurfaceTargets(binding) {
		if surface == target {
			return true
		}
	}
	return false
}

func bindingSurfaceTargets(binding domain.ClientBinding) []domain.ClientID {
	// The physical binding's own client identity is always an affected logical
	// surface, including when reading an older or partially written surface list.
	// Union it with the persisted list so target selection and compatibility
	// preflight can heal omissions instead of silently perpetuating them.
	values := append([]string(nil), binding.AffectedSurfaces...)
	values = append(values, binding.ClientID)
	if domain.ClientID(binding.ClientID) == domain.ClientCopilot || domain.ClientID(binding.ClientID) == domain.ClientVSCode {
		values = append(values, string(domain.ClientCopilot), string(domain.ClientVSCode))
	}
	result := make([]domain.ClientID, 0, len(values))
	seen := map[domain.ClientID]struct{}{}
	for _, value := range values {
		target := domain.ClientID(value)
		if _, ok := seen[target]; ok {
			continue
		}
		seen[target] = struct{}{}
		result = append(result, target)
	}
	return result
}

func expandAffectedSurfaceTargets(targets []domain.ClientID) []domain.ClientID {
	result := append([]domain.ClientID(nil), targets...)
	for _, target := range targets {
		if target == domain.ClientCopilot || target == domain.ClientVSCode {
			result = append(result, domain.ClientCopilot, domain.ClientVSCode)
			break
		}
	}
	seen := make(map[domain.ClientID]struct{}, len(result))
	unique := result[:0]
	for _, target := range result {
		if _, ok := seen[target]; ok {
			continue
		}
		seen[target] = struct{}{}
		unique = append(unique, target)
	}
	sortTargets(unique)
	return unique
}

func selectedTargetsNeedDesiredRelease(installation domain.Installation, targets []domain.ClientID, scope string) bool {
	if installation.OriginMode != domain.OriginModeDirectory || installation.Directory == nil {
		return false
	}
	desired := installation.Directory.DesiredReleaseSequence
	for _, target := range targets {
		for _, binding := range installation.Clients {
			if binding.Scope != scope || binding.Materialization == domain.MaterializationAbsent || !bindingAffectsTarget(binding, target) || binding.PackageRevision == nil {
				continue
			}
			if binding.PackageRevision.DistributionID == installation.Directory.DistributionID && binding.PackageRevision.ReleaseSequence < desired {
				return true
			}
		}
	}
	return false
}

func renderUpdateMultiResult(cmd *cobra.Command, opts *options, result updateMultiResult) error {
	if opts.format == "json" {
		overall := "success"
		if result.Failed > 0 {
			overall = "failure"
		}
		return writeJSONResult(cmd.OutOrStdout(), "update", overall, result)
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Plugin: %s %s\n", result.Plugin, result.Version); err != nil {
		return err
	}
	values := make([]string, len(result.Targets))
	for index, target := range result.Targets {
		if err := renderOpenCodeRuntimeNotice(cmd.OutOrStdout(), target.Output.Result); err != nil {
			return err
		}
		values[index] = target.Target
		rollout := "preflight only"
		if target.Selected {
			rollout = "selected"
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s (%s)\n", target.Target, target.Status, rollout); err != nil {
			return err
		}
		if target.NextAction != "" && !fullyInstalled(target.Output.Result.Activation) {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "    Next: %s\n", localTargetLifecycleAction(target.Output.Result, target.NextAction)); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "Targets: %s\n", strings.Join(values, ","))
	return err
}

func groupNextAction(targets []updateTargetResult) string {
	for _, target := range targets {
		if target.NextAction != "" {
			return "; next action: " + target.NextAction
		}
	}
	return ""
}
