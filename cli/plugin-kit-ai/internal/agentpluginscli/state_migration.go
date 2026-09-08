package agentpluginscli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statemigration"
	"github.com/spf13/cobra"
)

func newMigrateStateCommand(app App, opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "migrate-state",
		Short: "Explicitly migrate legacy plugin-kit-ai state into Agent Plugins state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateCommonOptions(opts); err != nil {
				return err
			}
			return runMigrateState(cmd.Context(), cmd, app, opts)
		},
	}
}

func runMigrateState(ctx context.Context, cmd *cobra.Command, app App, opts *options) error {
	if app.StateMigrator == nil {
		return fmt.Errorf("state migration is not configured")
	}
	_, statErr := os.Stat(app.StateMigrator.V2Store.Path)
	current := statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		return statErr
	}
	var plan statemigration.Plan
	var err error
	if current {
		plan, err = app.StateMigrator.PlanCurrentV2()
	} else {
		plan, err = app.StateMigrator.Plan()
	}
	if err != nil {
		return err
	}
	if opts.dryRun {
		return renderStateMigration(cmd.OutOrStdout(), opts.format, plan, statemigration.Report{}, true)
	}
	if opts.format == "human" {
		renderStateMigrationPlan(cmd.OutOrStdout(), plan)
	}
	confirmed := mutationConfirmed(app, opts)
	if !confirmed && opts.format == "human" && app.Terminal {
		confirmed, err = promptYesNo(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), "Create a backup and migrate this state? [y/N]")
		if err != nil {
			return err
		}
	}
	if !confirmed {
		if opts.format == "json" {
			return renderStateMigration(cmd.OutOrStdout(), opts.format, plan, statemigration.Report{}, false)
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No changes made.")
		return nil
	}
	if current {
		report, err := app.StateMigrator.MigrateCurrentV2(ctx, plan.LegacyDigest)
		if err != nil {
			return err
		}
		return renderStateMigration(cmd.OutOrStdout(), opts.format, plan, report, false)
	}
	if app.Lifecycle.Lock == nil {
		return fmt.Errorf("agentplugins mutation lock is required")
	}
	release, err := app.Lifecycle.Lock.Acquire(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = release() }()
	if app.LegacyStateLock == nil {
		return fmt.Errorf("legacy state lock is required")
	}
	legacyRelease, err := app.LegacyStateLock.Acquire(ctx, "state")
	if err != nil {
		return fmt.Errorf("acquire legacy state lock: %w", err)
	}
	defer func() { _ = legacyRelease() }()
	kernel := app.Lifecycle.Kernel
	kernel.StateStore = app.StateStore
	if err := kernel.Recover(ctx); err != nil {
		return fmt.Errorf("recover interrupted mutation before state migration: %w", err)
	}
	report, err := app.StateMigrator.MigrateExpected(plan.LegacyDigest)
	if err != nil {
		return err
	}
	return renderStateMigration(cmd.OutOrStdout(), opts.format, plan, report, false)
}

func renderStateMigration(writer io.Writer, format string, plan statemigration.Plan, report statemigration.Report, dryRun bool) error {
	data := struct {
		DryRun        bool `json:"dry_run"`
		SourceSchema  int  `json:"source_schema,omitempty"`
		Installations int  `json:"installations"`
		NeedsRebind   int  `json:"needs_rebind"`
		Migrated      int  `json:"migrated"`
		BackupCreated bool `json:"backup_created"`
	}{
		DryRun: dryRun, SourceSchema: plan.SourceSchema, Installations: plan.Installations, NeedsRebind: plan.NeedsRebind,
		Migrated: report.Migrated, BackupCreated: report.BackupPath != "",
	}
	if format == "json" {
		return writeJSONOutput(writer, "migrate-state", data)
	}
	if dryRun {
		renderStateMigrationPlan(writer, plan)
		return nil
	}
	if plan.SourceSchema != 0 {
		_, _ = fmt.Fprintf(writer, "Migrated %d authoritative schema %d installation(s); backup created before Agent Plugins state commit.\n", report.Migrated, plan.SourceSchema)
	} else {
		_, _ = fmt.Fprintf(writer, "Migrated %d legacy installation(s); backup created before Agent Plugins state commit.\n", report.Migrated)
	}
	if report.NeedsRebind > 0 {
		_, _ = fmt.Fprintf(writer, "%d installation(s) require explicit rebind before update.\n", report.NeedsRebind)
	}
	if plan.SourceSchema == 0 {
		_, _ = fmt.Fprintln(writer, "Legacy packages remain removable with plugin-kit-ai integrations remove.")
	}
	return nil
}

func renderStateMigrationPlan(writer io.Writer, plan statemigration.Plan) {
	if plan.SourceSchema != 0 {
		_, _ = fmt.Fprintf(writer, "Authoritative schema %d installations: %d\n", plan.SourceSchema, plan.Installations)
	} else {
		_, _ = fmt.Fprintf(writer, "Legacy installations: %d\n", plan.Installations)
	}
	_, _ = fmt.Fprintf(writer, "Require rebind: %d\n", plan.NeedsRebind)
	if plan.SourceSchema != 0 {
		_, _ = fmt.Fprintln(writer, "The authoritative state remains unchanged during planning and is backed up before the migrated state is committed.")
	} else {
		_, _ = fmt.Fprintln(writer, "The legacy state remains unchanged and is backed up before Agent Plugins state is committed.")
	}
}
