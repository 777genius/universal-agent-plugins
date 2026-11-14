package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	skillsapp "github.com/777genius/plugin-kit-ai/cli/internal/skills/app"
	"github.com/spf13/cobra"
)

type skillsRunner interface {
	Init(app.SkillsInitOptions) (string, error)
	Validate(app.SkillsValidateOptions) (skillsapp.ValidationReport, error)
	Generate(app.SkillsGenerateOptions) ([]string, error)
	InstallExternal(context.Context, app.ExternalSkillsInstallOptions) error
	ListExternal(context.Context, app.ExternalSkillsListOptions) error
	UpdateExternal(context.Context, app.ExternalSkillsUpdateOptions) error
	RemoveExternal(context.Context, app.ExternalSkillsRemoveOptions) error
}

var skillsService app.SkillsService
var skillsCmd = newSkillsCmd(skillsService)

func newSkillsCmd(runner skillsRunner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Experimental skill authoring tools",
		Long: strings.Join([]string{
			"Experimental SKILL.md-native authoring, validation, and generating tools for Claude and Codex.",
			"",
			"External install/list/update/remove commands are npm-backed pass-through wrappers around skills@1.5.5.",
		}, "\n"),
	}
	cmd.AddCommand(newSkillsInitCmd(runner))
	cmd.AddCommand(newSkillsValidateCmd(runner))
	cmd.AddCommand(newSkillsGenerateCmd(runner))
	cmd.AddCommand(newSkillsExternalInstallCmd(runner))
	cmd.AddCommand(newSkillsExternalListCmd(runner))
	cmd.AddCommand(newSkillsExternalUpdateCmd(runner))
	cmd.AddCommand(newSkillsExternalRemoveCmd(runner))
	return cmd
}

type skillsInitFlagState struct {
	output      string
	description string
	template    string
	command     string
	force       bool
}

func newSkillsInitCmd(runner skillsRunner) *cobra.Command {
	flags := skillsInitFlagState{
		output:   ".",
		template: "go-command",
		command:  "replace-me",
	}
	cmd := &cobra.Command{
		Use:   "init [skill-name]",
		Short: "Create a canonical SKILL.md skill package",
		Example: strings.Join([]string{
			"  plugin-kit-ai skills init lint-repo --template go-command",
			"  plugin-kit-ai skills init format-changed --template cli-wrapper --command \"ruff format .\"",
			"  plugin-kit-ai skills init review-checklist --template docs-only",
		}, "\n"),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := strings.TrimSpace(flags.output)
			if root == "" {
				root = "."
			}
			out, err := runner.Init(app.SkillsInitOptions{
				Name:        strings.TrimSpace(args[0]),
				Description: strings.TrimSpace(flags.description),
				Template:    strings.TrimSpace(flags.template),
				OutputDir:   root,
				Command:     strings.TrimSpace(flags.command),
				Force:       flags.force,
			})
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Created skill %q at %s\nNext: edit skills/%s/SKILL.md, then run `plugin-kit-ai skills validate %s` and `plugin-kit-ai skills generate %s --target all`.\n", args[0], out, args[0], root, root)
			return nil
		},
	}
	cmd.Flags().StringVarP(&flags.output, "output", "o", ".", "project root containing skills/")
	cmd.Flags().StringVar(&flags.description, "description", "", "skill description")
	cmd.Flags().StringVar(&flags.template, "template", "go-command", `template ("go-command", "cli-wrapper", "docs-only")`)
	cmd.Flags().StringVar(&flags.command, "command", "replace-me", "default command for cli-wrapper template")
	cmd.Flags().BoolVarP(&flags.force, "force", "f", false, "overwrite existing authored files")
	return cmd
}

func newSkillsValidateCmd(runner skillsRunner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [path]",
		Short: "Validate canonical SKILL.md skills in a project",
		Example: strings.Join([]string{
			"  plugin-kit-ai skills validate .",
			"  plugin-kit-ai skills validate ./examples/skills/go-command-lint",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			report, err := runner.Validate(app.SkillsValidateOptions{Root: root})
			if err != nil {
				return err
			}
			if len(report.Failures) > 0 {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Skill validation found %d problem(s):\n", len(report.Failures))
				for _, failure := range report.Failures {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "- %s: %s\n", failure.Path, failure.Message)
				}
				return fmt.Errorf("skill validation failed")
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Validated %d skill(s) in %s\n", len(report.Skills), root)
			return nil
		},
	}
	return cmd
}

