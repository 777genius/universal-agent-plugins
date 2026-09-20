package main

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/spf13/cobra"
)

var normalizeForce bool

var normalizeCmd = &cobra.Command{
	Use:   "normalize [path]",
	Short: "Normalize package-standard plugin.yaml",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root := "."
		if len(args) == 1 {
			root = args[0]
		}
		warnings, err := pluginService.Normalize(app.PluginNormalizeOptions{
			Root:  root,
			Force: normalizeForce,
		})
		if err != nil {
			return err
		}
		for _, warning := range warnings {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Warning: %s\n", warning.Message)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Normalized plugin.yaml in %s\n", root)
		return nil
	},
}

func init() {
	normalizeCmd.Flags().BoolVarP(&normalizeForce, "force", "f", true, "rewrite plugin.yaml with normalized content")
}
