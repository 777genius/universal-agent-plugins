package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

type addTargetResult struct {
	Target       string                      `json:"target"`
	Status       string                      `json:"status"`
	NextAction   string                      `json:"next_action,omitempty"`
	Error        *usecase.GroupTargetFailure `json:"error,omitempty"`
	RetryCommand string                      `json:"retry_command,omitempty"`
	Output       addResultData               `json:"output"`
	displayName  string
}

type addMultiResult struct {
	OperationID    string                     `json:"operation_id,omitempty"`
	Batch          bool                       `json:"batch"`
	Status         string                     `json:"status"`
	Succeeded      int                        `json:"succeeded"`
	Failed         int                        `json:"failed"`
	ActionRequired int                        `json:"action_required,omitempty"`
	Plugin         string                     `json:"plugin"`
	Version        string                     `json:"version,omitempty"`
	Source         string                     `json:"source"`
	Revision       string                     `json:"revision,omitempty"`
	TreeDigest     string                     `json:"tree_digest,omitempty"`
	ManifestDigest string                     `json:"manifest_digest,omitempty"`
	Security       *domain.SecurityAssessment `json:"security,omitempty"`
	Directory      *domain.DirectoryOrigin    `json:"directory,omitempty"`
	DryRun         bool                       `json:"dry_run"`
	Targets        []addTargetResult          `json:"targets"`
	Acquisition    *addAcquisitionProof       `json:"acquisition,omitempty"`
	TargetOutcomes map[string]addTargetProof  `json:"target_outcomes,omitempty"`
}

type batchPresentationKind string

const (
	batchPresentationInstalled      batchPresentationKind = "Installed"
	batchPresentationAttested       batchPresentationKind = "User-attested"
	batchPresentationSetupRequired  batchPresentationKind = "Setup required"
	batchPresentationSignInRequired batchPresentationKind = "Sign-in required"
	batchPresentationFailed         batchPresentationKind = "Failed"
	batchPresentationNotCompleted   batchPresentationKind = "Not completed"
	batchPresentationRolledBack     batchPresentationKind = "Rolled back"
)

type addAcquisitionProof struct {
	AcquisitionID    string `json:"acquisition_id"`
	AcquisitionCount int    `json:"acquisition_count"`
	TreeDigest       string `json:"tree_digest"`
	ManifestDigest   string `json:"manifest_digest"`
	ClosureDigest    string `json:"closure_digest"`
	SourceKind       string `json:"source_kind"`
	Fetched          bool   `json:"fetched"`
	Validated        bool   `json:"validated"`
}

type addTargetProof struct {
	Outcome        string `json:"outcome"`
	AcquisitionID  string `json:"acquisition_id"`
	TreeDigest     string `json:"tree_digest"`
	ManifestDigest string `json:"manifest_digest"`
	ClosureDigest  string `json:"closure_digest"`
}

// runAddMany is deliberately not implemented as repeated CLI invocations. It
// resolves and validates one package, detects clients once, builds every plan,
// and only begins applying after the complete selected set passes preflight.
func runAddMany(ctx context.Context, cmd *cobra.Command, app App, opts *options, source string, targets []domain.ClientID, activationComplete, authComplete bool) error {
	return runAddManyWithClients(ctx, cmd, app, opts, source, targets, activationComplete, authComplete, nil)
}

func runAddManyWithClients(ctx context.Context, cmd *cobra.Command, app App, opts *options, source string, targets []domain.ClientID, activationComplete, authComplete bool, clients []domain.DetectedClient) error {
	_, detected, err := preflightAddTargets(ctx, app, opts, source, targets, clients)
	if err != nil {
		return err
	}
	writeProgress(app, opts.format, "Resolving and validating one Agent Plugin package for every selected target...")
	loaded, err := app.loadPackageFor(ctx, source, withDetectedClients(app.addResolutionRequest(source, targets), detected))
	if err != nil {
		return err
	}
	if loaded.cleanup != nil {
		defer loaded.cleanup()
	}
	return runAddManyLoaded(ctx, cmd, app, opts, loaded, targets, activationComplete, authComplete, detectedClientValues(detected), false)
}