type skillsGenerateFlagState struct {
	target string
}

func newSkillsGenerateCmd(runner skillsRunner) *cobra.Command {
	flags := skillsGenerateFlagState{target: "all"}
	cmd := &cobra.Command{
		Use:   "generate [path]",
		Short: "Generate Claude/Codex artifacts from canonical SKILL.md files",
		Example: strings.Join([]string{
			"  plugin-kit-ai skills generate . --target all",
			"  plugin-kit-ai skills generate ./examples/skills/cli-wrapper-formatter --target codex",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			artifacts, err := runner.Generate(app.SkillsGenerateOptions{
				Root:   root,
				Target: flags.target,
			})
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Generated %d artifact(s) in %s\n", len(artifacts), root)
			return nil
		},
	}
	cmd.Flags().StringVar(&flags.target, "target", "all", `generate target ("all", "claude", "codex")`)
	return cmd
}

type externalSkillsInstallFlagState struct {
	version   string
	global    bool
	agents    []string
	skills    []string
	list      bool
	yes       bool
	copy      bool
	all       bool
	fullDepth bool
}

func newSkillsExternalInstallCmd(runner skillsRunner) *cobra.Command {
	flags := externalSkillsInstallFlagState{version: app.DefaultExternalSkillsCLIVersion}
	cmd := &cobra.Command{
		Use:     "install <source>",
		Aliases: []string{"add"},
		Short:   "Install external Agent Skills through the npm skills CLI",
		Long:    "Install external Agent Skills by forwarding to `npx -y skills@<version> add`.",
		Example: strings.Join([]string{
			"  plugin-kit-ai skills install flutter/skills --global --all",
			"  plugin-kit-ai skills install dart-lang/skills --skill '*' --agent codex --agent claude-code --global --yes",
			"  plugin-kit-ai skills add flutter/skills --list",
		}, "\n"),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runner.InstallExternal(cmd.Context(), app.ExternalSkillsInstallOptions{
				Source:    args[0],
				Version:   flags.version,
				Global:    flags.global,
				Agents:    flags.agents,
				Skills:    flags.skills,
				List:      flags.list,
				Yes:       flags.yes,
				Copy:      flags.copy,
				All:       flags.all,
				FullDepth: flags.fullDepth,
				Stdout:    cmd.OutOrStdout(),
				Stderr:    cmd.ErrOrStderr(),
			})
		},
	}
	addExternalSkillsVersionFlag(cmd, &flags.version)
	cmd.Flags().BoolVarP(&flags.global, "global", "g", false, "install skill globally (user-level) instead of project-level")
	cmd.Flags().StringSliceVarP(&flags.agents, "agent", "a", nil, "agent(s) to install to, use '*' for all agents")
	cmd.Flags().StringSliceVarP(&flags.skills, "skill", "s", nil, "skill name(s) to install, use '*' for all skills")
	cmd.Flags().BoolVarP(&flags.list, "list", "l", false, "list available skills in the repository without installing")
	cmd.Flags().BoolVarP(&flags.yes, "yes", "y", false, "skip confirmation prompts")
	cmd.Flags().BoolVar(&flags.copy, "copy", false, "copy files instead of symlinking to agent directories")
	cmd.Flags().BoolVar(&flags.all, "all", false, "shorthand for --skill '*' --agent '*' --yes in the upstream skills CLI")
	cmd.Flags().BoolVar(&flags.fullDepth, "full-depth", false, "search all subdirectories even when a root SKILL.md exists")
	return cmd
}

type externalSkillsListFlagState struct {
	version string
	global  bool
	agents  []string
	json    bool
}

