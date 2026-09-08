package agentpluginscli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	"github.com/spf13/cobra"
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

func runSwitch(ctx context.Context, cmd *cobra.Command, app App, opts *options, selector, source string) error {
	state, err := app.StateStore.Load()
	if err != nil {
		return err
	}
	installation, err := selectInstallation(state, selector)
	if err != nil {
		return err
	}
	installationID := installation.InstallationID
	targetIDs := installationTargets(installation, string(domain.ScopeUser))
	var detected map[domain.ClientID]domain.DetectedClient
	if len(targetIDs) > 0 {
		_, detected, err = preflightSelectedTargets(ctx, app, targetIDs, nil, !opts.dryRun && isDirectorySelector(source))
		if err != nil {
			return err
		}
	}
	loaded, err := app.loadPackageFor(ctx, source, packageResolutionRequest{Targets: targetIDs, Operation: domain.DirectoryInstall, Clients: detected})
	if err != nil {
		return err
	}
	if loaded.cleanup != nil {
		defer loaded.cleanup()
	}
	if loaded.envelope.Manifest.Name != installation.DeclaredName {
		return fmt.Errorf("switch source manifest name %q does not match installed product %q", loaded.envelope.Manifest.Name, installation.DeclaredName)
	}
	if err := authorizeSecurityAssessment(cmd, app, opts, &loaded); err != nil {
		return err
	}
	output := switchOutput{DryRun: true, Status: "planned", Source: publicPackageSource(loaded.envelope.Source), Revision: loaded.envelope.Source.ResolvedRevision,
		TreeDigest: loaded.envelope.TreeDigest, ManifestDigest: loaded.envelope.ManifestDigest, Directory: cloneDirectoryOrigin(loaded.directory),
	}
	service := app.Lifecycle
	service.StateStore = app.StateStore
	if installation.DataRetained && len(installation.Clients) == 0 {
		planned, err := service.SwitchRetained(ctx, usecase.BindingChangeInput{Selector: installationID, Envelope: loaded.envelope}, loaded.origin, loaded.directory)
		output.Retained = &planned
		output.PluginData = planned.PluginData
		if err != nil {
			output.Status = "preflight_failed"
			if renderErr := renderSwitchResult(cmd.OutOrStdout(), opts.format, output); renderErr != nil {
				return renderErr
			}
			return err
		}
		if opts.dryRun {
			return renderSwitchResult(cmd.OutOrStdout(), opts.format, output)
		}
		if opts.format == "human" {
			if err := renderSwitchResult(cmd.OutOrStdout(), opts.format, output); err != nil {
				return err
			}
		}
		applied, err := service.SwitchRetained(ctx, usecase.BindingChangeInput{Selector: installationID, Envelope: loaded.envelope, Confirmed: true}, loaded.origin, loaded.directory)
		output.DryRun, output.Retained = false, &applied
		output.PluginData = applied.PluginData
		if err != nil {
			output.Status = "apply_failed"
		} else {
			output.Status = "completed"
		}
		if renderErr := renderSwitchResult(cmd.OutOrStdout(), opts.format, output); renderErr != nil && err == nil {
			err = renderErr
		}
		return err
	}
	service = lifecycleService(app, detected)
	inputs := make([]usecase.AddInput, 0, len(targetIDs))
	for _, targetID := range targetIDs {
		client := detected[targetID]
		clientPackage := cloneLoadedPackage(loaded)
		if err := prepareLoadedPackageForClient(&clientPackage, targetID); err != nil {
			return err
		}
		inputs = append(inputs, usecase.AddInput{Envelope: clientPackage.envelope, Client: client, Scope: domain.ScopeUser, Hints: clientPackage.hints, InstallationID: installationID,
			BackendExecutable: backendExecutable(client, detected), OriginMode: loaded.origin, DirectoryResolution: cloneDirectoryOrigin(loaded.directory),
			DistributionSuspended: loaded.distributionSuspended, ReleaseRevoked: loaded.releaseRevoked})
	}
	operationID, err := newOperationGroupID()
	if err != nil {
		return err
	}
	planned, err := service.SwitchGroup(ctx, usecase.GroupInput{Targets: inputs, OperationGroupID: operationID, DryRun: true, Switch: true})
	output.Group = &planned
	output.PluginData = planned.PluginData
	output.Targets = switchTargets(planned)
	if err != nil {
		output.Status = "preflight_failed"
		if renderErr := renderSwitchResult(cmd.OutOrStdout(), opts.format, output); renderErr != nil {
			return renderErr
		}
		return fmt.Errorf("switch preflight failed; no target was changed: %w", err)
	}
	if opts.dryRun {
		return renderSwitchResult(cmd.OutOrStdout(), opts.format, output)
	}
	if opts.format == "human" {
		if err := renderSwitchResult(cmd.OutOrStdout(), opts.format, output); err != nil {
			return err
		}
	}
	applied, err := service.SwitchGroup(ctx, usecase.GroupInput{Targets: inputs, OperationGroupID: operationID, Confirmed: true, Switch: true})
	output.DryRun, output.Group = false, &applied
	output.PluginData = applied.PluginData
	output.Targets = switchTargets(applied)
	if err != nil {
		output.Status = groupFailureStatus(applied.Phase)
	} else {
		output.Status = string(applied.Phase)
	}
	if renderErr := renderSwitchResult(cmd.OutOrStdout(), opts.format, output); renderErr != nil && err == nil {
		err = renderErr
	}
	return err
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
	var action string
	if len(result.Activation.UserActions) > 0 {
		action = result.Activation.UserActions[0]
	} else if len(result.Plan.UserActions) > 0 {
		action = result.Plan.UserActions[0]
	}
	if result.Activation.Authentication == domain.AuthenticationPending || (result.Activation.Authentication == "" && result.Plan.Authentication == domain.AuthenticationPending) {
		if action != "" {
			return action + "; complete authentication, then verify activation and authentication in the target client"
		}
		return "complete authentication, then verify activation and authentication in the target client"
	}
	if result.Activation.Authentication == domain.AuthenticationNotChecked || (result.Activation.Authentication == "" && result.Plan.Authentication == domain.AuthenticationNotChecked) {
		if action != "" {
			return action + "; verify authentication requirements before using the plugin"
		}
		return "verify authentication requirements before using the plugin"
	}
	if action != "" {
		return action
	}
	if fullyInstalled(result.Activation) {
		return "start a new client session and verify the switched plugin is available"
	}
	return "finish activation in the target client, authenticate if requested, then verify the switched plugin is available"
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
	service := app.Lifecycle
	service.StateStore = app.StateStore
	if installation.DataRetained && len(installation.Clients) == 0 {
		if len(targets) != 0 {
			return fmt.Errorf("a data_retained purge does not accept --target")
		}
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
	if len(targets) == 0 {
		return fmt.Errorf("removing active bindings with --purge-data requires an explicit --target list")
	}
	return runRemoveMany(ctx, cmd, app, opts, installation.InstallationID, targets)
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
