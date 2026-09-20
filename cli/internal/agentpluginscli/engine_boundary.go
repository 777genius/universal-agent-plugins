package agentpluginscli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

type switchOutput struct {
	DryRun         bool                         `json:"dry_run"`
	Source         string                       `json:"source"`
	Revision       string                       `json:"revision,omitempty"`
	TreeDigest     string                       `json:"tree_digest"`
	ManifestDigest string                       `json:"manifest_digest"`
	Directory      *domain.DirectoryOrigin      `json:"directory,omitempty"`
	Group          *usecase.GroupResult         `json:"group,omitempty"`
	Retained       *usecase.BindingChangeResult `json:"retained,omitempty"`
	PluginData     domain.PluginDataDecision    `json:"plugin_data"`
	Targets        []switchTargetOutput         `json:"targets"`
	Status         string                       `json:"status"`
}

type switchTargetOutput struct {
	ClientID   domain.ClientID `json:"client_id"`
	Status     string          `json:"status"`
	NextAction string          `json:"next_action"`
}

func newSwitchCommand(app App, opts *options) *cobra.Command {
	var destination string
	command := &cobra.Command{
		Use:   "switch <name-or-installation-id> --to <distribution-or-exact-source>",
		Short: "Move a complete Agent Plugin installation to a reviewed source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateCommonOptions(opts); err != nil {
				return err
			}
			if strings.TrimSpace(opts.target) != "" {
				return fmt.Errorf("switch always moves the complete installation; do not pass --target")
			}
			if strings.TrimSpace(destination) == "" {
				return fmt.Errorf("switch requires --to with a qualified distribution or exact source")
			}
			if isShortName(strings.TrimSpace(destination)) {
				return fmt.Errorf("switch --to requires a qualified distribution ID or exact local/full-SHA source; a short name could select a changing default")
			}
			return runSwitch(cmd.Context(), cmd, app, opts, args[0], destination)
		},
	}
	command.Flags().StringVar(&destination, "to", "", "qualified distribution ID or exact immutable/local source")
	_ = command.MarkFlagRequired("to")
	return command
}

func renderSwitchResult(writer io.Writer, format string, result switchOutput) error {
	if format == "json" {
		return writeJSONOutput(writer, "switch", result)
	}
	status := result.Status
	if status == "" {
		status = "planned"
	}
	if status == string(usecase.GroupPhaseCompleted) && (result.PluginData.Present || switchHasPendingActions(result.Targets)) {
		_, _ = fmt.Fprintln(writer, "Switch: completed; follow-up required")
	} else {
		_, _ = fmt.Fprintf(writer, "Switch: %s\n", status)
	}
	return renderSwitchHumanDetails(writer, result)
}

func renderSwitchHumanDetails(writer io.Writer, result switchOutput) error {
	if result.PluginData.Present {
		_, _ = fmt.Fprintf(writer, "  PLUGIN_DATA: retained (%s; compatibility %s)\n", result.PluginData.Ownership, result.PluginData.Compatibility)
		_, _ = fmt.Fprintf(writer, "  Warning: %s\n", result.PluginData.Warning)
	}
	for _, target := range result.Targets {
		_, _ = fmt.Fprintf(writer, "  %s: %s\n", target.ClientID, target.Status)
		if target.NextAction != "" {
			_, _ = fmt.Fprintf(writer, "    Next: %s\n", target.NextAction)
		}
	}
	if result.Retained != nil {
		_, _ = fmt.Fprintln(writer, "  No active targets; source metadata switched and retained data was not changed.")
	}
	return nil
}

func switchTargets(group usecase.GroupResult) []switchTargetOutput {
	targets := make([]switchTargetOutput, 0, len(group.Targets))
	for _, target := range group.Targets {
		status := groupTargetStatus(target)
		if status == "" {
			status = string(target.Plan.Status)
		}
		targets = append(targets, switchTargetOutput{ClientID: target.Plan.ClientID, Status: status, NextAction: nextSwitchAction(target)})
	}
	return targets
}