func newSkillsExternalListCmd(runner skillsRunner) *cobra.Command {
	flags := externalSkillsListFlagState{version: app.DefaultExternalSkillsCLIVersion}
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List external Agent Skills through the npm skills CLI",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runner.ListExternal(cmd.Context(), app.ExternalSkillsListOptions{
				Version: flags.version,
				Global:  flags.global,
				Agents:  flags.agents,
				JSON:    flags.json,
				Stdout:  cmd.OutOrStdout(),
				Stderr:  cmd.ErrOrStderr(),
			})
		},
	}
	addExternalSkillsVersionFlag(cmd, &flags.version)
	cmd.Flags().BoolVarP(&flags.global, "global", "g", false, "list global skills instead of project skills")
	cmd.Flags().StringSliceVarP(&flags.agents, "agent", "a", nil, "filter by agent(s)")
	cmd.Flags().BoolVar(&flags.json, "json", false, "output JSON from the upstream skills CLI")
	return cmd
}

type externalSkillsUpdateFlagState struct {
	version string
	global  bool
	project bool
	yes     bool
}

func newSkillsExternalUpdateCmd(runner skillsRunner) *cobra.Command {
	flags := externalSkillsUpdateFlagState{version: app.DefaultExternalSkillsCLIVersion}
	cmd := &cobra.Command{
		Use:     "update [skills...]",
		Aliases: []string{"upgrade"},
		Short:   "Update external Agent Skills through the npm skills CLI",
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runner.UpdateExternal(cmd.Context(), app.ExternalSkillsUpdateOptions{
				Version: flags.version,
				Skills:  args,
				Global:  flags.global,
				Project: flags.project,
				Yes:     flags.yes,
				Stdout:  cmd.OutOrStdout(),
				Stderr:  cmd.ErrOrStderr(),
			})
		},
	}
	addExternalSkillsVersionFlag(cmd, &flags.version)
	cmd.Flags().BoolVarP(&flags.global, "global", "g", false, "update global skills only")
	cmd.Flags().BoolVarP(&flags.project, "project", "p", false, "update project skills only")
	cmd.Flags().BoolVarP(&flags.yes, "yes", "y", false, "skip scope prompt")
	return cmd
}

type externalSkillsRemoveFlagState struct {
	version string
	global  bool
	agents  []string
	skills  []string
	yes     bool
	all     bool
}

func newSkillsExternalRemoveCmd(runner skillsRunner) *cobra.Command {
	flags := externalSkillsRemoveFlagState{version: app.DefaultExternalSkillsCLIVersion}
	cmd := &cobra.Command{
		Use:     "remove [skills...]",
		Aliases: []string{"rm"},
		Short:   "Remove external Agent Skills through the npm skills CLI",
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runner.RemoveExternal(cmd.Context(), app.ExternalSkillsRemoveOptions{
				Version: flags.version,
				Skills:  args,
				Global:  flags.global,
				Agents:  flags.agents,
				Filter:  flags.skills,
				Yes:     flags.yes,
				All:     flags.all,
				Stdout:  cmd.OutOrStdout(),
				Stderr:  cmd.ErrOrStderr(),
			})
		},
	}
	addExternalSkillsVersionFlag(cmd, &flags.version)
	cmd.Flags().BoolVarP(&flags.global, "global", "g", false, "remove from global scope")
	cmd.Flags().StringSliceVarP(&flags.agents, "agent", "a", nil, "remove from agent(s), use '*' for all agents")
	cmd.Flags().StringSliceVarP(&flags.skills, "skill", "s", nil, "skill name(s) to remove, use '*' for all skills")
	cmd.Flags().BoolVarP(&flags.yes, "yes", "y", false, "skip confirmation prompts")
	cmd.Flags().BoolVar(&flags.all, "all", false, "shorthand for --skill '*' --agent '*' --yes in the upstream skills CLI")
	return cmd
}

func addExternalSkillsVersionFlag(cmd *cobra.Command, target *string) {
	cmd.Flags().StringVar(target, "skills-cli-version", app.DefaultExternalSkillsCLIVersion, "npm skills CLI version to run")
}

func init() {
	rootCmd.AddCommand(skillsCmd)
}
