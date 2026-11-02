package agentpluginscli

import (
	"fmt"
	"strings"
	"github.com/spf13/cobra"
)

func renderUpdateMultiResult(cmd *cobra.Command, opts *options, result updateMultiResult) error {
	if opts.format == "json" {
		overall := "success"
		if result.Failed > 0 {
			overall = "failure"
		}
		return writeJSONResult(cmd.OutOrStdout(), "update", overall, result)
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Plugin: %s %s\n", result.Plugin, result.Version); err != nil {
		return err
	}
	values := make([]string, len(result.Targets))
	for index, target := range result.Targets {
		if err := renderUpdateTarget(cmd, target); err != nil {
			return err
		}
		values[index] = target.Target
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "Targets: %s\n", strings.Join(values, ","))
	return err
}

func renderUpdateTarget(cmd *cobra.Command, target updateTargetResult) error {
	if err := renderOpenCodeRuntimeNotice(cmd.OutOrStdout(), target.Output.Result); err != nil {
		return err
	}
	rollout := "preflight only"
	if target.Selected {
		rollout = "selected"
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s (%s)\n", target.Target, target.Status, rollout); err != nil {
		return err
	}
	if target.NextAction != "" && !fullyInstalled(target.Output.Result.Activation) {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "    Next: %s\n", localTargetLifecycleAction(target.Output.Result, target.NextAction)); err != nil {
			return err
		}
	}
	return nil
}

func groupNextAction(targets []updateTargetResult) string {
	for _, target := range targets {
		if target.NextAction != "" {
			return "; next action: " + target.NextAction
		}
	}
	return ""
}
