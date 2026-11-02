package agentpluginscli

import (
	"fmt"
	"github.com/spf13/cobra"
)

func renderRemoveMultiResult(cmd *cobra.Command, opts *options, result removeMultiResult) error {
	if opts.format == "json" {
		overall := "success"
		if result.Failed > 0 {
			overall = "failure"
		}
		return writeJSONResult(cmd.OutOrStdout(), "remove", overall, result)
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Remove %s: %s\n", result.Plugin, result.Status); err != nil {
		return err
	}
	for _, target := range result.Targets {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", target.Target, target.Status); err != nil {
			return err
		}
		if target.Status == "blocked" && target.Output.NextAction != "" {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "    Next: %s\n", target.Output.NextAction); err != nil {
				return err
			}
		}
	}
	if !result.DryRun && multiRemovalNeedsManualStep(result) {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), "  "+removalIncompleteNotice); err != nil {
			return err
		}
	}
	if result.PluginDataPreserved {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), "  PLUGIN_DATA: preserved"); err != nil {
			return err
		}
	}
	if result.RetainedDataAction != "" {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  Next: %s\n", result.RetainedDataAction); err != nil {
			return err
		}
	}
	return nil
}
