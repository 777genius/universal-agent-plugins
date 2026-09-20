package agentpluginscli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	"github.com/spf13/cobra"
)

func newAddCommand(app App, opts *options) *cobra.Command {
	var activationComplete, authComplete bool
	command := &cobra.Command{
		Use:     "add <name-or-source>",
		Aliases: []string{"install"},
		Short:   "Plan and install one Agent Plugins 1.0 package for one or more clients",
		Args:    addCommandArgs(opts),
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeAddCommand(cmd, app, opts, args[0], activationComplete, authComplete)
		},
	}
	command.Flags().StringVar(&opts.chatGPTAppID, "chatgpt-app-id", "", "personal Context7 registration ID copied from ChatGPT Developer Mode")
	command.Flags().BoolVar(&activationComplete, "activation-complete", false, "attest that manual client activation is complete")
	command.Flags().BoolVar(&authComplete, "auth-complete", false, "attest that required authentication is complete or none is required after review")
	return command
}

func addCommandArgs(opts *options) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(1)(cmd, args); err != nil {
			if len(args) == 0 && opts.format != "json" {
				return fmt.Errorf("%w\nProvide a plugin name or source, for example:\n  agentplugins add ./my-plugin\nSee agentplugins add --help", err)
			}
			return err
		}
		return nil
	}
}

func executeAddCommand(cmd *cobra.Command, app App, opts *options, source string, activationComplete, authComplete bool) error {
	if err := validateCommonOptions(opts); err != nil {
		return err
	}
	opts.installIntents = make(map[domain.ClientID]domain.InstallIntent)
	requestedTargets, err := parseTargetOption(opts.target)
	if err != nil {
		return err
	}
	opts.installIntents, err = app.addLifecycleIntents(cmd.Context(), source, opts.scope, opts.installIntents, requestedTargets)
	if err != nil {
		return err
	}
	app.chatGPTPreparation = personalMappingPrepareSelected(opts.installIntents)
	if opts.chatGPTAppID != "" {
		if err := domain.ValidateChatGPTAppID(opts.chatGPTAppID); err != nil {
			return err
		}
		app.chatGPTAppID = opts.chatGPTAppID
	}
	targetProvided := cmd.Flags().Changed("target") && strings.TrimSpace(opts.target) != ""
	if !targetProvided && app.Terminal && opts.format == "human" {
		return executePromptedAdd(cmd, app, opts, source, activationComplete, authComplete)
	}
	targets, err := parseTargetOption(opts.target)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return fmt.Errorf("automated installation requires --target")
	}
	return runAddManyWithClients(cmd.Context(), cmd, app, opts, source, targets, activationComplete, authComplete, nil)
}

func executePromptedAdd(cmd *cobra.Command, app App, opts *options, source string, activationComplete, authComplete bool) error {
	var err error
	app, err = withPrompter(cmd, app, opts)
	if err != nil {
		return err
	}
	selection, clients, preloaded, err := promptCompatibleDetectedTargets(cmd.Context(), cmd, app, source, opts.installIntents)
	if err != nil {
		return err
	}
	if preloaded != nil && preloaded.cleanup != nil {
		defer func() { _ = preloaded.cleanup() }()
	}
	if preloaded != nil && !containsPersonalMappingTarget(selection) {
		preloaded.chatGPTPreparation = false
		preloaded.localChatGPTMapping = nil
	}
	detectedClients, err := detectSelectedTargetsForLifecycleResolution(cmd.Context(), app.Detector, automaticInstallTargets(selection, opts.installIntents), clients, !opts.dryRun && isDirectorySelector(source))
	if err != nil {
		return fmt.Errorf("detect selected AI clients: %w", err)
	}
	_, detected, err := preflightSelectedTargets(cmd.Context(), app, selection, detectedClients, false)
	if err != nil {
		return err
	}
	var loaded loadedPackage
	if preloaded != nil {
		loaded = *preloaded
	} else {
		writeProgress(app, opts.format, "Resolving and validating Agent Plugin...")
		loaded, err = app.loadPackageFor(cmd.Context(), source, withDetectedClients(app.addResolutionRequest(source, selection), detected))
		if err != nil {
			return err
		}
		if loaded.cleanup != nil {
			defer func() { _ = loaded.cleanup() }()
		}
	}
	return runAddManyLoaded(cmd.Context(), cmd, app, opts, loaded, selection, activationComplete, authComplete, detectedClientValues(detected), true)
}

func runAdd(ctx context.Context, cmd *cobra.Command, app App, opts *options, source string, activationComplete, authComplete bool) error {
	return runAddWithClients(ctx, cmd, app, opts, source, activationComplete, authComplete, nil)
}

func runAddWithClients(ctx context.Context, cmd *cobra.Command, app App, opts *options, source string, activationComplete, authComplete bool, clients []domain.DetectedClient) error {
	targets, err := parseTargetOption(opts.target)
	if err != nil {
		return err
	}
	_, detected, err := preflightAddTargets(ctx, app, opts, source, targets, clients)
	if err != nil {
		return err
	}
	writeProgress(app, opts.format, "Resolving and validating Agent Plugin...")
	loaded, err := app.loadPackageFor(ctx, source, withDetectedClients(app.addResolutionRequest(source, targets), detected))
	if err != nil {
		return err
	}
	if loaded.cleanup != nil {
		defer loaded.cleanup()
	}
	return runAddLoaded(ctx, cmd, app, opts, loaded, activationComplete, authComplete, detectedClientValues(detected), false)
}

