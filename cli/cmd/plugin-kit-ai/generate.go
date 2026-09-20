package main

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/spf13/cobra"
)

var pluginService app.PluginService

var (
	generateTarget string
	generateCheck  bool
)

var generateCmd = &cobra.Command{
	Use:   "generate [path]",
	Short: "Compile native target artifacts from the package graph",
	Long: `Compile native target artifacts from the package graph discovered via canonical plugin/plugin.yaml plus the standard authored directories.

Claude and Codex runtime/package lanes generate their managed native artifacts from the package graph.
Gemini generation always produces the native extension package artifacts and may also carry the optional Go runtime lane when the authored project includes it; that lane now exposes a production-ready 9-hook runtime surface, but it still does not imply blanket runtime parity for future hooks beyond the promoted contract.
OpenCode generation is workspace-config-only: it produces opencode.json plus mirrored skills, commands, agents, themes, local plugin code, and plugin-local package metadata without introducing a launcher/runtime contract.
Cursor generation now defaults to the packaged plugin lane: it produces .cursor-plugin/plugin.json plus managed skills/ and optional .mcp.json from canonical portable inputs. Use target cursor-workspace only when you intentionally need repo-local .cursor config generation and mirrored .cursor/rules/**.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root := "."
		if len(args) == 1 {
			root = args[0]
		}
		out, err := pluginService.Generate(app.PluginGenerateOptions{
			Root:   root,
			Target: generateTarget,
			Check:  generateCheck,
		})
		if err != nil {
			return err
		}
		if generateCheck {
			if len(out) > 0 {
				return fmt.Errorf("generated artifacts drifted: %v", out)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Generated artifacts are up to date in %s\n", root)
			return nil
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Generated %d artifact(s) in %s\n", len(out), root)
		return nil
	},
}

func init() {
	generateCmd.Flags().StringVar(&generateTarget, "target", "all", `generate target ("all", "claude", "codex-package", "codex-runtime", "gemini", "opencode", "cursor", or "cursor-workspace")`)
	generateCmd.Flags().BoolVar(&generateCheck, "check", false, "fail if generated artifacts are out of date")
}
