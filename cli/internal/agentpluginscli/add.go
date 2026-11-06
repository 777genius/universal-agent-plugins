package agentpluginscli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func newAddCommand(app App, opts *options) *cobra.Command {
	var activationComplete, authComplete bool
	command := &cobra.Command{
		Use:   "add <name-or-source>",
		Short: "Plan and install one Agent Plugins 1.0 package for one or more clients",
		Args:  addCommandArgs(opts),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, scopeErr := app.forOptions(opts)
			if scopeErr != nil {
				return scopeErr
			}
			return executeAddCommand(cmd, app, opts, args[0], activationComplete, authComplete)
		},
	}
	command.Flags().BoolVar(&opts.yes, "yes", false, "accept the preflighted installation plan without a confirmation prompt")
	command.Flags().BoolVar(&opts.noSave, "no-save", false, "install without saving portable dependency intent")
	command.Flags().StringVar(&opts.alias, "alias", "", "saved dependency alias")
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
	if opts.chatGPTAppID != "" {
		if err := domain.ValidateChatGPTAppID(opts.chatGPTAppID); err != nil {
			return err
		}
	}
	if handled, err := routeSavedAdd(cmd, app, opts, source, activationComplete, authComplete); handled {
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
		return CLIUsageError{Code: "TARGET_REQUIRED", Message: "automated installation requires --target"}
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
	writeProgress(app, opts.format, "Checking selected agents and preparing the install plan...")
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
	_, detected, err := preflightProjectLifecycleTargets(cmd.Context(), app, opts, selection, detectedClients, false)
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
