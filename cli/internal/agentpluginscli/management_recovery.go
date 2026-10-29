package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"reflect"
	"sort"
	"sync"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

type managementRecovery struct {
	mu       sync.Mutex
	backend  *ManagementBackend
	observed installer.Inspection
	files    dependencySyncRequest
	review   management.Plan
	closed   bool
}

func (b *ManagementBackend) prepareRecovery(ctx context.Context, input management.ChangeRequest, operationID string) (management.Prepared, error) {
	if err := b.validateManagementEnvironment(ctx, environment.Requirements{}); err != nil {
		return nil, err
	}
	observed, err := b.app.Installer.Inspect(ctx)
	if err != nil {
		return nil, err
	}
	if !observed.Recovery.Required {
		return b.prepareRecoveryConvergence(ctx, input, operationID)
	}
	if input.ExternalUninstalled {
		return nil, managementRefusal("EXTERNAL_UNINSTALL_NOT_APPLICABLE", "First review and reconcile pending installer receipts without externalUninstalled; a separate fresh review will identify any native removals requiring confirmation")
	}
	files, err := managementRecoveryFiles(b.app)
	if err != nil {
		return nil, err
	}
	review := management.Plan{Action: management.Recover, ContextId: b.ContextID(), Confirmable: true, Actions: []management.PlanAction{}, Warnings: []management.Diagnostic{}, Conflicts: []management.Diagnostic{}}
	for _, journal := range observed.Recovery.Journals {
		review.Actions = append(review.Actions, management.PlanAction{Action: "recover", Alias: journal.InstallationID, Description: "Recover the observed pending transaction", DestinationPath: journal.TargetPath})
	}
	for _, receipt := range observed.Recovery.Receipts {
		review.Actions = append(review.Actions, management.PlanAction{Action: "recover", Alias: receipt.InstallationID, Description: "Reconcile the observed pending receipt", DestinationPath: receipt.TargetPath})
	}
	for _, intent := range observed.Recovery.NativeIntents {
		review.Actions = append(review.Actions, management.PlanAction{Action: "recover", Alias: intent.Binding.InstallationID, TargetId: b.targetID(intent.Binding.ClientID), Description: "Reconcile the recorded native attempt"})
	}
	review.Warnings = append(review.Warnings, managementDiagnostic("RECOVERY_RECHECK", "After receipt reconciliation, a separate review may be required to restore the desired environment"))
	return &managementRecovery{backend: b, observed: observed, files: files, review: review}, nil
}

func managementRecoveryFiles(app App) (dependencySyncRequest, error) {
	manifest, lock := app.dependencyPaths()
	files := dependencySyncRequest{manifestPath: manifest, lockPath: lock}
	var err error
	files.manifestBytes, err = managementReadOptional(manifest)
	if err != nil {
		return files, err
	}
	files.lockBytes, err = managementReadOptional(lock)
	return files, err
}
func managementReadOptional(path string) ([]byte, error) {
	body, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return body, err
}
func (p *managementRecovery) Review() management.Plan { return p.review }
func (p *managementRecovery) OperationIDs() []string {
	seen := map[string]bool{}
	for _, journal := range p.observed.Recovery.Journals {
		seen[journal.OperationID] = true
	}
	for _, receipt := range p.observed.Recovery.Receipts {
		seen[receipt.OperationID] = true
	}
	for _, intent := range p.observed.Recovery.NativeIntents {
		seen[intent.Intent.AttemptID] = true
	}
	ids := []string{}
	for id := range seen {
		if id != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}
func (p *managementRecovery) Close() { p.mu.Lock(); defer p.mu.Unlock(); p.closed = true }
func (p *managementRecovery) Check(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.check(ctx)
}
func (p *managementRecovery) check(ctx context.Context) error {
	if p.closed {
		return management.ErrPlanStale
	}
	if err := p.backend.validateManagementEnvironment(ctx, environment.Requirements{}); err != nil {
		return err
	}
	if err := p.files.checkInputs(); err != nil {
		return management.ErrPlanStale
	}
	live, err := p.backend.app.Installer.Inspect(ctx)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(live.Recovery, p.observed.Recovery) {
		return management.ErrPlanStale
	}
	return nil
}
func (p *managementRecovery) Apply(ctx context.Context, progress func(management.Progress)) (management.Outcome, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	unlock, err := (processlock.Lock{Path: p.backend.app.EnvironmentLockPath}).Acquire(ctx)
	if err != nil {
		return management.Outcome{}, err
	}
	defer func() { _ = unlock() }()
	if err = p.check(ctx); err != nil {
		return management.Outcome{}, err
	}
	p.closed = true
	if progress != nil {
		progress(management.Progress{Phase: "recovering", Message: "Reconciling observed installer receipts", Total: len(p.OperationIDs())})
	}
	native, err := p.backend.app.Installer.RecoverEnvironment(ctx, p.observed, environment.EnvironmentUserDefault)
	out := p.recoveryOutcome(ctx, native, err)
	return out, err
}
func (p *managementRecovery) recoveryOutcome(ctx context.Context, native installer.Result, applyErr error) management.Outcome {
	result := management.OperationResult{Outcome: string(native.Outcome), Mutated: native.Mutated, Items: []management.OperationItem{}, NextActions: []management.NextAction{}, Message: "Installer recovery observation completed", DataRetained: native.DataRetained}
	for _, next := range native.NextActions {
		result.NextActions = append(result.NextActions, managementNativeNextAction(next.Kind, "", ""))
	}
	after, inspectErr := p.backend.app.Installer.Inspect(ctx)
	result.Mutated = result.Mutated || inspectErr == nil && !reflect.DeepEqual(after.Recovery, p.observed.Recovery)
	out := management.Outcome{State: management.Succeeded, Result: result, Diagnostics: []management.Diagnostic{}}
	if applyErr != nil || native.Outcome != installer.OutcomeCompleted && native.Outcome != installer.OutcomeUnchanged {
		out.State = management.Failed
		if result.Mutated {
			out.State = management.Partial
		}
		if !result.Mutated && errors.Is(applyErr, context.Canceled) {
			out.State = management.Canceled
		}
		out.Result.Message = "Recovery stopped; inspect current receipts before continuing"
		out.Result.NextActions = append(out.Result.NextActions, management.NextAction{Kind: "recover", Reason: "Prepare a fresh review of the remaining installer recovery"})
		return out
	}
	return p.backend.proveManagementRecoveryLocked(ctx, p.backend.app, out)
}
