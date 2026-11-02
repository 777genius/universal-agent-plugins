package agentpluginscli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"github.com/spf13/cobra"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func newRemoveCommand(app App, opts *options) *cobra.Command {
	command := &cobra.Command{
		Use:   "remove <name-or-installation-id>",
		Short: "Safely remove selected targets while retaining plugin data by default",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, scopeErr := app.forOptions(opts)
			if scopeErr != nil {
				return scopeErr
			}
			if err := validateCommonOptions(opts); err != nil {
				return err
			}
			if handled, err := runSavedLifecycle(cmd, app, opts, args[0], "remove"); handled || err != nil {
				return err
			}
			// JSON calls must reject missing selectors before
			// loading state, presenting choices, or touching an open input stream.
			// Purging retained data is installation-wide and needs no target.
			if opts.format == "json" && !opts.purgeData {
				if err := requireNonInteractiveMutation(app, opts, "removal"); err != nil {
					return err
				}
			}
			if strings.TrimSpace(opts.target) == "" && app.Terminal && !opts.purgeData {
				selection, err := promptBoundTargets(cmd, app, args[0], opts.scope)
				if err != nil {
					return err
				}
				opts.target = selection
				defer func() { opts.target = "" }()
			}
			if opts.purgeData {
				return runRemove(cmd.Context(), cmd, app, opts, args[0])
			}
			targets, err := parseTargetOption(opts.target)
			if err != nil {
				return err
			}
			if len(targets) == 0 {
				return runRemove(cmd.Context(), cmd, app, opts, args[0])
			}
			if len(targets) == 1 && targets[0] == "legacy-all" {
				return runRemove(cmd.Context(), cmd, app, opts, args[0])
			}
			return runRemoveMany(cmd.Context(), cmd, app, opts, args[0], targets)
		},
	}
	command.Flags().BoolVar(&opts.externalUninstalled, "external-uninstalled", false, "confirm the selected client plugin was uninstalled manually or was never activated/imported")
	command.Flags().BoolVar(&opts.purgeData, "purge-data", false, "permanently delete ownership-verified plugin data after removal")
	return command
}

func defaultBoundTargets(app App, selector, scope string, degradedOnly bool) ([]domain.ClientID, error) {
	state, err := app.StateStore.Load()
	if err != nil {
		return nil, err
	}
	installation, err := selectInstallation(state, selector)
	if err != nil {
		return nil, err
	}
	var targets []domain.ClientID
	for _, binding := range installation.Clients {
		if binding.Scope != scope || binding.Materialization == domain.MaterializationAbsent {
			continue
		}
		if degradedOnly && binding.Materialization != domain.MaterializationDegraded && binding.Activation != domain.ActivationFailed && binding.Activation != domain.ActivationManual && binding.Activation != domain.ActivationPrepared && binding.Authentication != domain.AuthenticationFailed && binding.Authentication != domain.AuthenticationPending && binding.Verification != domain.VerificationFailed {
			continue
		}
		targets = append(targets, bindingSurfaceTargets(binding)...)
	}
	if len(targets) == 0 {
		if degradedOnly {
			return defaultBoundTargets(app, selector, scope, false)
		}
		return nil, fmt.Errorf("plugin has no materialized target in %s scope", scope)
	}
	sortTargets(targets)
	return targets, nil
}

func defaultInstalledBindingTargets(ctx context.Context, app App, selector, scope string, degradedOnly bool) ([]domain.ClientID, error) {
	state, err := app.StateStore.Load()
	if err != nil {
		return nil, err
	}
	installation, err := selectInstallation(state, selector)
	if err != nil {
		return nil, err
	}
	var bindingTargets []domain.ClientID
	for _, binding := range installation.Clients {
		if binding.Scope != scope || binding.Materialization == domain.MaterializationAbsent {
			continue
		}
		if degradedOnly && binding.Materialization != domain.MaterializationDegraded && binding.Activation != domain.ActivationFailed && binding.Activation != domain.ActivationManual && binding.Activation != domain.ActivationPrepared && binding.Authentication != domain.AuthenticationFailed && binding.Authentication != domain.AuthenticationPending && binding.Verification != domain.VerificationFailed {
			continue
		}
		bindingTargets = append(bindingTargets, domain.ClientID(binding.ClientID))
	}
	if len(bindingTargets) == 0 {
		if degradedOnly {
			return defaultInstalledBindingTargets(ctx, app, selector, scope, false)
		}
		return nil, fmt.Errorf("plugin has no materialized target in %s scope", scope)
	}
	return resolveInstalledBindingTargets(ctx, app, bindingTargets, false)
}

