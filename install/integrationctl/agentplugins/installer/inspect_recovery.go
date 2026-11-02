package installer

import (
	"context"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// Recover finishes already recorded UAP transactions for the observed scope.
// It reconciles only persisted selected native intents, without rediscovery. A new pending
// operation that was not in observed returns ErrPlanChanged without recovery.
func (e *Engine) Recover(ctx context.Context, observed Inspection) (Result, error) {
	return e.recover(ctx, observed, "")
}

// RecoverEnvironment admits only a complete pending scope owned by this environment.
func (e *Engine) RecoverEnvironment(ctx context.Context, observed Inspection, environmentID string) (Result, error) {
	if environmentID == "" {
		return Result{}, ErrInvalidRequest
	}
	return e.recover(ctx, observed, environmentID)
}

func (e *Engine) recover(ctx context.Context, observed Inspection, environmentID string) (Result, error) {
	if ctx == nil {
		return Result{Outcome: OutcomeIncomplete, Reason: "context is required"}, fmt.Errorf("%w: context is required", ErrInvalidRequest)
	}
	if observed.StateRoot != e.cfg.StateRoot {
		return Result{Outcome: OutcomeConflict, Reason: "plan_changed"}, ErrPlanChanged
	}
	initial, err := e.observe()
	if err != nil {
		return Result{Outcome: OutcomeRecovery, Reason: err.Error(), Recovery: classifyRecovery(observed.Recovery, initial.Recovery, false, err)}, fmt.Errorf("%w: %w", ErrRecoveryRequired, err)
	}
	if !liveWithinObserved(initial.Recovery, observed.Recovery) {
		return Result{Outcome: OutcomeConflict, Reason: "plan_changed"}, ErrPlanChanged
	}
	if err := e.checkEnvironmentRecovery(initial, environmentID); err != nil {
		return Result{Outcome: OutcomeRecovery, Reason: err.Error()}, err
	}
	if result, err := e.prepareRecovery(ctx); err != nil {
		return result, err
	}
	svc := e.lifecycle(nil, BindingFacts{}, nil)
	release, err := svc.Lock.Acquire(ctx)
	if err != nil {
		return Result{Outcome: OutcomeIncomplete, Reason: err.Error()}, err
	}
	defer func() { _ = release() }()
	if observed.StateRoot != "" && observed.StateRoot != e.cfg.StateRoot {
		return Result{Outcome: OutcomeConflict, Reason: "plan_changed"}, ErrPlanChanged
	}
	live, err := e.observe()
	if err != nil {
		result := Result{Outcome: OutcomeRecovery, Reason: live.Recovery.Reason, Recovery: classifyRecovery(observed.Recovery, live.Recovery, false, err)}
		if result.Reason == "" {
			result.Reason = err.Error()
		}
		return result, fmt.Errorf("%w: %w", ErrRecoveryRequired, err)
	}
	if live.Recovery.Reason != "" {
		return Result{Outcome: OutcomeRecovery, Reason: live.Recovery.Reason, Recovery: classifyRecovery(observed.Recovery, live.Recovery, true, ErrRecoveryRequired)}, ErrRecoveryRequired
	}
	if !liveWithinObserved(live.Recovery, observed.Recovery) {
		return Result{Outcome: OutcomeConflict, Reason: "plan_changed", Recovery: classifyRecovery(observed.Recovery, live.Recovery, true, ErrPlanChanged)}, ErrPlanChanged
	}
	if err := e.checkEnvironmentRecovery(live, environmentID); err != nil {
		return Result{Outcome: OutcomeRecovery, Reason: err.Error()}, err
	}
	return e.applyObservedRecovery(ctx, svc, observed, live)
}

func (e *Engine) finishRecovery(observed Inspection) (Result, error) {
	after, afterErr := e.observe()
	if afterErr != nil || after.Recovery.Required {
		reason := after.Recovery.Reason
		if afterErr != nil && reason == "" {
			reason = afterErr.Error()
		}
		if reason == "" {
			reason = "pending transactions remain"
		}
		return Result{Outcome: OutcomeRecovery, Reason: reason, Recovery: classifyRecovery(observed.Recovery, after.Recovery, afterErr == nil, fmt.Errorf("%s", reason))}, fmt.Errorf("%w: %s", ErrRecoveryRequired, reason)
	}
	return Result{Outcome: OutcomeCompleted, Recovery: classifyRecovery(observed.Recovery, after.Recovery, true, nil)}, nil
}

// RecoverCurrent inspects this root and recovers that exact live scope.
func (e *Engine) RecoverCurrent(ctx context.Context) (Result, error) {
	view, err := e.Inspect(ctx)
	if err != nil {
		reason := view.Recovery.Reason
		if reason == "" {
			reason = err.Error()
		}
		return Result{Outcome: OutcomeRecovery, Reason: reason, Recovery: classifyRecovery(view.Recovery, view.Recovery, false, err)}, err
	}
	return e.Recover(ctx, view)
}

func classifyRecovery(before, after RecoveryObservation, observedAfter bool, recoverErr error) RecoveryReport {
	afterKeys := map[string]bool{}
	if observedAfter {
		for _, key := range pendingIdentity(after) {
			afterKeys[key] = true
		}
	}
	report := RecoveryReport{}
	place := func(item PendingReceipt, key string) {
		switch {
		case !observedAfter:
			report.Unknown = append(report.Unknown, item)
		case afterKeys[key]:
			report.Remaining = append(report.Remaining, item)
		case recoverErr != nil:
			report.Unknown = append(report.Unknown, item)
		default:
			report.Resolved = append(report.Resolved, item)
		}
	}
	for _, journal := range before.Journals {
		place(PendingReceipt{
			OperationID: journal.OperationID, BindingID: journal.BindingID,
			InstallationID: journal.InstallationID, TargetPath: journal.TargetPath,
			Phase: journal.Phase, JournalPresent: true,
		}, "journal|"+journal.OperationID+"|"+journal.Phase+"|"+journal.Digest)
	}
	for _, receipt := range before.Receipts {
		place(receipt, "receipt|"+receipt.OperationID+"|"+receipt.Phase+"|"+receipt.BindingID)
	}
	for _, pending := range before.NativeIntents {
		b := pending.Binding
		place(PendingReceipt{OperationID: pending.Intent.AttemptID, InstallationID: b.InstallationID, BindingID: b.BindingID, TargetPath: b.TargetPath, Phase: "native_" + string(pending.Intent.Direction)}, nativeObservationIdentity(pending))
	}
	return report
}

func recordedBindingDigest(binding domain.ClientBinding, fallback string) string {
	if binding.PackageRevision != nil && binding.PackageRevision.TreeDigest != "" {
		return binding.PackageRevision.TreeDigest
	}
	return fallback
}

func planClientDigest(plan Plan, clientID string) string {
	for _, target := range plan.Targets {
		if target.ClientID == clientID && target.TreeDigest != "" {
			return target.TreeDigest
		}
	}
	return plan.TreeDigest
}

func liveClientResult(binding domain.ClientBinding, required []string, fallbackDigest string) ClientResult {
	return ClientResult{ProfileAuthority: domain.CloneProfileAuthority(binding.ProfileAuthority),
		SelectedDelivery:   binding.SelectedDelivery,
		ClientID:           binding.ClientID,
		BindingID:          binding.ClientBindingID,
		TreeDigest:         recordedBindingDigest(binding, fallbackDigest),
		Materialization:    string(binding.Materialization),
		Activation:         string(binding.Activation),
		Authentication:     string(binding.Authentication),
		Policy:             string(binding.Policy),
		Verification:       string(binding.Verification),
		RequiredComponents: append([]string(nil), required...),
	}
}

func (e *Engine) prepareRecovery(ctx context.Context) (Result, error) {
	if err := e.lifecycle(nil, BindingFacts{}, nil).Kernel.PrevalidateRecovery(ctx); err != nil {
		return Result{Outcome: OutcomeRecovery, Reason: err.Error()}, err
	}
	if err := e.ensureDirs(); err != nil {
		return Result{Outcome: OutcomeIncomplete, Reason: err.Error()}, err
	}
	return Result{}, nil
}

func (e *Engine) checkEnvironmentRecovery(observed Inspection, environmentID string) error {
	state, err := e.store.Load()
	if err != nil {
		return err
	}
	var authority *domain.EnvironmentAuthority
	if environmentID != "" {
		authority = &domain.EnvironmentAuthority{EnvironmentID: environmentID}
	}
	if err := transaction.ValidateEnvironmentRecoveryScope(state, authority); err != nil {
		return fmt.Errorf("%w: %w", ErrRecoveryRequired, err)
	}
	for _, j := range observed.Recovery.Journals {
		owner := j.EnvironmentID
		if owner != environmentID {
			return fmt.Errorf("%w: foreign or indeterminate journal %s", ErrRecoveryRequired, j.OperationID)
		}
	}
	return nil
}

func (e *Engine) applyObservedRecovery(ctx context.Context, svc usecase.Service, observed, live Inspection) (Result, error) {
	if !live.Recovery.Required {
		if observed.Recovery.Required && leftoverSwapArtifacts(observed.Recovery) {
			return Result{
				Outcome: OutcomeRecovery, Reason: "incomplete_recovery",
				Recovery: classifyRecovery(observed.Recovery, live.Recovery, true, ErrRecoveryRequired),
			}, fmt.Errorf("%w: journal vanished with leftover swap artifacts", ErrRecoveryRequired)
		}
		return Result{Outcome: OutcomeUnchanged, Reason: "already_recovered", Recovery: classifyRecovery(observed.Recovery, live.Recovery, true, nil)}, nil
	}
	if err := e.recoverNativeIntents(ctx, svc, live.Recovery); err != nil {
		after, afterErr := e.observe()
		result := Result{Outcome: OutcomeRecovery, Reason: err.Error(), Recovery: classifyRecovery(observed.Recovery, after.Recovery, afterErr == nil, err)}
		if errors.Is(err, ErrPlanChanged) {
			result.Outcome, result.Reason = OutcomeConflict, "plan_changed"
		}
		return result, fmt.Errorf("%w: %w", ErrRecoveryRequired, err)
	}
	return e.finishRecovery(observed)
}

func populateInspectedBinding(inspected *InspectedBinding, binding domain.ClientBinding, owners []domain.EnvironmentReceipt, installationID string) {
	if binding.PackageRevision != nil {
		inspected.Revision = binding.PackageRevision.ResolvedRevision
	}
	for _, object := range binding.NativeObjects {
		if object.Kind == "managed_package_directory" {
			inspected.ProjectionDigest = object.ManagedDigest
		}
	}
	for _, owner := range owners {
		if owner.InstallationID == installationID && owner.ClientBindingID == binding.ClientBindingID {
			cloned := owner
			inspected.Owner = &cloned
		}
	}
}

func (e *Engine) appendPendingTransitions(out *Inspection) error {
	pending, err := (opencode.NativeTransitions{ProjectRoot: e.cfg.ProjectRoot, State: transaction.Kernel{StateStore: e.store}}).Pending()
	if err != nil {
		return err
	}
	for _, p := range pending {
		if p.Root == "" {
			out.Recovery.Receipts = append(out.Recovery.Receipts, PendingReceipt{OperationID: p.OperationID, InstallationID: p.InstallationID, BindingID: p.BindingID, TargetPath: p.TargetPath, Phase: p.Phase})
		} else {
			out.Recovery.Journals = append(out.Recovery.Journals, PendingJournal{OperationID: p.OperationID, InstallationID: p.InstallationID, BindingID: p.BindingID, TargetPath: p.TargetPath, Phase: p.Phase, Digest: p.Digest})
		}
	}
	return nil
}
