package agentpluginscli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func runRepair(ctx context.Context, cmd *cobra.Command, app App, opts *options, selector string, stdin *bufio.Reader) error {
	state, err := app.StateStore.Load()
	if err != nil {
		return err
	}
	installation, err := selectInstallation(state, selector)
	if err != nil {
		return err
	}
	if installation.NeedsRebind || installation.Package.LoaderKind != domain.LoaderKindAgentPlugins {
		return fmt.Errorf("repair requires a bound Agent Plugins installation")
	}
	clients, err := app.Detector.Detect(ctx)
	if err != nil {
		return fmt.Errorf("detect AI clients: %w", err)
	}
	selected, detectedMap, err := selectBoundClient(cmd, app, opts, installation, clients, true, stdin)
	if err != nil {
		return err
	}
	binding, err := selectedRepairBinding(installation, selected.ClientID, domain.InstallScope(opts.scope))
	if err != nil {
		return err
	}
	source, err := repairSource(installation, binding)
	if err != nil {
		return err
	}
	writeProgress(app, opts.format, "Resolving and validating the exact installed Agent Plugin revision...")
	var loaded loadedPackage
	if installation.Source.SourceKind != "" {
		loaded, err = app.loadRecordedDirect(ctx, installation, binding)
	} else {
		loaded, err = app.loadPackage(ctx, source)
	}
	if err != nil {
		return err
	}
	if loaded.cleanup != nil {
		defer loaded.cleanup()
	}
	restoreCatalogEvidence(&loaded, binding)
	if err := authorizeSecurityAssessment(cmd, app, opts, &loaded); err != nil {
		return err
	}
	if err := prepareLoadedPackageForClient(&loaded, selected.ClientID); err != nil {
		return err
	}
	if err := requireNonInteractiveMutation(app, opts, "repair"); err != nil && !opts.dryRun {
		return err
	}
	service := lifecycleService(app, detectedMap)
	authority, err := environmentAuthorityForBinding(state, installation.InstallationID, selected.ClientID, "repair")
	if err != nil {
		return err
	}
	input := usecase.AddInput{Envelope: loaded.envelope, Client: selected, Scope: domain.InstallScope(opts.scope), DryRun: opts.dryRun,
		InstallationID: installation.InstallationID, Interactive: app.Terminal, Hints: loaded.hints,
		BackendExecutable: backendExecutable(selected, detectedMap, app.scope()), EnvironmentAuthority: authority}
	planned, err := service.Repair(ctx, input)
	if err != nil {
		if planned.Plan.Status == domain.PlanUnsupported {
			if opts.format == "json" {
				if renderErr := renderRepairResult(cmd.OutOrStdout(), opts.format, installation, planned, opts.dryRun); renderErr != nil {
					return renderErr
				}
			} else if renderErr := renderHumanPlan(cmd.OutOrStdout(), loaded.envelope, planned); renderErr != nil {
				return renderErr
			}
		}
		return err
	}
	if opts.dryRun || planned.NoChange {
		return renderRepairResult(cmd.OutOrStdout(), opts.format, installation, planned, opts.dryRun)
	}
	confirmed := mutationConfirmed(app, opts)
	if !confirmed && opts.format == "human" && app.Terminal {
		confirmed, err = promptYesNo(cmd.Context(), stdin, cmd.OutOrStdout(), cmd.ErrOrStderr(), "Repair the managed package and its recorded verification state? [y/N]")
		if err != nil {
			return err
		}
	}
	if !confirmed {
		if opts.format == "json" {
			return renderRepairResult(cmd.OutOrStdout(), opts.format, installation, planned, false)
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "No changes made.")
		return err
	}
	retainPlannedClient(&input, planned.Plan)
	input.Confirmed = true
	result, repairErr := service.Repair(ctx, input)
	if renderErr := renderRepairResult(cmd.OutOrStdout(), opts.format, installation, result, false); renderErr != nil && repairErr == nil {
		repairErr = renderErr
	}
	return repairErr
}

