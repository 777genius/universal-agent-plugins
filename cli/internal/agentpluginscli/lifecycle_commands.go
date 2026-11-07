package agentpluginscli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newUpdateCommand(app App, opts *options) *cobra.Command {
	var all bool
	command := &cobra.Command{
		Use:   "update <name-or-installation-id> | update --all",
		Short: "Safely update a tracked Agent Plugin across selected or installed clients",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, scopeErr := app.forOptions(opts)
			if scopeErr != nil {
				return scopeErr
			}
			if err := validateCommonOptions(opts); err != nil {
				return err
			}
			operationApp := app
			operationApp.Lifecycle.PrepareHostsForPreview = !opts.dryRun
			if all {
				if len(args) > 0 {
					return fmt.Errorf("choose either one installation or --all")
				}
				if strings.TrimSpace(opts.target) != "" {
					return fmt.Errorf("--target cannot be combined with --all; every installation keeps its recorded targets")
				}
				return runUpdateAll(cmd.Context(), cmd, operationApp, opts)
			}
			if len(args) != 1 {
				return fmt.Errorf("provide one installation or use --all")
			}
			if handled, err := runSavedLifecycle(cmd, operationApp, opts, args[0], "update"); handled || err != nil {
				return err
			}
			if strings.TrimSpace(opts.target) == "" {
				targets, err := defaultInstalledBindingTargets(cmd.Context(), app, args[0], opts.scope, false)
				if err != nil {
					return err
				}
				opts.target = joinTargets(targets)
				defer func() { opts.target = "" }()
			}
			targets, err := parseTargetOption(opts.target)
			if err != nil {
				return err
			}
			return runUpdateMany(cmd.Context(), cmd, operationApp, opts, args[0], targets)
		},
	}
	command.Flags().BoolVar(&all, "all", false, "update saved dependencies and eligible tracked installations after one complete preflight")
	command.Flags().BoolVar(&opts.yes, "yes", false, "accept the complete preflighted update plan")
	return command
}

func newRepairCommand(app App, opts *options) *cobra.Command {
	command := &cobra.Command{
		Use: "repair <name-or-installation-id>", Short: "Explicitly restore a missing or modified managed package",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, scopeErr := app.forOptions(opts)
			if scopeErr != nil {
				return scopeErr
			}
			if err := validateCommonOptions(opts); err != nil {
				return err
			}
			operationApp := app
			operationApp.Lifecycle.PrepareHostsForPreview = !opts.dryRun
			if handled, err := runSavedLifecycle(cmd, operationApp, opts, args[0], "repair"); handled || err != nil {
				return err
			}
			if strings.TrimSpace(opts.target) == "" {
				targets, err := defaultInstalledBindingTargets(cmd.Context(), app, args[0], opts.scope, true)
				if err != nil {
					return err
				}
				opts.target = joinTargets(targets)
				defer func() { opts.target = "" }()
			}
			stdin := bufio.NewReader(cmd.InOrStdin())
			targets, err := parseTargetOption(opts.target)
			if err != nil {
				return err
			}
			_ = stdin
			if handled, err := repairOwnedDependencies(cmd, operationApp, opts, args[0], targets); handled || err != nil {
				return err
			}
			return runRepairMany(cmd.Context(), cmd, operationApp, opts, args[0], targets)
		},
	}
	command.Flags().BoolVar(&opts.yes, "yes", false, "accept the preflighted repair plan")
	return command
}

func newRemoveCommand(app App, opts *options) *cobra.Command {
	command := &cobra.Command{
		Use:   "remove <name-or-installation-id>",
		Short: "Safely remove selected targets while retaining plugin data by default",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, scopeErr := app.forOptions(opts)
			if scopeErr != nil {
				return scopeErr
			}
			if err := validateCommonOptions(opts); err != nil {
				return err
			}
			if opts.yes && strings.TrimSpace(opts.target) == "" && !opts.purgeData {
				return CLIUsageError{Code: "TARGET_REQUIRED", Message: "remove --yes requires explicit --target"}
			}
			if handled, err := runSavedLifecycle(cmd, app, opts, args[0], "remove"); handled || err != nil {
				return err
			}
			// JSON calls must reject missing selectors before
			// loading state, presenting choices, or touching an open input stream.
			// Purging retained data is installation-wide and needs no target.
			if opts.format == "json" && !opts.purgeData {
				if err := requireNonInteractiveMutation(app, opts, "removal"); err != nil {
					return err
				}
			}
			if strings.TrimSpace(opts.target) == "" && app.Terminal && !opts.purgeData {
				selection, err := promptBoundTargets(cmd, app, args[0], opts.scope)
				if err != nil {
					return err
				}
				opts.target = selection
				defer func() { opts.target = "" }()
			}
			return dispatchSelectedRemoval(cmd, app, opts, args[0])
		},
	}
	command.Flags().BoolVar(&opts.externalUninstalled, "external-uninstalled", false, "confirm the selected client plugin was uninstalled manually or was never activated/imported")
	command.Flags().BoolVar(&opts.purgeData, "purge-data", false, "permanently delete ownership-verified plugin data after removal")
	command.Flags().BoolVar(&opts.yes, "yes", false, "accept the preflighted removal plan (never implies --purge-data)")
	return command
}

// dispatchSelectedRemoval preserves single-target and legacy-all behavior
// while keeping the command's input/confirmation policy separate from routing.
func dispatchSelectedRemoval(cmd *cobra.Command, app App, opts *options, selector string) error {
	if opts.purgeData {
		return runRemove(cmd.Context(), cmd, app, opts, selector)
	}
	targets, err := parseTargetOption(opts.target)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return runRemove(cmd.Context(), cmd, app, opts, selector)
	}
	if len(targets) == 1 && targets[0] == "legacy-all" {
		return runRemove(cmd.Context(), cmd, app, opts, selector)
	}
	return runRemoveMany(cmd.Context(), cmd, app, opts, selector, targets)
}