func runAddManyLoaded(ctx context.Context, cmd *cobra.Command, app App, opts *options, loaded loadedPackage, targets []domain.ClientID, activationComplete, authComplete bool, clients []domain.DetectedClient, needsInstallConfirmation bool) error {
	if err := rejectImperativePackageRequirements(loaded); err != nil {
		return err
	}

	applyLoadedGuidedIntents(opts, loaded, targets)
	if err := revalidateLoadedTargetIntents(ctx, app, opts, loaded, targets, clients); err != nil {
		return err
	}
	if opts.chatGPTAppID != "" && !loaded.chatGPTPreparation {
		return fmt.Errorf("--chatgpt-app-id requires the signed Context7 Directory source and --target chatgptweb")
	}
	if err := authorizeSecurityAssessment(cmd, app, opts, &loaded); err != nil {
		return err
	}
	if useInstallerFacadeForLocalAdd(app, opts, loaded, targets, activationComplete, authComplete, needsInstallConfirmation, clients) {
		return runInstallerFacadeLocalAdd(ctx, cmd, app, opts, loaded, clients)
	}
	deferredChatGPT := loaded.chatGPTPreparation && loaded.localChatGPTMapping == nil && containsPersonalMappingTarget(targets)
	if deferredChatGPT && len(targets) == 1 {
		// Setup-required ChatGPT registration is not an install failure.
		personal, _ := personalMappingTarget(targets)
		action := chatGPTRegistrationResumeAction(cmd, loaded.envelope.Source.RequestedSource, []domain.ClientID{personal})
		if opts.format == "json" {
			return writeJSONResult(cmd.OutOrStdout(), "add", outputResultSuccess, map[string]any{
				"status": "action_required", "target": "chatgpt", "mcp_url": domain.Context7ChatGPTURL,
				"authentication": "none", "next_action": action, "remote_verified": true, "mutated": false,
			})
		}
		_, err := fmt.Fprintln(cmd.OutOrStdout(), action)
		return err
	}
	executionTargets := targets
	if deferredChatGPT {
		executionTargets = withoutPersonalMappingTargets(targets)
	}
	if len(executionTargets) == 1 && !deferredChatGPT {
		selectedOptions := *opts
		selectedOptions.target = string(executionTargets[0])
		if activationComplete || authComplete {
			return runAddLoaded(ctx, cmd, app, &selectedOptions, loaded, activationComplete, authComplete, clients, needsInstallConfirmation)
		}
		if state, err := app.StateStore.Load(); err == nil {
			if installation, ok := locallyMatchedInstallation(state, loaded.envelope.Manifest.Name); ok && installationHasTarget(installation, executionTargets[0], string(app.scope())) {
				return runAddLoaded(ctx, cmd, app, &selectedOptions, loaded, false, false, clients, needsInstallConfirmation)
			}
		}
	}
	combined := addMultiResult{
		Batch: true, Status: "planned", Plugin: loaded.envelope.Manifest.Name,
		Version: loaded.envelope.Manifest.Version, Source: publicPackageSource(loaded.envelope.Source),
		Revision: loaded.envelope.Source.ResolvedRevision, TreeDigest: loaded.envelope.TreeDigest, ManifestDigest: loaded.envelope.ManifestDigest,
		Security:  loaded.security,
		Directory: cloneDirectoryOrigin(loaded.directory),
		DryRun:    opts.dryRun, Targets: make([]addTargetResult, 0, len(targets)),
	}
	selected, detected, err := preflightSelectedTargets(ctx, app, executionTargets, clients, false)
	if err != nil {
		return err
	}
	combined.Targets = combined.Targets[:0]
	service := lifecycleService(app, detected)
	service.PrepareHostsForPreview = !opts.dryRun
	inputs := make([]usecase.AddInput, len(selected))
	for index, client := range selected {
		clientPackage := cloneLoadedPackage(loaded)
		if err := prepareLoadedPackageForClient(&clientPackage, client.ClientID); err != nil {
			return fmt.Errorf("preflight target %s: %w; no target was changed", client.ClientID, err)
		}
		input := usecase.AddInput{
			InstallIntent: opts.installIntents[client.ClientID],
			Envelope:      clientPackage.envelope, Client: client, Scope: app.scope(),
			DryRun: true, Interactive: app.Terminal, Hints: clientPackage.hints,
			BackendExecutable:  backendExecutable(client, detected, app.scope()),
			ActivationComplete: activationComplete, AuthComplete: authComplete,
			PersistAuthoritativeObservations: false,
			OriginMode:                       loaded.origin, DirectoryResolution: cloneDirectoryOrigin(loaded.directory),
			DistributionSuspended: loaded.distributionSuspended, ReleaseRevoked: loaded.releaseRevoked,
		}
		inputs[index] = input
	}
	operationID, err := newOperationGroupID()
	if err != nil {
		return err
	}
	combined.OperationID = operationID
	groupInput := usecase.GroupInput{Targets: inputs, OperationGroupID: operationID, DryRun: true, OnlinePreview: !opts.dryRun}
	planned, err := service.AddGroup(ctx, groupInput)
	combined.Targets = combined.Targets[:0]
	for index, result := range planned.Targets {
		output := newAddResultData(inputs[index].Envelope, result, true)
		output.OperationID = operationID
		combined.Targets = append(combined.Targets, addTargetResult{
			Target: string(selected[index].ClientID), Status: groupTargetStatus(result), Output: output,
			NextAction: nextLifecycleAction(result), displayName: clientDisplayName(selected[index]),
		})
		combined.setTargetProof(selected[index].ClientID, "not_run")
	}
	combined.Succeeded = len(planned.Targets)
	if err != nil {
		combined.Status, combined.Failed, combined.Succeeded = "preflight_failed", len(inputs), 0
		setPreflightNextActions(combined.Targets)
		_ = renderAddMultiResult(cmd, opts, combined, loaded.envelope)
		return fmt.Errorf("group preflight failed; no target was changed (selected targets: %v): %w%s", targets, err, addGroupNextAction(combined.Targets))
	}
	if opts.dryRun {
		if deferredChatGPT {
			appendDeferredChatGPT(&combined, cmd, loaded)
		}
		return renderAddMultiResult(cmd, opts, combined, loaded.envelope)
	}
	if needsInstallConfirmation && !opts.yes {
		accepted, err := confirmInstall(ctx, cmd, app, loaded, humanAddGroupPlans(combined.Targets))
		if err != nil {
			return err
		}
		if !accepted {
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "Installation not applied.")
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(targets) > 1 {
		proof, proofErr := newAddAcquisitionProof(loaded)
		if proofErr != nil {
			return proofErr
		}
		combined.Acquisition = &proof
		combined.TargetOutcomes = make(map[string]addTargetProof, len(selected))
		for _, client := range selected {
			combined.setTargetProof(client.ClientID, "not_run")
		}
	}
	writeProgress(app, opts.format, fmt.Sprintf("Installing %s for %d selected clients...", prompt.SafeText(loaded.envelope.Manifest.Name), len(selected)))
	retainPreparedClients(groupInput.Targets, planned.PreparedClients)
	groupInput.OnlinePreview = false
	groupInput.DryRun, groupInput.Confirmed = false, true
	board := startGroupProgressBoard(app, opts, selected)
	if board != nil {
		groupInput.Progress = board.observe
	} else if app.Terminal && opts.format == "human" {
		names := make(map[domain.ClientID]string, len(selected))
		for _, client := range selected {
			names[client.ClientID] = prompt.SafeText(reviewClientName(client.ClientID))
		}
		groupInput.Progress = func(event usecase.GroupProgressEvent) {
			name := names[event.ClientID]
			if name == "" {
				name = prompt.SafeText(string(event.ClientID))
			}
			var stage string
			switch event.Phase {
			case usecase.GroupProgressPreparing:
				stage = "1/3  preparing package..."
			case usecase.GroupProgressConfiguring:
				stage = "2/3  configuring clients..."
			case usecase.GroupProgressConfigured:
				stage = "2/3  configuration ready"
			case usecase.GroupProgressActivated:
				stage = "3/3  " + progressResultLabel(event.Result)
			default:
				return
			}
			_, _ = fmt.Fprintf(app.errorOutput(), "%s  %s\n", name, stage)
		}
	}
	applied, err := service.AddGroup(ctx, groupInput)
	if board != nil {
		board.finish(applied.Targets)
	}
	if len(applied.Targets) != len(selected) || len(applied.Targets) != len(inputs) {
		if err != nil {
			return fmt.Errorf("group apply returned %d targets for %d selected clients: %w", len(applied.Targets), len(selected), err)
		}
		return fmt.Errorf("group apply returned %d targets for %d selected clients", len(applied.Targets), len(selected))
	}
	combined.Status, combined.Targets, combined.Succeeded, combined.Failed, combined.ActionRequired = string(applied.Phase), combined.Targets[:0], 0, 0, 0
	sourceArg := batchRetrySource(loaded.envelope, loaded.origin, combined.Source)
	for index, result := range applied.Targets {
		output := newAddResultData(inputs[index].Envelope, result, false)
		output.OperationID = operationID
		entry := addTargetResult{
			Target: string(selected[index].ClientID), Status: groupTargetStatus(result), Output: output,
			NextAction: nextLifecycleAction(result), displayName: clientDisplayName(selected[index]),
			Error: result.Failure,
		}
		if retry := batchRetryCommandFor(entry, sourceArg); retry != "" {
			entry.RetryCommand = retry
		}
		combined.Targets = append(combined.Targets, entry)
		combined.setTargetProof(selected[index].ClientID, addTargetProofOutcome(result))
		countBatchTarget(&combined, entry)
	}
	if err != nil {
		if applied.Phase == usecase.GroupPhasePlanned && !applied.Mutated {
			combined.Status, combined.Failed, combined.Succeeded, combined.ActionRequired = "preflight_failed", len(inputs), 0, 0
			setPreflightNextActions(combined.Targets)
			_ = renderAddMultiResult(cmd, opts, combined, loaded.envelope)
			return fmt.Errorf("group apply preflight failed; no target was changed (selected targets: %v): %w%s", targets, err, addGroupNextAction(combined.Targets))
		}
		if applied.Phase == usecase.GroupPhasePreparationFailed && !applied.Mutated {
			combined.Status, combined.Failed, combined.Succeeded, combined.ActionRequired = "preparation_failed", len(inputs), 0, 0
			for index := range combined.Targets {
				combined.Targets[index].Status = string(usecase.GroupTargetExternalNotAttempted)
				combined.Targets[index].NextAction = "resolve the preparation error and retry the same command"
				combined.Targets[index].Output.NextAction = combined.Targets[index].NextAction
				combined.Targets[index].Error = &usecase.GroupTargetFailure{Stage: "preparation", Message: err.Error()}
				combined.setTargetProof(domain.ClientID(combined.Targets[index].Target), "not_completed")
			}
			_ = renderAddMultiResult(cmd, opts, combined, loaded.envelope)
			return fmt.Errorf("group preparation failed before client changes (selected targets: %v): %w; no selected client was changed", targets, err)
		}
		combined.Status = groupFailureStatus(applied.Phase)
		// Always report selected ChatGPT setup, even when installable peers failed.
		if deferredChatGPT {
			appendDeferredChatGPT(&combined, cmd, loaded)
		}
		_ = renderAddMultiResult(cmd, opts, combined, loaded.envelope)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		if applied.Phase == usecase.GroupPhaseManagedActivationFailed || applied.Phase == usecase.GroupPhaseExternalPartialFailure {
			return batchActivationAggregateError(combined)
		}
		return err
	}
	if deferredChatGPT {
		// Manual ChatGPT registration stays outside the atomic local-client group.
		appendDeferredChatGPT(&combined, cmd, loaded)
	}
	if err := renderAddMultiResult(cmd, opts, combined, loaded.envelope); err != nil {
		return err
	}
	if combined.Failed > 0 {
		return batchActivationAggregateError(combined)
	}
	if app.Terminal && opts.format == "human" && !opts.yes && len(applied.Targets) == 1 && !deferredChatGPT {
		input := inputs[0]
		input.DryRun = false
		input.InstallationID = applied.Targets[0].InstallationID
		return resumeInteractiveLifecycle(ctx, cmd, service, input, inputs[0].Envelope, applied.Targets[0])
	}
	return nil
}

func renderAddMultiApplySummary(writer io.Writer, result addMultiResult, envelope domain.PackageEnvelope) error {
	theme := terminaltheme.For(writer)
	title := batchApplyHeadline(result, envelope)
	if _, err := fmt.Fprintln(writer, theme.Text(terminaltheme.Label, title)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer); err != nil {
		return err
	}
	width := utf8.RuneCountInString("Client")
	for _, target := range result.Targets {
		if name := batchTargetLabel(target); utf8.RuneCountInString(name) > width {
			width = utf8.RuneCountInString(name)
		}
	}
	if _, err := fmt.Fprintf(writer, "%-*s  %s\n", width, "Client", "Result"); err != nil {
		return err
	}
	for _, target := range result.Targets {
		label := batchTargetLabel(target)
		kind := classifyBatchPresentation(target)
		if _, err := fmt.Fprintf(writer, "%-*s  %s\n", width, prompt.SafeText(label), theme.Text(batchResultTone(kind), string(kind))); err != nil {
			return err
		}
	}
	attention := make([]addTargetResult, 0, len(result.Targets))
	for _, target := range result.Targets {
		switch classifyBatchPresentation(target) {
		case batchPresentationAttested, batchPresentationSetupRequired, batchPresentationSignInRequired, batchPresentationFailed, batchPresentationNotCompleted, batchPresentationRolledBack:
			attention = append(attention, target)
		}
	}
	if len(attention) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(writer); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, theme.Text(terminaltheme.Warning, "Needs attention")); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer); err != nil {
		return err
	}
	for index, target := range attention {
		if index > 0 {
			if _, err := fmt.Fprintln(writer); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(writer, prompt.SafeText(batchTargetLabel(target))); err != nil {
			return err
		}
		if err := renderOpenCodeRuntimeNotice(writer, target.Output.Result); err != nil {
			return err
		}
		for _, line := range batchAttentionLines(target) {
			for _, part := range strings.Split(line, "\n") {
				if strings.TrimSpace(part) == "" {
					continue
				}
				if _, err := fmt.Fprintf(writer, "  %s\n", prompt.SafeText(part)); err != nil {
					return err
				}
			}
		}
		if target.RetryCommand != "" {
			if _, err := fmt.Fprintln(writer, "  Retry:"); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(writer, "    %s\n", prompt.SafeText(target.RetryCommand)); err != nil {
				return err
			}
		}
	}
	return nil
}
