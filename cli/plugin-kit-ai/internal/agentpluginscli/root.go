package agentpluginscli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewRoot(app App) *cobra.Command {
	opts := &options{}
	root := &cobra.Command{
		Use:           "agentplugins",
		Short:         "Install and manage Agent Plugins 1.0 packages across AI clients",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetIn(app.input())
	root.SetOut(app.output())
	root.SetErr(app.errorOutput())
	flags := root.PersistentFlags()
	flags.StringVar(&opts.target, "target", "", "target client(s), comma-separated: "+supportedTargetHelp())
	flags.StringVar(&opts.scope, "scope", "user", "installation scope (user only in this release)")
	flags.BoolVar(&opts.dryRun, "dry-run", false, "show the exact plan without changes")
	flags.StringVar(&opts.format, "format", "human", "output format: human or json")
	flags.BoolVar(&opts.noColor, "no-color", false, "disable color output")
	flags.BoolVar(&opts.acceptSecurityRisk, "accept-security-risk", false, "continue despite blocking automated security findings")
	flags.BoolVar(&opts.securityDetails, "security-details", false, "show every automated security finding in human output")

	root.AddCommand(newAddCommand(app, opts))
	root.AddCommand(newUpdateCommand(app, opts))
	root.AddCommand(newRepairCommand(app, opts))
	root.AddCommand(newRemoveCommand(app, opts))
	root.AddCommand(newSwitchCommand(app, opts))
	root.AddCommand(newMigrateFormatCommand(app, opts))
	root.AddCommand(newMigrateStateCommand(app, opts))
	root.AddCommand(newRebindCommand(app, opts))
	root.AddCommand(newListCommand(app, opts))
	root.AddCommand(newInfoCommand(app, opts))
	root.AddCommand(newSearchCommand(app, opts))
	root.AddCommand(newValidateCommand(app, opts))
	root.AddCommand(newOutdatedCommand(app, opts))
	root.AddCommand(newDoctorCommand(app, opts))
	root.AddCommand(newVersionCommand(app, opts))
	return root
}

func validateCommonOptions(opts *options) error {
	if opts.format != "human" && opts.format != "json" {
		return fmt.Errorf("--format must be human or json")
	}
	if opts.scope != "user" && opts.scope != "project" {
		return fmt.Errorf("--scope must be user or project")
	}
	if opts.scope == "project" {
		return fmt.Errorf("--scope project is not supported by the current client adapters; the public CLI supports user scope only")
	}
	if _, err := parseTargetOption(opts.target); err != nil {
		return err
	}
	return nil
}