func runUpdate(ctx context.Context, cmd *cobra.Command, app App, opts *options, selector string) error {
	state, err := app.StateStore.Load()
	if err != nil {
		return err
	}
	installation, err := selectInstallation(state, selector)
	if err != nil {
		return err
	}
	if installation.NeedsRebind {
		return fmt.Errorf("installation %s requires explicit rebind before update", installation.InstallationID)
	}
	if installation.Package.LoaderKind != domain.LoaderKindAgentPlugins {
		return fmt.Errorf("legacy plugin.yaml installations cannot switch format during update; use plugin-kit-ai integrations update or agentplugins migrate-format")
	}
	writeProgress(app, opts.format, "Resolving and validating the updated Agent Plugin...")
	loaded, err := app.loadPackage(ctx, updateSource(installation))
	if err != nil {
		return err
	}
	if loaded.cleanup != nil {
		defer loaded.cleanup()
	}
	if domain.ComputeSourceBindingID(loaded.envelope.Source) != installation.Source.SourceBindingID {
		return fmt.Errorf("resolved source identity changed; use agentplugins rebind after reviewing provenance")
	}
	if err := authorizeSecurityAssessment(cmd, app, opts, &loaded); err != nil {
		return err
	}
	clients, err := app.Detector.Detect(ctx)
	if err != nil {
		return fmt.Errorf("detect AI clients: %w", err)
	}
	selected, detectedMap, err := selectBoundClient(cmd, app, opts, installation, clients, true, cmd.InOrStdin())
	if err != nil {
		return err
	}
	if err := prepareLoadedPackageForClient(&loaded, selected.ClientID); err != nil {
		return err
	}
	if err := requireNonInteractiveMutation(app, opts, "update"); err != nil && !opts.dryRun {
		return err
	}
	service := lifecycleService(app, detectedMap)
	authority, err := environmentAuthorityForBinding(state, installation.InstallationID, selected.ClientID, "update")
	if err != nil {
		return err
	}
	input := usecase.AddInput{
		Envelope: loaded.envelope, Client: selected, Scope: domain.InstallScope(opts.scope),
		DryRun: opts.dryRun, Interactive: app.Terminal, Hints: loaded.hints,
		BackendExecutable:                backendExecutable(selected, detectedMap, app.scope()),
		PersistAuthoritativeObservations: true,
		EnvironmentAuthority:             authority,
	}
	planned, err := service.Update(ctx, input)
	if err != nil {
		if planned.Plan.Status == domain.PlanUnsupported {
			if opts.format == "json" {
				if renderErr := renderUpdateResult(cmd.OutOrStdout(), opts.format, loaded.envelope, planned, opts.dryRun); renderErr != nil {
					return renderErr
				}
			} else if renderErr := renderHumanPlan(cmd.OutOrStdout(), loaded.envelope, planned); renderErr != nil {
				return renderErr
			}
		}
		return err
	}
	if opts.dryRun || planned.NoChange {
		return renderUpdateResult(cmd.OutOrStdout(), opts.format, loaded.envelope, planned, opts.dryRun)
	}
	if opts.format == "human" {
		if err := renderHumanPlan(cmd.OutOrStdout(), loaded.envelope, planned); err != nil {
			return err
		}
	}
	confirmed := mutationConfirmed(app, opts)
	if !confirmed && opts.format == "human" && app.Terminal {
		confirmed, err = promptYesNo(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), "Apply this update? [y/N]")
		if err != nil {
			return err
		}
	}
	if !confirmed {
		if opts.format == "json" {
			return renderUpdateResult(cmd.OutOrStdout(), opts.format, loaded.envelope, planned, false)
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No changes made.")
		return nil
	}
	writeProgress(app, opts.format, "Applying transactional package update...")
	retainPlannedClient(&input, planned.Plan)
	input.Confirmed = true
	result, updateErr := service.Update(ctx, input)
	if renderErr := renderUpdateResult(cmd.OutOrStdout(), opts.format, loaded.envelope, result, false); renderErr != nil && updateErr == nil {
		updateErr = renderErr
	}
	return updateErr
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

func selectBoundClient(
	cmd *cobra.Command,
	app App,
	opts *options,
	installation domain.Installation,
	clients []domain.DetectedClient,
	requireDetected bool,
	reader io.Reader,
) (domain.DetectedClient, map[domain.ClientID]domain.DetectedClient, error) {
	detectedMap := make(map[domain.ClientID]domain.DetectedClient, len(clients))
	for _, client := range clients {
		detectedMap[client.ClientID] = client
	}
	bound := make(map[domain.ClientID]struct{})
	for _, binding := range installation.Clients {
		if binding.Materialization != domain.MaterializationAbsent && binding.Scope == opts.scope {
			bound[domain.ClientID(binding.ClientID)] = struct{}{}
		}
	}
	if len(bound) == 0 {
		return domain.DetectedClient{}, detectedMap, fmt.Errorf("plugin has no materialized target in %s scope", opts.scope)
	}
	choose := func(clientID domain.ClientID) (domain.DetectedClient, error) {
		if _, ok := bound[clientID]; !ok {
			return domain.DetectedClient{}, fmt.Errorf("plugin is not installed for target %q in %s scope", clientID, opts.scope)
		}
		if err := selectedDetectionError(clientID, detectedMap); err != nil {
			return domain.DetectedClient{}, err
		}
		client, ok := detectedMap[clientID]
		if !ok {
			client = domain.DetectedClient{ClientID: clientID, DisplayName: string(clientID), Status: domain.DetectionNotDetected}
		}
		if requireDetected && client.Status != domain.DetectionDetected && !plansWithoutHostPresence(clientID) {
			return domain.DetectedClient{}, fmt.Errorf("target %q is no longer detected; remove remains available", clientID)
		}
		return client, nil
	}
	if strings.EqualFold(strings.TrimSpace(opts.target), "openai") {
		return domain.DetectedClient{}, detectedMap, fmt.Errorf("target %q is ambiguous; use --target codex or --target chatgptweb", opts.target)
	}
	if target := normalizeTarget(opts.target); target != "" {
		client, err := choose(target)
		return client, detectedMap, err
	}
	ids := make([]domain.ClientID, 0, len(bound))
	for clientID := range bound {
		ids = append(ids, clientID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if len(ids) == 1 {
		client, err := choose(ids[0])
		return client, detectedMap, err
	}
	if !app.Terminal || opts.format == "json" {
		return domain.DetectedClient{}, detectedMap, fmt.Errorf("plugin has multiple installed targets; choose one or more with --target codex,cursor")
	}
	if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Installed targets:"); err != nil {
		return domain.DetectedClient{}, detectedMap, err
	}
	for index, clientID := range ids {
		client := detectedMap[clientID]
		display := client.DisplayName
		if display == "" {
			display = string(clientID)
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s\n", index+1, display); err != nil {
			return domain.DetectedClient{}, detectedMap, err
		}
	}
	if _, err := fmt.Fprint(cmd.OutOrStdout(), "Choose one target: "); err != nil {
		return domain.DetectedClient{}, detectedMap, err
	}
	line, readErr := readInputLine(cmd.Context(), reader)
	if readErr != nil && readErr != io.EOF {
		return domain.DetectedClient{}, detectedMap, readErr
	}
	choice, parseErr := strconv.Atoi(strings.TrimSpace(line))
	if parseErr != nil || choice < 1 || choice > len(ids) {
		return domain.DetectedClient{}, detectedMap, fmt.Errorf("invalid client selection")
	}
	client, err := choose(ids[choice-1])
	return client, detectedMap, err
}
