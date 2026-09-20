package main

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/spf13/cobra"
)

type initCommandRunner interface {
	Run(app.InitOptions) (string, error)
}

type initFlagState struct {
	template              string
	platform              string
	runtime               string
	typescript            bool
	runtimePackage        bool
	runtimePackageVersion string
	output                string
	force                 bool
	extras                bool
	claudeExtendedHooks   bool
}

var initRunner app.InitRunner

var initCmd = newInitCmd(initRunner)

const initLongDescription = `Creates a package-standard plugin-kit-ai project scaffold.

Start with the job you want to solve:

Connect an online service:
  Use --template online-service for hosted integrations like Notion, Stripe, Cloudflare, or Vercel.
  This starter creates an MCP-first repo with shared authored source under plugin/ and no launcher code.

Connect a local tool:
  Use --template local-tool for local MCP-backed tools like Docker Hub, Chrome DevTools, or HubSpot Developer.
  This starter creates an MCP-first repo with local command wiring under plugin/ and no launcher code.

Build custom plugin logic - Advanced:
  Use --template custom-logic when you need launcher-backed code, hooks, or your own runtime behavior.
  This path is more powerful and more engineering-heavy than the first two starters.
  Plain init remains as a legacy compatibility path for the older codex-runtime plus Go starter.

Already have native config:
  Use plugin-kit-ai import to bring current Claude/Codex/Gemini/OpenCode/Cursor native files into the package-standard authored layout.
  init is for creating a new package-standard project, not for preserving native files as the authored source of truth.

Public flags:
  --template   Recommended start: "online-service", "local-tool", or "custom-logic".
  --platform   Advanced override: "codex-runtime" (default), "codex-package", "claude", "gemini", "opencode", "cursor", or "cursor-workspace".
  --runtime    Supported: "go" (default), "python", "node", "shell" for launcher-based targets only.
  --typescript Generate a TypeScript scaffold on top of the node runtime lane (requires --runtime node).
  --runtime-package
               For --runtime python or --runtime node, import the shared plugin-kit-ai-runtime package instead of vendoring the helper file into plugin/.
  --runtime-package-version
               Pin the generated plugin-kit-ai-runtime dependency version. Required on development builds; released CLIs default to their own stable tag.
  -o, --output Target directory (default: ./<project-name>).
  -f, --force  Allow writing into a non-empty directory and overwrite generated files.
  --extras     Also emit optional release helpers such as Makefile, .goreleaser.yml, portable skills/, and stable Python/Node bundle-release workflow scaffolding where supported.
  --claude-extended-hooks
               For --platform claude, scaffold the full runtime-supported hook set instead of the stable default subset.`

func newInitCmd(runner initCommandRunner) *cobra.Command {
	flags := initFlagState{}
	cmd := &cobra.Command{
		Use:   "init [project-name]",
		Short: "Create a plugin-kit-ai package scaffold",
		Long:  initLongDescription,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd, runner, flags, args)
		},
	}
	cmd.Flags().StringVar(&flags.template, "template", "", `recommended start ("online-service", "local-tool", or "custom-logic")`)
	cmd.Flags().StringVar(&flags.platform, "platform", "codex-runtime", `target lane ("codex-runtime", "codex-package", "claude", "gemini", "opencode", "cursor", or "cursor-workspace")`)
	cmd.Flags().StringVar(&flags.runtime, "runtime", "go", `runtime ("go", "python", "node", or "shell")`)
	cmd.Flags().BoolVar(&flags.typescript, "typescript", false, "generate a TypeScript scaffold on top of the node runtime lane")
	cmd.Flags().BoolVar(&flags.runtimePackage, "runtime-package", false, "for --runtime python or --runtime node, import the shared plugin-kit-ai-runtime package instead of vendoring the helper file")
	cmd.Flags().StringVar(&flags.runtimePackageVersion, "runtime-package-version", "", "pin the generated plugin-kit-ai-runtime dependency version")
	cmd.Flags().StringVarP(&flags.output, "output", "o", "", "output directory (default: ./<project-name>)")
	cmd.Flags().BoolVarP(&flags.force, "force", "f", false, "overwrite generated files; allow non-empty output directory")
	cmd.Flags().BoolVar(&flags.extras, "extras", false, "include optional scaffold files (runtime-dependent extras plus skills and commands)")
	cmd.Flags().BoolVar(&flags.claudeExtendedHooks, "claude-extended-hooks", false, "for --platform claude, scaffold the full runtime-supported hook set instead of the stable default subset")
	return cmd
}