func promptBoundTargets(cmd *cobra.Command, app App, selector, scope string) (string, error) {
	targets, err := defaultBoundTargets(app, selector, scope, false)
	if err != nil {
		return "", err
	}
	if len(targets) == 1 {
		return string(targets[0]), nil
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Installed targets (all selected by default):")
	for index, target := range targets {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d. [x] %s\n", index+1, target)
	}
	_, _ = fmt.Fprint(cmd.OutOrStdout(), "Choose targets by number, comma-separated [all]: ")
	line, readErr := readInputLine(cmd.Context(), cmd.InOrStdin())
	if readErr != nil && readErr != io.EOF {
		return "", readErr
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return joinTargets(targets), nil
	}
	seen := map[int]struct{}{}
	var selected []domain.ClientID
	for _, raw := range strings.Split(line, ",") {
		choice, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || choice < 1 || choice > len(targets) {
			return "", fmt.Errorf("invalid installed-target multiselect")
		}
		if _, duplicate := seen[choice]; duplicate {
			return "", fmt.Errorf("duplicate installed-target multiselect choice %d", choice)
		}
		seen[choice] = struct{}{}
		selected = append(selected, targets[choice-1])
	}
	return joinTargets(selected), nil
}

func runRemove(ctx context.Context, cmd *cobra.Command, app App, opts *options, selector string) error {
	state, err := app.StateStore.Load()
	if err != nil {
		return err
	}
	installation, err := selectInstallation(state, selector)
	if err != nil {
		return err
	}
	if opts.purgeData {
		return runPurgeData(ctx, cmd, app, opts, installation)
	}
	if installation.Package.LoaderKind == domain.LoaderKindLegacy {
		return runLegacyRemove(ctx, cmd, app, opts, installation)
	}
	clients, err := app.Detector.Detect(ctx)
	if err != nil {
		return fmt.Errorf("detect AI clients: %w", err)
	}
	selected, detectedMap, err := selectBoundClient(cmd, app, opts, installation, clients, false, cmd.InOrStdin())
	if err != nil {
		return err
	}
	if err := requireNonInteractiveMutation(app, opts, "removal"); err != nil && !opts.dryRun {
		return err
	}
	service := lifecycleService(app, detectedMap)
	authority, err := environmentAuthorityForBinding(state, installation.InstallationID, selected.ClientID, "remove")
	if err != nil {
		return err
	}
	input := usecase.RemoveInput{
		Selector: installation.InstallationID, Client: selected, Scope: domain.InstallScope(opts.scope),
		DryRun: opts.dryRun, Interactive: app.Terminal,
		ExternalUninstalled:  opts.externalUninstalled,
		BackendExecutable:    backendExecutable(selected, detectedMap, app.scope()),
		EnvironmentAuthority: authority,
	}
	planned, err := service.Remove(ctx, input)
	if err != nil {
		return err
	}
	if opts.dryRun {
		return renderRemoveResult(cmd.OutOrStdout(), opts.format, installation, planned, true)
	}
	if !planned.Deactivation.ArtifactRemovalAllowed {
		if opts.format == "human" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Plugin: %s %s\n", installation.DeclaredName, installation.Package.Version)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Target: %s\n", selected.ClientID)
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Result: removal is blocked until the external client uninstall is confirmed")
		}
		return renderRemoveResult(cmd.OutOrStdout(), opts.format, installation, planned, false)
	}
	if opts.format == "human" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Plugin: %s %s\n", installation.DeclaredName, installation.Package.Version)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Target: %s\n", selected.ClientID)
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Result: managed package and its lifecycle binding will be removed")
	}
	confirmed := mutationConfirmed(app, opts)
	if !confirmed && opts.format == "human" && app.Terminal {
		confirmed, err = promptYesNo(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), "Remove this target? [y/N]")
		if err != nil {
			return err
		}
	}
	if !confirmed {
		if opts.format == "json" {
			return renderRemoveResult(cmd.OutOrStdout(), opts.format, installation, planned, false)
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No changes made.")
		return nil
	}
	writeProgress(app, opts.format, "Removing the selected managed package...")
	input.Confirmed = true
	result, removeErr := service.Remove(ctx, input)
	if renderErr := renderRemoveResult(cmd.OutOrStdout(), opts.format, installation, result, false); renderErr != nil && removeErr == nil {
		removeErr = renderErr
	}
	return removeErr
}

