package installer

import (
	"context"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// Apply executes a prepared operation. A cancelled decision returns before usecase
// mutation, recovery, and callbacks. Result is populated even when err != nil.
// Close during Apply returns ErrHandleBusy without releasing the snapshot.
func (e *Engine) Apply(ctx context.Context, prepared *PreparedOperation, decision Decision) (result Result, err error) {
	if prepared == nil || prepared.engine != e {
		return Result{}, ErrInvalidHandle
	}
	op, err := prepared.beginApply(decision)
	if err != nil {
		if errors.Is(err, ErrCancelled) {
			return Result{Operation: op, Outcome: OutcomeCancelled, Reason: "host cancelled"}, err
		}
		return Result{}, err
	}
	defer func() {
		prepared.mu.Lock()
		prepared.busy = false
		if err == nil {
			prepared.applied = true
		}
		prepared.mu.Unlock()
	}()
	if result, err = e.preflightApply(ctx, prepared, op); err != nil {
		return result, err
	}
	return e.applyPreparedOperation(ctx, prepared, op)
}

func (e *Engine) preflightApply(ctx context.Context, prepared *PreparedOperation, op Operation) (Result, error) {
	var result Result
	var err error
	view, inspectErr := e.Inspect(ctx)
	if inspectErr != nil || view.Recovery.Required {
		reason := view.Recovery.Reason
		if reason == "" && inspectErr != nil {
			reason = inspectErr.Error()
		}
		if reason == "" {
			reason = "pending transactions remain"
		}
		result = Result{Operation: op, Outcome: OutcomeRecovery, Reason: reason}
		err = fmt.Errorf("%w: %s", ErrRecoveryRequired, reason)
		attachNextActions(&result)
		return result, err
	}
	if err := e.confirmPreparedPlan(ctx, prepared); err != nil {
		reason := "plan_changed"
		if errors.Is(err, ErrUpdateRequired) {
			reason = "update_required"
		}
		result = Result{Operation: op, Outcome: OutcomeConflict, Reason: reason}
		attachNextActions(&result)
		return result, err
	}
	if !prepared.plan.NoChange {
		if op == OpInstall || op == OpUpdate || op == OpRepair {
			if _, err = e.helper(); err != nil {
				result = Result{Operation: op, Outcome: OutcomeIncomplete, Reason: err.Error()}
				attachNextActions(&result)
				return result, err
			}
		}
		if err = e.ensureDirs(); err != nil {
			result = Result{Operation: op, Outcome: OutcomeIncomplete, Reason: err.Error()}
			attachNextActions(&result)
			return result, err
		}
	}
	return Result{}, nil
}

func (e *Engine) applyPreparedOperation(ctx context.Context, prepared *PreparedOperation, op Operation) (Result, error) {
	var result Result
	var err error
	e.report(ProgressPreflight)
	if len(prepared.req.Targets) > 1 {
		result, err = e.applyGroup(ctx, prepared)
	} else {
		switch op {
		case OpInstall:
			result, err = e.applyInstall(ctx, prepared)
		case OpUpdate:
			result, err = e.applyUpdate(ctx, prepared)
		case OpRepair:
			result, err = e.applyRepair(ctx, prepared)
		case OpRemove:
			result, err = e.applyRemove(ctx, prepared)
		default:
			err = fmt.Errorf("%w: %s", ErrUnsupported, op)
			return Result{}, err
		}
	}
	attachNextActions(&result)
	return result, err
}

func (e *Engine) applyInstall(ctx context.Context, prepared *PreparedOperation) (Result, error) {
	return e.applyMutatingPackage(ctx, prepared, func(svc usecase.Service, in usecase.AddInput) (usecase.AddResult, error) {
		return svc.Add(ctx, in)
	})
}

func (e *Engine) applyUpdate(ctx context.Context, prepared *PreparedOperation) (Result, error) {
	return e.applyMutatingPackage(ctx, prepared, func(svc usecase.Service, in usecase.AddInput) (usecase.AddResult, error) {
		return e.updateWithCompatibility(ctx, svc, prepared.req, in)
	})
}

func (e *Engine) applyRepair(ctx context.Context, prepared *PreparedOperation) (Result, error) {
	return e.applyMutatingPackage(ctx, prepared, func(svc usecase.Service, in usecase.AddInput) (usecase.AddResult, error) {
		return svc.Repair(ctx, in)
	})
}

func (e *Engine) applyMutatingPackage(ctx context.Context, prepared *PreparedOperation, call func(usecase.Service, usecase.AddInput) (usecase.AddResult, error)) (Result, error) {
	if committed, binding, ok := e.liveBinding(prepared); ok && hostHandoffPending(binding) && e.cfg.OnCommittedBinding != nil {
		if err := e.cfg.OnCommittedBinding(ctx, committed.Binding); err != nil {
			committed.Outcome = OutcomeIncomplete
			committed.Reason = err.Error()
			e.report(ProgressCommit)
			return committed, err
		}
	}
	helper, err := e.helper()
	if err != nil {
		return Result{Operation: prepared.req.Operation, Outcome: OutcomeIncomplete, Reason: err.Error()}, err
	}
	svc := e.lifecycle(helper, prepared.facts, prepared.detected)
	e.report(ProgressStage)
	added, err := call(svc, usecase.AddInput{
		Envelope: prepared.envelope, Client: prepared.client, Scope: domain.ScopeUser, Confirmed: true,
		InstallationID: prepared.req.InstallationID, OperationID: prepared.req.OperationID,
		BackendExecutable: prepared.req.ClientExecutable,
	})
	err = wrapLifecycleError(err)
	result := Result{Operation: prepared.req.Operation, InstallationID: added.InstallationID, Binding: prepared.facts}
	if added.Activation.UserActions != nil {
		result.ManualActions = append([]string(nil), added.Activation.UserActions...)
	}
	result, committed, loadErr := e.readCommittedPackage(prepared, added, result, err)
	if loadErr != nil {
		return result, loadErr
	}
	return e.finishMutatingPackage(result, added, committed, err)
}

func (e *Engine) readCommittedPackage(prepared *PreparedOperation, added usecase.AddResult, result Result, err error) (Result, bool, error) {
	committed := false
	if state, loadErr := e.store.Load(); loadErr == nil {
		installationID := firstNonEmpty(added.InstallationID, prepared.req.InstallationID)
		if installation, ok := findInstall(state, installationID); ok {
			if binding, receipt, ok := findBinding(installation, prepared.client.ClientID); ok {
				committed = true
				result.InstallationID = firstNonEmpty(installation.InstallationID, installationID)
				result.Binding = BindingFacts{
					InstallationID: result.InstallationID, ClientID: string(prepared.client.ClientID),
					BindingID: binding.ClientBindingID, Scope: binding.Scope, TargetPath: binding.TargetLocator,
					DataRoot: receipt.Locator, DataReceiptID: binding.DataReceiptID,
					OperationID: prepared.req.OperationID, TreeDigest: recordedBindingDigest(binding, prepared.plan.TreeDigest),
				}
				result.Client = liveClientResult(binding, prepared.req.RequiredComponents, prepared.plan.TreeDigest)
			}
		}
	} else if err == nil && !added.NoChange {
		result.Outcome = OutcomeIncomplete
		result.Reason = "committed state is unknown"
		result.Recovery.Unknown = []PendingReceipt{{
			OperationID: prepared.req.OperationID, InstallationID: added.InstallationID,
		}}
		return result, false, loadErr
	}
	return result, committed, nil
}

func (e *Engine) finishMutatingPackage(result Result, added usecase.AddResult, committed bool, err error) (Result, error) {
	if errors.Is(err, ErrUpdateRequired) {
		result.Outcome = OutcomeConflict
		result.Reason = "update_required"
		return result, err
	}
	if errors.Is(err, ErrTargetFactsUnavailable) {
		result.Outcome = OutcomeConflict
		result.Reason = "target_facts_unavailable"
		return result, err
	}
	if added.NoChange {
		result.Outcome = OutcomeUnchanged
		result.NoChange = true
	} else if err == nil {
		result.Outcome = OutcomeCompleted
		e.report(ProgressCommit)
		e.report(ProgressActivate)
		e.report(ProgressVerify)
		e.report(ProgressComplete)
	} else {
		result.Outcome = OutcomeIncomplete
		result.Reason = err.Error()
		if committed {
			e.report(ProgressCommit)
		}
	}
	return result, err
}

func (e *Engine) applyRemove(ctx context.Context, prepared *PreparedOperation) (Result, error) {
	if prepared.plan.NoChange {
		return Result{
			Operation: OpRemove, InstallationID: prepared.plan.InstallationID,
			Outcome: OutcomeUnchanged, Reason: "already_absent", NoChange: true,
			DataRetained: true, Binding: prepared.facts,
		}, nil
	}
	state, err := e.store.Load()
	if err != nil {
		return Result{Operation: OpRemove, Outcome: OutcomeIncomplete, Reason: err.Error()}, err
	}
	installation, ok := findInstall(state, prepared.plan.InstallationID)
	if !ok {
		return Result{Operation: OpRemove, Outcome: OutcomeIncomplete, Reason: "installation is not installed"}, fmt.Errorf("%w: installation %s is not installed", ErrInvalidRequest, prepared.plan.InstallationID)
	}
	binding, receipt, ok := findBinding(installation, prepared.client.ClientID)
	if !ok {
		return Result{Operation: OpRemove, Outcome: OutcomeIncomplete, Reason: "client is not installed"}, fmt.Errorf("%w: client %s is not installed", ErrInvalidRequest, prepared.client.ClientID)
	}
	if err := e.removalPreflight(ctx, prepared.client, binding, receipt); err != nil {
		return Result{Operation: OpRemove, Outcome: OutcomeIncomplete, Reason: err.Error()}, err
	}
	helper, _ := e.helper()
	svc := e.lifecycle(helper, prepared.facts, prepared.detected)
	e.report(ProgressStage)
	removed, err := svc.Remove(ctx, usecase.RemoveInput{
		Selector: prepared.plan.InstallationID, Client: prepared.client, Scope: domain.ScopeUser,
		Confirmed: true, OperationID: prepared.req.OperationID, BackendExecutable: prepared.req.ClientExecutable,
		ExternalUninstalled: prepared.req.ExternalUninstalled,
	})
	result := Result{Operation: OpRemove, InstallationID: removed.InstallationID, Binding: prepared.facts}
	if err == nil {
		result.Outcome = OutcomeCompleted
		if !removed.Mutated {
			result.Outcome = OutcomeUnchanged
			result.NoChange = true
		} else {
			e.report(ProgressCommit)
			e.report(ProgressActivate)
			e.report(ProgressVerify)
			e.report(ProgressComplete)
		}
		if view, inspectErr := e.observe(); inspectErr == nil {
			for _, installation := range view.Installations {
				if installation.InstallationID == result.InstallationID {
					result.DataRetained = installation.DataRetained
				}
			}
		}
	} else {
		result.Outcome = OutcomeIncomplete
		result.Reason = err.Error()
	}
	return result, err
}

func (e *Engine) liveBinding(prepared *PreparedOperation) (Result, domain.ClientBinding, bool) {
	result := Result{Operation: OpInstall, Binding: prepared.facts}
	state, err := e.store.Load()
	if err != nil {
		return result, domain.ClientBinding{}, false
	}
	installationID := firstNonEmpty(prepared.req.InstallationID, prepared.plan.InstallationID)
	installation, ok := findInstall(state, installationID)
	if !ok {
		return result, domain.ClientBinding{}, false
	}
	binding, receipt, ok := findBinding(installation, prepared.client.ClientID)
	if !ok {
		return result, domain.ClientBinding{}, false
	}
	result.InstallationID = firstNonEmpty(installation.InstallationID, installationID)
	result.Binding = BindingFacts{
		InstallationID: result.InstallationID, ClientID: string(prepared.client.ClientID),
		BindingID: binding.ClientBindingID, Scope: binding.Scope, TargetPath: binding.TargetLocator,
		DataRoot: receipt.Locator, DataReceiptID: binding.DataReceiptID,
		OperationID: prepared.req.OperationID, TreeDigest: recordedBindingDigest(binding, prepared.plan.TreeDigest),
	}
	result.Client = liveClientResult(binding, prepared.req.RequiredComponents, prepared.plan.TreeDigest)
	return result, binding, true
}

func (e *Engine) updateWithCompatibility(ctx context.Context, svc usecase.Service, req Request, in usecase.AddInput) (usecase.AddResult, error) {
	checks, err := e.compatibilityChecks(req, in)
	if err != nil {
		return usecase.AddResult{}, err
	}
	if len(checks) == 1 {
		return svc.Update(ctx, in)
	}
	got, err := svc.UpdateGroup(ctx, usecase.GroupInput{
		Targets:             []usecase.AddInput{in},
		CompatibilityChecks: checks,
		OperationGroupID:    firstNonEmpty(req.OperationID, "update"),
		DryRun:              in.DryRun,
		Confirmed:           in.Confirmed,
	})
	if err != nil {
		return usecase.AddResult{InstallationID: got.InstallationID}, err
	}
	for _, target := range got.Targets {
		if target.Plan.ClientID == in.Client.ClientID || target.Plan.ClientID == domain.ClientID(req.ClientID) {
			return target, nil
		}
	}
	if len(got.Targets) > 0 {
		return got.Targets[0], nil
	}
	return usecase.AddResult{InstallationID: got.InstallationID, Mutated: got.Mutated}, nil
}

func (e *Engine) compatibilityChecks(req Request, target usecase.AddInput) ([]usecase.AddInput, error) {
	if req.InstallationID == "" {
		return []usecase.AddInput{target}, nil
	}
	// A group request owns all of its explicit targets. The group use case
	// performs the cross-target reconciliation itself; do not reinterpret it
	// as a single-target update that needs host facts for the same siblings.
	if req.ClientID == "" && len(req.Targets) > 0 {
		return []usecase.AddInput{target}, nil
	}
	state, err := e.store.Load()
	if err != nil {
		return nil, fmt.Errorf("%w: load owned state: %v", ErrTargetFactsUnavailable, err)
	}
	installation, ok := findInstall(state, req.InstallationID)
	if !ok {
		return []usecase.AddInput{target}, nil
	}
	selected := make(map[string]struct{}, len(req.Targets))
	for _, candidate := range req.Targets {
		selected[candidate.ClientID] = struct{}{}
	}
	known, err := e.knownTargetIndex(req.KnownTargets)
	if err != nil {
		return nil, err
	}
	var checks []usecase.AddInput
	for _, binding := range installation.Clients {
		if binding.Materialization == domain.MaterializationAbsent {
			continue
		}
		// Group operations already carry explicit request-scoped facts for every
		// selected target. Only bindings outside that group need KnownTargets;
		// treating selected siblings as unknown would make a valid two-client
		// repair/update impossible when ClientID is intentionally empty.
		if _, ok := selected[binding.ClientID]; ok {
			continue
		}
		configRoot := req.ClientConfigRoot
		executable := req.ClientExecutable
		if binding.ClientID != req.ClientID {
			facts, ok := known[binding.ClientID]
			if !ok {
				return nil, fmt.Errorf("%w: binding %s (%s) has no host facts", ErrTargetFactsUnavailable, binding.ClientBindingID, binding.ClientID)
			}
			if facts.BindingID != binding.ClientBindingID {
				return nil, fmt.Errorf("%w: binding %s (%s) does not match host binding %s", ErrTargetFactsUnavailable, binding.ClientBindingID, binding.ClientID, facts.BindingID)
			}
			configRoot = facts.ConfigRoot
			executable = facts.Executable
		}
		client, err := e.detectedClient(Request{
			Operation: OpUpdate, ClientID: binding.ClientID, ClientConfigRoot: configRoot,
			ClientExecutable: executable,
		})
		if err != nil {
			return nil, fmt.Errorf("%w: binding %s (%s): %v", ErrTargetFactsUnavailable, binding.ClientBindingID, binding.ClientID, err)
		}
		check := target
		check.Client = client
		check.BackendExecutable = client.ExecutablePath
		checks = append(checks, check)
	}
	return checks, nil
}

func attachNextActions(result *Result) {
	if result == nil || len(result.NextActions) > 0 {
		return
	}
	switch {
	case result.Outcome == OutcomeRecovery:
		result.NextActions = []NextAction{{Kind: "recover", Reason: result.Reason}}
	case result.Reason == "update_required":
		result.NextActions = []NextAction{{Kind: "update", Reason: result.Reason}}
	case result.Reason == "plan_changed":
		result.NextActions = []NextAction{{Kind: "reprepare", Reason: result.Reason}}
	case result.Reason == "target_facts_unavailable":
		result.NextActions = []NextAction{{Kind: "reprepare", Reason: result.Reason}}
	case result.Outcome == OutcomeIncomplete && result.Client.Materialization != "" && result.Client.Materialization != string(domain.MaterializationAbsent):
		result.NextActions = []NextAction{{Kind: "activate", Reason: result.Reason}}
	}
}

func (p *PreparedOperation) beginApply(decision Decision) (Operation, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return "", ErrHandleClosed
	}
	if p.applied {
		p.mu.Unlock()
		return "", ErrAlreadyApplied
	}
	if p.busy {
		p.mu.Unlock()
		return "", ErrHandleBusy
	}
	if !decision.Confirmed {
		op := p.req.Operation
		p.mu.Unlock()
		return op, ErrCancelled
	}
	p.busy = true
	op := p.req.Operation
	p.mu.Unlock()
	return op, nil
}
