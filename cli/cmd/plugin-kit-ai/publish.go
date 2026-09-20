package main

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/spf13/cobra"
)

type publishRunner interface {
	Publish(app.PluginPublishOptions) (app.PluginPublishResult, error)
}

var publishCmd = newPublishCmd(pluginService)

func newPublishCmd(runner publishRunner) *cobra.Command {
	flags := publishFlags{Format: "text"}
	cmd := &cobra.Command{
		Use:   "publish [path]",
		Short: "Publish a package target through a bounded channel workflow",
		Long: `Publish a package target through a bounded channel-family workflow.

This first-class publish entrypoint is intentionally bounded to documented channel flows:
- codex-marketplace
- claude-marketplace
- gemini-gallery (dry-run plan only)
- all authored channels (dry-run plan only)

Codex and Claude materialize a safe local marketplace root.
		Gemini stays repository/release rooted, so publish only supports --dry-run planning there instead of a local marketplace materialization path.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPublishCommand(cmd, runner, flags, args)
		},
	}
	cmd.Flags().StringVar(&flags.Channel, "channel", "", `publish channel ("codex-marketplace", "claude-marketplace", or "gemini-gallery")`)
	cmd.Flags().BoolVar(&flags.All, "all", false, "plan across all authored publication channels (dry-run only)")
	cmd.Flags().StringVar(&flags.Dest, "dest", "", "destination marketplace root directory for local Codex/Claude marketplace flows")
	cmd.Flags().StringVar(&flags.PackageRoot, "package-root", "", "relative package root inside the destination marketplace root (default: plugins/<name>)")
	cmd.Flags().BoolVar(&flags.DryRun, "dry-run", false, "preview the materialized publish result without writing changes")
	cmd.Flags().StringVar(&flags.Format, "format", "text", `output format ("text" or "json")`)
	return cmd
}

func buildPublishJSONPayload(result app.PluginPublishResult) map[string]any {
	details := result.Details
	if details == nil {
		details = map[string]string{}
	}
	issues := result.Issues
	if issues == nil {
		issues = []app.PluginPublishIssue{}
	}
	warnings := result.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	nextSteps := result.NextSteps
	if nextSteps == nil {
		nextSteps = []string{}
	}
	channels := buildPublishJSONPayloads(result.Channels)
	payload := map[string]any{
		"format":          "plugin-kit-ai/publish-report",
		"schema_version":  1,
		"ready":           result.Ready,
		"status":          result.Status,
		"mode":            result.Mode,
		"workflow_class":  result.WorkflowClass,
		"detail_count":    len(details),
		"details":         details,
		"issue_count":     len(issues),
		"issues":          append([]app.PluginPublishIssue{}, issues...),
		"next_step_count": len(nextSteps),
		"next_steps":      nextSteps,
	}
	if result.Channel != "" {
		payload["channel"] = result.Channel
	}
	if result.Target != "" {
		payload["target"] = result.Target
	}
	if result.Dest != "" {
		payload["dest"] = result.Dest
	}
	if result.PackageRoot != "" {
		payload["package_root"] = result.PackageRoot
	}
	if result.WorkflowClass == "multi_channel_plan" || len(warnings) > 0 {
		payload["warning_count"] = len(warnings)
		payload["warnings"] = append([]string(nil), warnings...)
	}
	if result.WorkflowClass == "multi_channel_plan" || len(channels) > 0 {
		payload["channel_count"] = len(channels)
		payload["channels"] = channels
	}
	return payload
}

func buildPublishJSONPayloads(results []app.PluginPublishResult) []map[string]any {
	return buildPublishJSONPayloadCollection(results)
}