func runLegacyRemove(ctx context.Context, cmd *cobra.Command, app App, opts *options, installation domain.Installation) error {
	if app.LegacyLifecycle == nil {
		return fmt.Errorf("legacy lifecycle bridge is not configured")
	}
	if target := strings.TrimSpace(opts.target); target != "" && target != "legacy-all" {
		return fmt.Errorf("legacy removal is integration-wide; use --target legacy-all after reviewing every target")
	}
	if !opts.dryRun && automatedMutation(app, opts) && strings.TrimSpace(opts.target) != "legacy-all" {
		return fmt.Errorf("automated legacy removal requires --target legacy-all")
	}
	service := app.Lifecycle
	service.StateStore = app.StateStore
	service.Legacy = app.LegacyLifecycle
	service.LegacyLock = app.LegacyStateLock
	input := usecase.LegacyRemoveInput{Selector: installation.InstallationID, DryRun: opts.dryRun}
	planned, err := service.RemoveLegacy(ctx, input)
	if err != nil {
		return err
	}
	if opts.dryRun {
		return renderLegacyRemove(cmd.OutOrStdout(), opts.format, planned, true)
	}
	if opts.format == "human" {
		renderLegacyRemovePlan(cmd.OutOrStdout(), planned)
	}
	confirmed := mutationConfirmed(app, opts)
	if !confirmed && opts.format == "human" && app.Terminal {
		confirmed, err = promptYesNo(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), "Remove every legacy target listed above? [y/N]")
		if err != nil {
			return err
		}
	}
	if !confirmed {
		if opts.format == "json" {
			return renderLegacyRemove(cmd.OutOrStdout(), opts.format, planned, false)
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No changes made.")
		return nil
	}
	input.Confirmed = true
	result, removeErr := service.RemoveLegacy(ctx, input)
	if renderErr := renderLegacyRemove(cmd.OutOrStdout(), opts.format, result, false); renderErr != nil && removeErr == nil {
		removeErr = renderErr
	}
	return removeErr
}

func renderLegacyRemove(writer io.Writer, format string, result usecase.LegacyRemoveResult, dryRun bool) error {
	data := struct {
		DryRun bool                       `json:"dry_run"`
		Result usecase.LegacyRemoveResult `json:"result"`
	}{DryRun: dryRun, Result: result}
	if format == "json" {
		return writeJSONOutput(writer, "remove", data)
	}
	if dryRun {
		renderLegacyRemovePlan(writer, result)
		return nil
	}
	if result.Mutated {
		_, _ = fmt.Fprintln(writer, "Legacy targets removed and Agent Plugins state reconciled.")
	}
	return nil
}

func renderLegacyRemovePlan(writer io.Writer, result usecase.LegacyRemoveResult) {
	_, _ = fmt.Fprintf(writer, "Legacy plugin: %s\n", result.Plugin)
	_, _ = fmt.Fprintln(writer, "Removal is integration-wide through the original plugin-kit-ai lifecycle:")
	for _, target := range result.Targets {
		_, _ = fmt.Fprintf(writer, "  - %s\n", target)
	}
	if result.Reconciled {
		_, _ = fmt.Fprintln(writer, "Legacy lifecycle already reports the installation absent; only Agent Plugins state reconciliation remains.")
	}
}
