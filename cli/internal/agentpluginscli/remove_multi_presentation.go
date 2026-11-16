package agentpluginscli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func renderRemoveMultiResult(cmd *cobra.Command, opts *options, result removeMultiResult) error {
	if opts.format == "json" {
		overall := "success"
		if result.Failed > 0 {
			overall = "failure"
		}
		return writeJSONResult(cmd.OutOrStdout(), "remove", overall, result)
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Remove %s: %s\n", result.Plugin, removeStatusText(result.Status)); err != nil {
		return err
	}
	for _, target := range result.Targets {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", target.Target, removeStatusText(target.Status)); err != nil {
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

// removeStatusText keeps JSON statuses stable while human output reads as text.
// Retained data has its own PLUGIN_DATA line, so the removal itself is "removed".
func removeStatusText(status string) string {
	switch status {
	case "data_retained", string(usecase.GroupPhaseCompleted), string(usecase.GroupTargetExternalCompleted):
		return "removed"
	case string(usecase.GroupTargetExternalPartial):
		return "removed from the client; managed cleanup is incomplete"
	case string(usecase.GroupTargetExternalFailed):
		return "failed"
	}
	return strings.ReplaceAll(status, "_", " ")
}
