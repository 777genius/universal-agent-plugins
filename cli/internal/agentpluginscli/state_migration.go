package agentpluginscli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statemigration"
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

type migrateStateSession struct {
	ctx     context.Context
	cmd     *cobra.Command
	app     App
	opts    *options
	current bool
	plan    statemigration.Plan
}

func runMigrateState(ctx context.Context, cmd *cobra.Command, app App, opts *options) error {
	session := &migrateStateSession{ctx: ctx, cmd: cmd, app: app, opts: opts}
	if err := session.loadPlan(); err != nil {
		return err
	}
	if opts.dryRun {
		return renderStateMigration(cmd.OutOrStdout(), opts.format, session.plan, statemigration.Report{}, true)
	}
	confirmed, err := session.confirm()
	if err != nil {
		return err
	}
	if !confirmed {
		return session.canceled()
	}
	if session.current {
		return session.migrateCurrent()
	}
	return session.migrateExpected()
}

func (session *migrateStateSession) loadPlan() error {
	if session.app.StateMigrator == nil {
		return fmt.Errorf("state migration is not configured")
	}
	_, statErr := os.Stat(session.app.StateMigrator.V2Store.Path)
	session.current = statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		return statErr
	}
	var plan statemigration.Plan
	var err error
	if session.current {
		plan, err = session.app.StateMigrator.PlanCurrentV2()
	} else {
		plan, err = session.app.StateMigrator.Plan()
	}
	if err != nil {
		return err
	}
	session.plan = plan
	return nil
}

func (session *migrateStateSession) confirm() (bool, error) {
	if session.opts.format == "human" {
		renderStateMigrationPlan(session.cmd.OutOrStdout(), session.plan)
	}
	confirmed := mutationConfirmed(session.app, session.opts)
	if !confirmed && session.opts.format == "human" && session.app.Terminal {
		var err error
		confirmed, err = promptYesNo(session.cmd.Context(), session.cmd.InOrStdin(), session.cmd.OutOrStdout(), session.cmd.ErrOrStderr(), "Create a backup and migrate this state? [y/N]")
		if err != nil {
			return false, err
		}
	}
	return confirmed, nil
}

func (session *migrateStateSession) canceled() error {
	if session.opts.format == "json" {
		return renderStateMigration(session.cmd.OutOrStdout(), session.opts.format, session.plan, statemigration.Report{}, false)
	}
	_, _ = fmt.Fprintln(session.cmd.OutOrStdout(), "No changes made.")
	return nil
}

func (session *migrateStateSession) migrateCurrent() error {
	report, err := session.app.StateMigrator.MigrateCurrentV2(session.ctx, session.plan.LegacyDigest)
	if err != nil {
		return err
	}
	return renderStateMigration(session.cmd.OutOrStdout(), session.opts.format, session.plan, report, false)
}

func (session *migrateStateSession) migrateExpected() error {
	if session.app.Lifecycle.Lock == nil {
		return fmt.Errorf("agentplugins mutation lock is required")
	}
	release, err := session.app.Lifecycle.Lock.Acquire(session.ctx)
	if err != nil {
		return err
	}
	defer func() { _ = release() }()
	if session.app.LegacyStateLock == nil {
		return fmt.Errorf("legacy state lock is required")
	}
	legacyRelease, err := session.app.LegacyStateLock.Acquire(session.ctx, "state")
	if err != nil {
		return fmt.Errorf("acquire legacy state lock: %w", err)
	}
	defer func() { _ = legacyRelease() }()
	if err := session.recoverKernel(); err != nil {
		return err
	}
	report, err := session.app.StateMigrator.MigrateExpected(session.plan.LegacyDigest)
	if err != nil {
		return err
	}
	return renderStateMigration(session.cmd.OutOrStdout(), session.opts.format, session.plan, report, false)
}

func (session *migrateStateSession) recoverKernel() error {
	kernel := session.app.Lifecycle.Kernel
	kernel.StateStore = session.app.StateStore
	if err := kernel.Recover(session.ctx); err != nil {
		return fmt.Errorf("recover interrupted mutation before state migration: %w", err)
	}
	return nil
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
	return renderStateMigrationApplied(writer, plan, report)
}

func renderStateMigrationApplied(writer io.Writer, plan statemigration.Plan, report statemigration.Report) error {
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
