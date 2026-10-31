package agentpluginscli

import "github.com/spf13/cobra"

func newDependencyInitCommand(app App, opts *options) *cobra.Command {
	return &cobra.Command{Use: "init", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		app, err := app.forOptions(opts)
		if err != nil {
			return err
		}
		if err := validateDepsOptions(opts); err != nil {
			return err
		}
		return initializeDependencies(cmd, app, opts)
	}}
}

func newDependencyLockCommand(app App, opts *options, update bool) *cobra.Command {
	command := &cobra.Command{Use: "lock", Args: cobra.NoArgs}
	if update {
		command.Use = "update <alias>"
		command.Args = cobra.ExactArgs(1)
	}
	command.RunE = func(cmd *cobra.Command, args []string) error {
		app, err := app.forOptions(opts)
		if err != nil {
			return err
		}
		if err := validateDepsOptions(opts); err != nil {
			return err
		}
		alias := ""
		if update {
			alias = args[0]
		}
		result, err := runDependencyLock(cmd.Context(), app, opts, alias, true)
		if err != nil {
			return err
		}
		return renderDependencyLock(cmd, opts, result)
	}
	return command
}

func newDependencyStatusCommand(app App, opts *options) *cobra.Command {
	return &cobra.Command{Use: "status", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		app, err := app.forOptions(opts)
		if err != nil {
			return err
		}
		if err := validateDepsOptions(opts); err != nil {
			return err
		}
		return runDependencyStatus(cmd, app, opts)
	}}
}

func newDependencySyncCommand(app App, opts *options, locked, yes *bool) *cobra.Command {
	command := &cobra.Command{Use: "sync", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		app, err := app.forOptions(opts)
		if err != nil {
			return err
		}
		if err := validateDepsOptions(opts); err != nil {
			return err
		}
		return runDependencySync(cmd, app, opts, *locked, *yes)
	}}
	command.Flags().BoolVar(&opts.externalUninstalled, "external-uninstalled", false, "attest removed targets were uninstalled manually or never imported")
	command.Flags().BoolVar(locked, "locked", false, "require an exact existing lock and never resolve refs")
	command.Flags().BoolVar(yes, "yes", false, "confirm the complete preflighted sync without an interactive prompt")
	return command
}
