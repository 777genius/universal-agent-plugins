package installer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statemigration"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// StateMigrationPlan is a read-only review of the authoritative state. Hosts may
// serialize it between confirmation and apply. Keep every field unchanged and
// pass it to an engine configured for the same state paths.
type StateMigrationPlan struct {
	StateRoot     string `json:"state_root"`
	StateFile     string `json:"state_file"`
	LockFile      string `json:"lock_file"`
	OperationsDir string `json:"operations_dir"`
	StateDigest   string `json:"state_digest"`
	Required      bool   `json:"required"`
	SourceSchema  int    `json:"source_schema"`
	TargetSchema  int    `json:"target_schema"`
	Installations int    `json:"installations"`
	NeedsRebind   int    `json:"needs_rebind"`
}

// StateMigrationResult includes a backup even when committing the migrated
// state fails. The original state is never removed before that backup exists.
type StateMigrationResult struct {
	BackupPath  string `json:"backup_path,omitempty"`
	Migrated    int    `json:"migrated"`
	NeedsRebind int    `json:"needs_rebind"`
	NoChange    bool   `json:"no_change"`
}

// PrepareStateMigration validates and reviews the supported schema 2-4 upgrade
// without acquiring locks, creating paths, recovering operations, or invoking
// host callbacks. Missing and current state return Required=false. Corrupt,
// unsupported, and old state with pending operations are refused.
func (e *Engine) PrepareStateMigration(ctx context.Context) (StateMigrationPlan, error) {
	if ctx == nil {
		return StateMigrationPlan{}, fmt.Errorf("%w: context is required", ErrInvalidRequest)
	}
	if err := ctx.Err(); err != nil {
		return StateMigrationPlan{}, err
	}
	if err := e.CheckProjectNamespace(); err != nil {
		return StateMigrationPlan{}, err
	}
	plan := StateMigrationPlan{StateRoot: e.cfg.StateRoot, StateFile: e.cfg.StateFile,
		LockFile: e.cfg.LockFile, OperationsDir: e.cfg.OperationsDir, TargetSchema: domain.StateSchemaVersion}
	body, err := os.ReadFile(e.cfg.StateFile)
	if os.IsNotExist(err) {
		plan.StateDigest, plan.SourceSchema = "missing", domain.StateSchemaVersion
		return plan, nil
	}
	if err != nil {
		return StateMigrationPlan{}, fmt.Errorf("read authoritative state: %w", err)
	}
	state, err := e.store.Load()
	if err != nil {
		return StateMigrationPlan{}, err
	}
	var header struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(body, &header); err != nil {
		return StateMigrationPlan{}, fmt.Errorf("decode authoritative state header: %w", err)
	}
	plan.StateDigest, plan.SourceSchema, plan.Installations = migrationDigest(body), header.SchemaVersion, len(state.Installations)
	if header.SchemaVersion == domain.StateSchemaVersion {
		return plan, nil
	}
	review, err := e.stateMigrator().PlanCurrentV2()
	if err != nil {
		// A concurrent migration may publish the current schema between the
		// read above and the migrator's read-only review.
		if live, readErr := os.ReadFile(e.cfg.StateFile); readErr == nil && migrationDigest(live) != plan.StateDigest {
			return StateMigrationPlan{}, ErrPlanChanged
		}
		if errors.Is(err, statemigration.ErrPendingRecovery) {
			return StateMigrationPlan{}, fmt.Errorf("%w: %w", ErrRecoveryRequired, err)
		}
		return StateMigrationPlan{}, err
	}
	if review.LegacyDigest != plan.StateDigest || review.SourceSchema != plan.SourceSchema {
		return StateMigrationPlan{}, ErrPlanChanged
	}
	plan.Required, plan.Installations, plan.NeedsRebind = true, review.Installations, review.NeedsRebind
	return plan, nil
}
