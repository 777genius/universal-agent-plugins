package agentpluginscli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func runAddLoaded(ctx context.Context, cmd *cobra.Command, app App, opts *options, loaded loadedPackage, activationComplete, authComplete bool, clients []domain.DetectedClient, needsInstallConfirmation bool) error {
	if err := rejectImperativePackageRequirements(loaded); err != nil {
		return err
	}

	targets, err := parseTargetOption(opts.target)
	if err != nil {
		return err
	}
	applyLoadedGuidedIntents(opts, loaded, targets)
	if opts.chatGPTAppID != "" && !loaded.chatGPTPreparation {
		return fmt.Errorf("--chatgpt-app-id requires the signed Context7 Directory source and --target chatgptweb")
	}
	if err := authorizeSecurityAssessment(cmd, app, opts, &loaded); err != nil {
		return err
	}
	if automatedMutation(app, opts) && strings.TrimSpace(opts.target) == "" {
		return fmt.Errorf("automated installation requires --target")
	}
	selected, detectedMap, err := selectClient(cmd, app, opts, clients)
	if err != nil {
		return err
	}
	if err := prepareLoadedPackageForClient(&loaded, selected.ClientID); err != nil {
		return err
	}
	service := lifecycleService(app, detectedMap)
	input := usecase.AddInput{
		InstallIntent: opts.installIntents[selected.ClientID],
		Envelope:      loaded.envelope, Client: selected, Scope: domain.InstallScope(opts.scope),
		DryRun: opts.dryRun, Confirmed: false, Interactive: app.Terminal,
		Hints: loaded.hints, BackendExecutable: backendExecutable(selected, detectedMap, app.scope()),
		ActivationComplete: activationComplete, AuthComplete: authComplete,
		PersistAuthoritativeObservations: true,
		OriginMode:                       loaded.origin, DirectoryResolution: cloneDirectoryOrigin(loaded.directory),
		DistributionSuspended: loaded.distributionSuspended, ReleaseRevoked: loaded.releaseRevoked,
	}
	planned, err := service.Add(ctx, input)
	if err != nil {
		if planned.Plan.Status == domain.PlanUnsupported {
			if renderErr := renderAddResultErrorWithSecurity(cmd.OutOrStdout(), opts.format, loaded.envelope, loaded.security, planned, opts.dryRun, err); renderErr != nil {
				return renderErr
			}
		}
		return err
	}
	if opts.dryRun || planned.NoChange {
		return renderAddResultWithSecurity(cmd.OutOrStdout(), opts.format, loaded.envelope, loaded.security, planned, opts.dryRun)
	}
	return applyPlannedAdd(ctx, cmd, app, opts, service, input, loaded, planned, activationComplete, authComplete, needsInstallConfirmation)
}

func applyPlannedAdd(ctx context.Context, cmd *cobra.Command, app App, opts *options, service usecase.Service, input usecase.AddInput, loaded loadedPackage, planned usecase.AddResult, activationComplete, authComplete, needsInstallConfirmation bool) error {
	retainPlannedClient(&input, planned.Plan)
	if opts.format == "human" {
		if err := renderHumanPlan(cmd.OutOrStdout(), loaded.envelope, planned); err != nil {
			return err
		}
	}
	freshInstall := planned.Activation.Activation == ""
	if !freshInstall && opts.format == "human" && app.Terminal && !opts.yes && !activationComplete && !authComplete {
		if err := renderAddResultWithSecurity(cmd.OutOrStdout(), opts.format, loaded.envelope, loaded.security, planned, false); err != nil {
			return err
		}
		return resumeInteractiveLifecycle(ctx, cmd, service, input, loaded.envelope, planned)
	}
	confirmed, err := confirmPlannedAdd(ctx, cmd, app, opts, loaded, planned, freshInstall, needsInstallConfirmation)
	if err != nil {
		return err
	}
	if !confirmed {
		return renderDeclinedPlannedAdd(cmd, opts, loaded, planned)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	writeProgress(app, opts.format, "Applying transactional client package...")
	input.Confirmed = true
	input.InstallationID = planned.InstallationID
	result, err := service.Add(ctx, input)
	if renderErr := renderAddResultErrorWithSecurity(cmd.OutOrStdout(), opts.format, loaded.envelope, loaded.security, result, false, err); renderErr != nil && err == nil {
		err = renderErr
	}
	if err == nil && freshInstall && opts.format == "human" && app.Terminal && !opts.yes {
		return resumeInteractiveLifecycle(ctx, cmd, service, input, loaded.envelope, result)
	}
	return err
}

// renderDeclinedPlannedAdd reports an unconfirmed plan without mutating anything.
func renderDeclinedPlannedAdd(cmd *cobra.Command, opts *options, loaded loadedPackage, planned usecase.AddResult) error {
	if opts.format == "json" {
		return renderAddResultWithSecurity(cmd.OutOrStdout(), opts.format, loaded.envelope, loaded.security, planned, false)
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No changes made.")
	return nil
}

func confirmPlannedAdd(ctx context.Context, cmd *cobra.Command, app App, opts *options, loaded loadedPackage, planned usecase.AddResult, freshInstall, needsInstallConfirmation bool) (bool, error) {
	confirmed := mutationConfirmed(app, opts) && (!needsInstallConfirmation || opts.yes)
	if needsInstallConfirmation && !opts.yes {
		return confirmInstall(ctx, cmd, app, loaded, []usecase.AddResult{planned})
	}
	if confirmed || opts.format != "human" || !app.Terminal {
		return confirmed, nil
	}
	question := "Apply this plan? [Y/n]"
	if !freshInstall {
		question = "Apply these explicit lifecycle attestations? [y/N]"
		return promptYesNo(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), question)
	}
	return promptYesNoDefault(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), question, true)
}

