package main

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/spf13/cobra"
)

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap [path]",
	Short: "Bootstrap repo-local interpreted runtime dependencies",
	Long: `Bootstrap repo-local interpreted runtime dependencies for package-standard projects.

This helper is bounded to repo-local launcher-based lanes. It does not replace ecosystem package managers or the binary-only install flow.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root := "."
		if len(args) == 1 {
			root = args[0]
		}
		result, err := pluginService.Bootstrap(context.Background(), app.PluginBootstrapOptions{Root: root})
		if err != nil {
			return err
		}
		for _, line := range result.Lines {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
		}
		return nil
	},
}