func runAddLoaded(ctx context.Context, cmd *cobra.Command, app App, opts *options, loaded loadedPackage, activationComplete, authComplete bool, clients []domain.DetectedClient, needsInstallConfirmation bool) error {
	targets, err := parseTargetOption(opts.target)
	if err != nil {
		return err
	}
	applyLoadedGuidedIntents(opts, loaded, targets)
	if opts.chatGPTAppID != "" && !loaded.chatGPTPreparation {
		return fmt.Errorf("--chatgpt-app-id requires the signed Context7 Directory source and --target chatgpt")
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
		Hints: loaded.hints, BackendExecutable: backendExecutable(selected, detectedMap),
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
	if opts.format == "human" {
		if err := renderHumanPlan(cmd.OutOrStdout(), loaded.envelope, planned); err != nil {
			return err
		}
	}
	freshInstall := planned.Activation.Activation == ""
	if !freshInstall && opts.format == "human" && app.Terminal && !activationComplete && !authComplete {
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
		if opts.format == "json" {
			return renderAddResultWithSecurity(cmd.OutOrStdout(), opts.format, loaded.envelope, loaded.security, planned, false)
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No changes made.")
		return nil
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
	if err == nil && freshInstall && opts.format == "human" && app.Terminal {
		return resumeInteractiveLifecycle(ctx, cmd, service, input, loaded.envelope, result)
	}
	return err
}

func confirmPlannedAdd(ctx context.Context, cmd *cobra.Command, app App, opts *options, loaded loadedPackage, planned usecase.AddResult, freshInstall, needsInstallConfirmation bool) (bool, error) {
	confirmed := mutationConfirmed(app, opts) && !needsInstallConfirmation
	if needsInstallConfirmation {
		return confirmInstall(ctx, cmd, app, loaded, []usecase.AddResult{planned})
	}
	if confirmed || opts.format != "human" || !app.Terminal {
		return confirmed, nil
	}
	question := "Apply this plan? [y/N]"
	if !freshInstall {
		question = "Apply these explicit lifecycle attestations? [y/N]"
	}
	return promptYesNo(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), question)
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
		request.SkippedLabels = append(request.SkippedLabels, prompt.SafeText(string(c.Client)+": "+c.Reason))
	}
	if len(detected) > 0 {
		if err := prompt.ValidateRequest(request); err != nil {
			return nil, nil, err
		}
	}
	personalPreparationChoice := len(detected) == 1 && requiresPersonalMapping(detected[0].ClientID) && strings.Contains(detected[0].DisplayName, "prepare personal marketplace")
	if len(detected) <= 1 && !personalPreparationChoice {
		for _, label := range request.SkippedLabels {
			if _, err := fmt.Fprintln(reviewWriter(cmd, app), "Skipped (not installed in this attempt): "+label); err != nil {
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

func resumeInteractiveLifecycle(
	ctx context.Context,
	cmd *cobra.Command,
	service usecase.Service,
	input usecase.AddInput,
	envelope domain.PackageEnvelope,
	current usecase.AddResult,
) error {
	if current.Plan.InstallIntent == domain.InstallIntentPrepare {
		return nil
	}
	var err error
	if current.Activation.Activation != domain.ActivationActive || current.Activation.Verification != domain.VerificationInstalled {
		current, err = attestActivationComplete(ctx, cmd, service, input, envelope, current)
		if err != nil || current.Activation.Activation != domain.ActivationActive || current.Activation.Verification != domain.VerificationInstalled {
			return err
		}
	}
	if current.Activation.Authentication == domain.AuthenticationPending || current.Activation.Authentication == domain.AuthenticationNotChecked {
		current, err = attestAuthenticationComplete(ctx, cmd, service, input, envelope, current)
		if err != nil || current.Activation.Authentication == domain.AuthenticationPending || current.Activation.Authentication == domain.AuthenticationNotChecked {
			return err
		}
	}
	return renderAddResult(cmd.OutOrStdout(), "human", envelope, current, false)
}

func attestActivationComplete(ctx context.Context, cmd *cobra.Command, service usecase.Service, input usecase.AddInput, envelope domain.PackageEnvelope, current usecase.AddResult) (usecase.AddResult, error) {
	complete, err := promptYesNo(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), "Have you completed activation and verified the plugin is enabled in the client? [y/N]")
	if err != nil || !complete {
		if err == nil {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Activation remains unconfirmed. Rerun add when it is complete.")
		}
		return current, err
	}
	return resumeAddPhase(ctx, cmd, service, input, envelope, current, true, false)
}

func attestAuthenticationComplete(ctx context.Context, cmd *cobra.Command, service usecase.Service, input usecase.AddInput, envelope domain.PackageEnvelope, current usecase.AddResult) (usecase.AddResult, error) {
	complete, err := promptYesNo(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), "Have you completed required authentication, or reviewed the package and confirmed none is required? [y/N]")
	if err != nil || !complete {
		if err == nil {
			if current.Activation.ActivationAttested {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Activation is user-attested. Authentication remains unconfirmed; rerun add after completing or reviewing it.")
			} else {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Authentication remains unconfirmed. Rerun add after completing or reviewing it.")
			}
		}
		return current, err
	}
	return resumeAddPhase(ctx, cmd, service, input, envelope, current, false, true)
}

func resumeAddPhase(ctx context.Context, cmd *cobra.Command, service usecase.Service, input usecase.AddInput, envelope domain.PackageEnvelope, current usecase.AddResult, activationComplete, authComplete bool) (usecase.AddResult, error) {
	resume := input
	resume.Confirmed = true
	resume.InstallationID = current.InstallationID
	resume.ActivationComplete = activationComplete
	resume.AuthComplete = authComplete
	result, err := service.Add(ctx, resume)
	if err != nil {
		if renderErr := renderAddResultError(cmd.OutOrStdout(), "human", envelope, result, false, err); renderErr != nil {
			return result, renderErr
		}
	}
	return result, err
}