func promptTargetChoices(cmd *cobra.Command, app App, detected []domain.DetectedClient, skipped []targetSkip, allClients []domain.DetectedClient) ([]domain.ClientID, []domain.DetectedClient, error) {
	detected = append([]domain.DetectedClient(nil), detected...)
	skipped = append([]targetSkip(nil), skipped...)
	sort.SliceStable(detected, func(i, j int) bool { return targetOrder(detected[i].ClientID) < targetOrder(detected[j].ClientID) })
	sort.SliceStable(skipped, func(i, j int) bool { return targetOrder(skipped[i].Client) < targetOrder(skipped[j].Client) })
	request := prompt.TargetSelectionRequest{}
	for _, c := range detected {
		request.Choices = append(request.Choices, prompt.TargetChoice{ID: c.ClientID, Label: prompt.SafeText(c.DisplayName)})
		request.DefaultIDs = append(request.DefaultIDs, c.ClientID)
	}
	for _, c := range skipped {
		label := string(c.Client)
		if definition, ok := domain.ClientDefinitionFor(c.Client); ok {
			label = definition.DisplayName
		}
		request.SkippedLabels = append(request.SkippedLabels, prompt.SafeText(label+": "+c.Reason))
	}
	if len(detected) > 0 {
		if err := prompt.ValidateRequest(request); err != nil {
			return nil, nil, err
		}
	}
	personalPreparationChoice := len(detected) == 1 && requiresPersonalMapping(detected[0].ClientID) && strings.Contains(detected[0].DisplayName, "prepare personal marketplace")
	if len(detected) <= 1 && !personalPreparationChoice {
		if notice := prompt.SkippedClientsNotice(request.SkippedLabels); notice != "" {
			if _, err := fmt.Fprint(reviewWriter(cmd, app), notice); err != nil {
				return nil, nil, err
			}
		}
		if len(detected) == 0 {
			return nil, nil, fmt.Errorf("no eligible detected client; nothing was installed. Install/detect a supported client or address the skipped reasons. --target chatgpt checks package eligibility for ChatGPT; it does not register remote MCP or install the full plugin in ChatGPT Plugins")
		}
		return request.DefaultIDs, allClients, cmd.Context().Err()
	}
	if app.Prompter == nil {
		return nil, nil, prompt.ErrPromptUnavailable
	}
	result, err := app.Prompter.SelectTargets(cmd.Context(), request)
	if err != nil {
		return nil, nil, err
	}
	if err = cmd.Context().Err(); err != nil {
		return nil, nil, err
	}
	result, err = prompt.ValidateSelection(request, result.IDs)
	if err != nil {
		return nil, nil, err
	}
	return result.IDs, allClients, nil
}

// detectSelectedTargetsForLifecycleResolution keeps ambient discovery strictly
// read-only. Version execution is an opt-in capability invoked only after the
// user has selected the complete target set. Detectors without targeted probing
// retain the read-only observations rather than falling back to executing every
// discovered client binary.
func detectSelectedTargetsForLifecycleResolution(ctx context.Context, detector ports.ClientDetector, targets []domain.ClientID, detected []domain.DetectedClient, probeVersion bool) ([]domain.DetectedClient, error) {
	if !probeVersion || len(targets) == 0 {
		return detected, nil
	}
	if targeted, ok := detector.(ports.TargetedVersionProbingClientDetector); ok {
		return targeted.DetectTargetsWithVersionProbe(ctx, targets)
	}
	return detected, nil
}
