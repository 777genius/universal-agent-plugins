package agentpluginscli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func renderRepairMultiResult(cmd *cobra.Command, opts *options, result repairMultiResult) error {
	if opts.format == "json" {
		overall := "success"
		if result.Failed > 0 {
			overall = "failure"
		}
		return writeJSONResult(cmd.OutOrStdout(), "repair", overall, result)
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Repair %s: %s\n", result.Plugin, result.Status); err != nil {
		return err
	}
	if err := renderRepairTargets(cmd, result); err != nil {
		return err
	}
	if result.Status == "completed" {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Managed package repaired from the exact installed revision and its package digest verified; external client activation and authentication were not reverified."); err != nil {
			return err
		}
	}
	return nil
}

func renderRepairTargets(cmd *cobra.Command, result repairMultiResult) error {
	for _, target := range result.Targets {
		if err := renderOpenCodeRuntimeNotice(cmd.OutOrStdout(), target.Output.Result); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", target.Target, target.Status); err != nil {
			return err
		}
		if target.NextAction != "" {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "    Next: %s\n", localTargetLifecycleAction(target.Output.Result, target.NextAction)); err != nil {
				return err
			}
		}
	}
	return nil
}