func nextSwitchAction(result usecase.AddResult) string {
	action := firstSwitchUserAction(result)
	if pending := pendingAuthSwitchAction(result, action); pending != "" {
		return pending
	}
	if unchecked := uncheckedAuthSwitchAction(result, action); unchecked != "" {
		return unchecked
	}
	if action != "" {
		return action
	}
	if fullyInstalled(result.Activation) {
		return "start a new client session and verify the switched plugin is available"
	}
	return "finish activation in the target client, authenticate if requested, then verify the switched plugin is available"
}

func firstSwitchUserAction(result usecase.AddResult) string {
	if len(result.Activation.UserActions) > 0 {
		return result.Activation.UserActions[0]
	}
	if len(result.Plan.UserActions) > 0 {
		return result.Plan.UserActions[0]
	}
	return ""
}

func pendingAuthSwitchAction(result usecase.AddResult, action string) string {
	if result.Activation.Authentication != domain.AuthenticationPending && (result.Activation.Authentication != "" || result.Plan.Authentication != domain.AuthenticationPending) {
		return ""
	}
	if action != "" {
		return action + "; complete authentication, then verify activation and authentication in the target client"
	}
	return "complete authentication, then verify activation and authentication in the target client"
}

func uncheckedAuthSwitchAction(result usecase.AddResult, action string) string {
	if result.Activation.Authentication != domain.AuthenticationNotChecked && (result.Activation.Authentication != "" || result.Plan.Authentication != domain.AuthenticationNotChecked) {
		return ""
	}
	if action != "" {
		return action + "; verify authentication requirements before using the plugin"
	}
	return "verify authentication requirements before using the plugin"
}

func switchHasPendingActions(targets []switchTargetOutput) bool {
	for _, target := range targets {
		if target.NextAction != "" {
			return true
		}
	}
	return false
}

func (result switchOutput) outputResult() string {
	switch result.Status {
	case "", "planned", "completed":
		return outputResultSuccess
	default:
		return outputResultFailure
	}
}

func runPurgeData(ctx context.Context, cmd *cobra.Command, app App, opts *options, installation domain.Installation) error {
	targets, err := parseTargetOption(opts.target)
	if err != nil {
		return err
	}
	if installation.DataRetained && len(installation.Clients) == 0 {
		return runPurgeRetainedData(ctx, cmd, app, opts, installation, targets)
	}
	if len(targets) == 0 {
		return fmt.Errorf("removing active bindings with --purge-data requires an explicit --target list")
	}
	return runRemoveMany(ctx, cmd, app, opts, installation.InstallationID, targets)
}

func runPurgeRetainedData(ctx context.Context, cmd *cobra.Command, app App, opts *options, installation domain.Installation, targets []domain.ClientID) error {
	if len(targets) != 0 {
		return fmt.Errorf("a data_retained purge does not accept --target")
	}
	service := app.Lifecycle
	service.StateStore = app.StateStore
	if err := service.PurgeRetainedData(ctx, installation.InstallationID, false); err != nil {
		return err
	}
	if opts.dryRun {
		return renderPurgeDataResult(cmd.OutOrStdout(), opts.format, installation, true)
	}
	if err := service.PurgeRetainedData(ctx, installation.InstallationID, true); err != nil {
		return err
	}
	return renderPurgeDataResult(cmd.OutOrStdout(), opts.format, installation, false)
}

func renderPurgeDataResult(writer io.Writer, format string, installation domain.Installation, dryRun bool) error {
	data := struct {
		Plugin string `json:"plugin"`
		DryRun bool   `json:"dry_run"`
		Status string `json:"status"`
	}{Plugin: installation.DeclaredName, DryRun: dryRun, Status: map[bool]string{true: "planned", false: "purged"}[dryRun]}
	if format == "json" {
		return writeJSONOutput(writer, "remove", data)
	}
	_, err := fmt.Fprintf(writer, "Data purge: %s\n", data.Status)
	return err
}
