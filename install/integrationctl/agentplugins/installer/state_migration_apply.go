package installer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statemigration"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
)

// ApplyStateMigration applies a host-confirmed, unchanged review. It reuses the
// explicit migrator, including its mutation lock, pending-operation checks,
// digest recheck under that lock, exact backup, validation, and atomic save.
// A no-change plan is also rechecked, so a newly appeared old state cannot be
// mistaken for the missing or current state the host reviewed.
func (e *Engine) ApplyStateMigration(ctx context.Context, plan StateMigrationPlan) (StateMigrationResult, error) {
	if ctx == nil {
		return StateMigrationResult{}, fmt.Errorf("%w: context is required", ErrInvalidRequest)
	}
	if err := ctx.Err(); err != nil {
		return StateMigrationResult{}, err
	}
	if err := e.CheckProjectNamespace(); err != nil {
		return StateMigrationResult{}, err
	}
	if plan.StateRoot != e.cfg.StateRoot || plan.StateFile != e.cfg.StateFile ||
		plan.LockFile != e.cfg.LockFile || plan.OperationsDir != e.cfg.OperationsDir {
		return StateMigrationResult{}, ErrPlanChanged
	}
	// Compare the reviewed bytes before decoding. Changed corrupt or future
	// state is a stale review, rather than a diagnosis of the reviewed state.
	if err := e.checkStateMigrationDigest(plan.StateDigest); err != nil {
		return StateMigrationResult{}, err
	}
	live, err := e.PrepareStateMigration(ctx)
	if err != nil {
		if changed := e.checkStateMigrationDigest(plan.StateDigest); errors.Is(changed, ErrPlanChanged) {
			return StateMigrationResult{}, changed
		}
		return StateMigrationResult{}, err
	}
	if live != plan {
		return StateMigrationResult{}, ErrPlanChanged
	}
	if !plan.Required {
		return StateMigrationResult{NoChange: true}, nil
	}
	report, err := e.stateMigrator().MigrateCurrentV2(ctx, plan.StateDigest)
	result := StateMigrationResult{BackupPath: report.BackupPath, Migrated: report.Migrated, NeedsRebind: report.NeedsRebind}
	// A replacement can also race the precheck and the migrator's read-only
	// validation. Once a backup exists, retain the actual commit error instead.
	if err != nil && report.BackupPath == "" {
		if changed := e.checkStateMigrationDigest(plan.StateDigest); errors.Is(changed, ErrPlanChanged) {
			return result, changed
		}
	}
	if errors.Is(err, statemigration.ErrPlanChanged) {
		return result, fmt.Errorf("%w: %w", ErrPlanChanged, err)
	}
	if errors.Is(err, statemigration.ErrPendingRecovery) {
		return result, fmt.Errorf("%w: %w", ErrRecoveryRequired, err)
	}
	return result, err
}

func (e *Engine) checkStateMigrationDigest(expected string) error {
	body, err := os.ReadFile(e.cfg.StateFile)
	if os.IsNotExist(err) {
		if expected == "missing" {
			return nil
		}
		return ErrPlanChanged
	}
	if err != nil {
		return fmt.Errorf("read authoritative state: %w", err)
	}
	if migrationDigest(body) != expected {
		return ErrPlanChanged
	}
	return nil
}

func (e *Engine) stateMigrator() statemigration.Migrator {
	return statemigration.Migrator{
		V2Store: statev2.Store{Path: e.cfg.StateFile, ProjectRoot: e.cfg.ProjectRoot},
		Lock:    processlock.Lock{Path: e.cfg.LockFile},
		PendingCheck: func(ctx context.Context) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			observed, err := e.observe()
			if err != nil {
				return err
			}
			if observed.Recovery.Required {
				return fmt.Errorf("%w: old-schema pending operations require the corresponding previous SDK/binary; recover with it, then prepare migration again", ErrRecoveryRequired)
			}
			return nil
		},
	}
}

func migrationDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}
