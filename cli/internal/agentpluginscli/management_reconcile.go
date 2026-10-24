package agentpluginscli

import (
	"context"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// A second, explicit recovery review restores current desired intent. This is
// a new preparation with fresh receipts and source facts, never an old job replay.
func (b *ManagementBackend) prepareRecoveryConvergence(ctx context.Context, input management.ChangeRequest, operationID string) (management.Prepared, error) {
	preview, cleanup, err := dependencyPreviewApp(b.app, &options{dryRun: true})
	if err != nil {
		return nil, err
	}
	manifest, _ := preview.dependencyPaths()
	var prepared *managementPrepared
	if _, statErr := os.Stat(manifest); os.IsNotExist(statErr) {
		prepared, err = b.prepareEnvironment(ctx, preview, cleanup, input, nil, operationID)
	} else {
		var request dependencySyncRequest
		request, err = prepareDependencySyncCandidateContext(ctx, preview, &options{dryRun: true}, false)
		if err == nil {
			prepared, err = b.finishManagementPreparation(ctx, preview, cleanup, input, nil, request, operationID)
		}
	}
	if err != nil {
		cleanup()
		return nil, err
	}
	prepared.review.Warnings = append(prepared.review.Warnings, managementDiagnostic("FRESH_RECOVERY", "This fresh review reconciles the complete current environment; it does not resume an earlier operation"))
	return prepared, nil
}

func (b *ManagementBackend) proveManagementRecovery(ctx context.Context, app App, out management.Outcome) management.Outcome {
	unlock, err := (processlock.Lock{Path: app.EnvironmentLockPath}).Acquire(ctx)
	if err != nil {
		return managementRecoveryPending(out)
	}
	defer func() { _ = unlock() }()
	return b.proveManagementRecoveryLocked(ctx, app, out)
}

// The caller holds environment.lock; mutation.lock also fences imperative CLI
// operations and local Skill edits while the full settled observation is made.
func (b *ManagementBackend) proveManagementRecoveryLocked(ctx context.Context, app App, out management.Outcome) management.Outcome {
	unlock, err := (processlock.Lock{Path: filepath.Join(b.config.StateRoot, "mutation.lock")}).Acquire(ctx)
	if err != nil {
		return managementRecoveryPending(out)
	}
	defer func() { _ = unlock() }()
	settled, next, err := b.managementRecoveryCoherent(ctx, app)
	if err == nil && settled {
		out.RecoverySettled = true
		out.Result.NextActions = append(out.Result.NextActions, next...)
		return out
	}
	return managementRecoveryPending(out)
}

func managementRecoveryPending(out management.Outcome) management.Outcome {
	out.State = management.Partial
	out.Result.Outcome = "incomplete"
	out.Result.Message = "Receipts were inspected; the complete desired environment still needs reconciliation"
	out.Result.NextActions = append(out.Result.NextActions, management.NextAction{Kind: "recover", Reason: "Prepare another fresh recovery review to reconcile current uap.toml, uap.lock and native bindings"})
	return out
}

func (b *ManagementBackend) managementRecoveryCoherent(ctx context.Context, app App) (bool, []management.NextAction, error) {
	snapshot, err := snapshotEnvironmentContext(ctx, app)
	if err != nil {
		return false, nil, err
	}
	if err = b.validateManagementEnvironment(ctx, snapshot.Requirements); err != nil {
		return false, nil, err
	}
	view, err := app.Installer.Inspect(ctx)
	if err != nil {
		return false, nil, err
	}
	observed, err := environment.Observe(view)
	if err != nil {
		return false, nil, err
	}
	if snapshot.Lock.LockVersion == 0 {
		return len(view.EnvironmentReceipts) == 0, nil, nil
	}
	prepared, err := (environment.Coordinator{}).PrepareSync(snapshot.Requirements, snapshot.Lock, observed)
	if err != nil {
		return false, nil, err
	}
	for _, action := range prepared.Bindings {
		if action.Action != environment.ActionUnchanged {
			return false, nil, nil
		}
	}
	next := []management.NextAction{}
	for _, installation := range view.Installations {
		for _, binding := range installation.Bindings {
			if binding.Owner == nil {
				continue
			}
			retry, facts := environment.LifecycleFacts(binding)
			if retry || binding.ProjectionStatus != "" {
				return false, nil, nil
			}
			for _, fact := range facts {
				next = append(next, management.NextAction{Kind: "readiness", Reason: "Known readiness state: " + fact, Alias: binding.Owner.Alias, TargetId: b.targetID(binding.ClientID)})
			}
		}
	}
	return true, next, nil
}
