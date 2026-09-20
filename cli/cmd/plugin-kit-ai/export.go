package main

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/spf13/cobra"
)

type exportRunner interface {
	Export(app.PluginExportOptions) (app.PluginExportResult, error)
}

var (
	exportPlatform string
	exportOutput   string
)

var exportCmd = newExportCmd(pluginService)

func newExportCmd(runner exportRunner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export [path]",
		Short: "Create a portable interpreted-runtime bundle without changing install semantics",
		Long: `Create a deterministic portable .tar.gz bundle for launcher-based interpreted runtime projects.

This beta surface is a bounded handoff/export flow for python, node, and shell runtime repos.
It does not extend plugin-kit-ai install, and it does not imply marketplace packaging or dependency-preinstalled installs.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			result, err := runner.Export(app.PluginExportOptions{
				Root:     root,
				Platform: exportPlatform,
				Output:   exportOutput,
			})
			if err != nil {
				return err
			}
			for _, line := range result.Lines {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&exportPlatform, "platform", "", `target override ("codex-runtime" or "claude")`)
	cmd.Flags().StringVar(&exportOutput, "output", "", "write bundle to this .tar.gz path (default: <root>/<name>_<platform>_<runtime>_bundle.tar.gz)")
	_ = cmd.MarkFlagRequired("platform")
	return cmd
}
