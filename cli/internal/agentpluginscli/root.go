package agentpluginscli

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
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
	app.Output = terminaltheme.Wrap(app.output(), &opts.color, &opts.format)
	app.ErrorOutput = terminaltheme.Wrap(app.errorOutput(), &opts.color, &opts.format)
	root.SetOut(app.output())
	root.SetErr(app.errorOutput())
	flags := root.PersistentFlags()
	flags.Var(targetFlagValue{target: &opts.target, input: &opts.targetInput}, "target", "target client(s), comma-separated: "+supportedTargetHelp())
	flags.StringVar(&opts.scope, "scope", "user", "installation scope: user or project")
	flags.StringVar(&opts.projectRoot, "project-root", "", "existing project directory (project scope only; default: current directory)")
	flags.BoolVar(&opts.dryRun, "dry-run", false, "show the exact plan without changes")
	flags.StringVar(&opts.format, "format", "human", "output format: human or json")
	flags.BoolVar(&opts.plain, "plain", false, "use accessible line prompts (independent of color)")
	flags.Var(&opts.color, "color", "human output color: auto, never, always")
	flags.Var(terminaltheme.NoColorFlag{Policy: &opts.color}, "no-color", "alias for --color=never")
	flags.Lookup("no-color").NoOptDefVal = "true"
	flags.BoolVar(&opts.acceptSecurityRisk, "accept-security-risk", false, "continue despite blocking automated security findings")
	flags.BoolVar(&opts.securityDetails, "security-details", false, "show every automated security finding in human output")

	root.AddCommand(newAddCommand(app, opts))
	root.AddCommand(newSkillsCommand(app, opts))
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
	root.AddCommand(newDepsCommand(app, opts))
	root.AddCommand(newInstallCommand(app, opts))
	root.AddCommand(newDoctorCommand(app, opts))
	root.AddCommand(newRunClientCommand(app, opts))
	root.AddCommand(newVersionCommand(app, opts))
	root.AddCommand(newManagementCommand(app))
	return root
}

func validateCommonOptions(opts *options) error {
	if opts.format != "human" && opts.format != "json" {
		return CLIUsageError{Code: "INVALID_FORMAT", Message: "--format must be human or json"}
	}
	if opts.scope != "user" && opts.scope != "project" {
		return CLIUsageError{Code: "INVALID_SCOPE", Message: "--scope must be user or project"}
	}
	if opts.scope != "project" && opts.projectRoot != "" {
		return CLIUsageError{Code: "INVALID_SCOPE", Message: "--project-root requires --scope project"}
	}
	if _, err := parseTargetOption(opts.target); err != nil {
		return err
	}
	// The public Desktop name is rewritten to canonical "codex" at the flag
	// boundary. Check the original flag input instead: otherwise --target
	// chatgpt --scope project silently installs into a Codex project.
	if opts.scope == "project" {
		for _, target := range strings.Split(opts.targetInput, ",") {
			switch strings.ToLower(strings.TrimSpace(target)) {
			case "chatgpt", "chatgpt-desktop":
				return CLIUsageError{
					Code:    "INVALID_SCOPE",
					Message: "chatgpt (Desktop) supports only --scope user; use --target codex for project installation",
				}
			}
		}
	}
	return nil
}
