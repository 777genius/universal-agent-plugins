package agentpluginscli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	"github.com/spf13/cobra"
)

func newUpdateCommand(app App, opts *options) *cobra.Command {
	var all bool
	command := &cobra.Command{
		Use:   "update <name-or-installation-id> | update --all",
		Short: "Safely update a tracked Agent Plugin across selected or installed clients",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, scopeErr := app.forOptions(opts)
			if scopeErr != nil {
				return scopeErr
			}
			if err := validateCommonOptions(opts); err != nil {
				return err
			}
			operationApp := app
			operationApp.Lifecycle.PrepareHostsForPreview = !opts.dryRun
			if all {
				if len(args) > 0 {
					return fmt.Errorf("choose either one installation or --all")
				}
				if strings.TrimSpace(opts.target) != "" {
					return fmt.Errorf("--target cannot be combined with --all; every installation keeps its recorded targets")
				}
				return runUpdateAll(cmd.Context(), cmd, operationApp, opts)
			}
			if len(args) != 1 {
				return fmt.Errorf("provide one installation or use --all")
			}
			if handled, err := runSavedLifecycle(cmd, operationApp, opts, args[0], "update"); handled || err != nil {
				return err
			}
			if strings.TrimSpace(opts.target) == "" {
				targets, err := defaultInstalledBindingTargets(cmd.Context(), app, args[0], opts.scope, false)
				if err != nil {
					return err
				}
				opts.target = joinTargets(targets)
				defer func() { opts.target = "" }()
			}
			targets, err := parseTargetOption(opts.target)
			if err != nil {
				return err
			}
			return runUpdateMany(cmd.Context(), cmd, operationApp, opts, args[0], targets)
		},
	}
	command.Flags().BoolVar(&all, "all", false, "update saved dependencies and eligible tracked installations after one complete preflight")
	return command
}

func newRepairCommand(app App, opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "repair <name-or-installation-id>", Short: "Explicitly restore a missing or modified managed package",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, scopeErr := app.forOptions(opts)
			if scopeErr != nil {
				return scopeErr
			}
			if err := validateCommonOptions(opts); err != nil {
				return err
			}
			operationApp := app
			operationApp.Lifecycle.PrepareHostsForPreview = !opts.dryRun
			if handled, err := runSavedLifecycle(cmd, operationApp, opts, args[0], "repair"); handled || err != nil {
				return err
			}
			if strings.TrimSpace(opts.target) == "" {
				targets, err := defaultInstalledBindingTargets(cmd.Context(), app, args[0], opts.scope, true)
				if err != nil {
					return err
				}
				opts.target = joinTargets(targets)
				defer func() { opts.target = "" }()
			}
			stdin := bufio.NewReader(cmd.InOrStdin())
			targets, err := parseTargetOption(opts.target)
			if err != nil {
				return err
			}
			_ = stdin
			if handled, err := repairOwnedDependencies(cmd, operationApp, opts, args[0], targets); handled || err != nil {
				return err
			}
			return runRepairMany(cmd.Context(), cmd, operationApp, opts, args[0], targets)
		},
	}
}

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

func renderRepairResult(writer io.Writer, format string, installation domain.Installation, result usecase.AddResult, dryRun bool) error {
	data := newRepairResultData(installation, result, dryRun)
	if format == "json" {
		return writeJSONOutput(writer, "repair", data)
	}
	if err := renderOpenCodeRuntimeNotice(writer, result); err != nil {
		return err
	}
	if !dryRun {
		if action := localTargetLifecycleAction(result, ""); action != "" {
			if _, err := fmt.Fprintf(writer, "Next: %s\n", action); err != nil {
				return err
			}
		}
	}
	if result.NoChange {
		_, err := fmt.Fprintln(writer, "Managed package digest is valid. No repair was needed.")
		return err
	}
	if dryRun {
		_, err := fmt.Fprintln(writer, "The managed package or its recorded verification state can be safely repaired. No changes made.")
		return err
	}
	if result.Mutated {
		if result.Receipt.OperationID == "" {
			_, err := fmt.Fprintln(writer, "Managed package digest reverified and recorded lifecycle state corrected; no external client mutation was attempted.")
			return err
		} else {
			_, err := fmt.Fprintln(writer, "Managed package repaired from the exact installed revision and its package digest verified; external client activation and authentication were not reverified.")
			return err
		}
	}
	return nil
}

type repairResultData struct {
	OperationID    string            `json:"operation_id,omitempty"`
	Plugin         string            `json:"plugin"`
	Version        string            `json:"version,omitempty"`
	Source         string            `json:"source"`
	TreeDigest     string            `json:"tree_digest"`
	ManifestDigest string            `json:"manifest_digest"`
	NextAction     string            `json:"next_action,omitempty"`
	DryRun         bool              `json:"dry_run"`
	Result         usecase.AddResult `json:"result"`
}

func newRepairResultData(installation domain.Installation, result usecase.AddResult, dryRun bool) repairResultData {
	return repairResultData{
		OperationID: result.Receipt.OperationID, Plugin: installation.DeclaredName,
		Version: installation.Package.Version, Source: publicSource(installation.Source),
		TreeDigest: installation.Source.TreeDigest, ManifestDigest: installation.Package.ManifestDigest,
		NextAction: nextLifecycleAction(result), DryRun: dryRun, Result: result,
	}
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
