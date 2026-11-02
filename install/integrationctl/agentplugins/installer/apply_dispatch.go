package installer

import (
	"context"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// Apply executes a prepared operation. A canceled decision returns before usecase
// mutation, recovery, and callbacks. Result is populated even when err != nil.
// Close during Apply returns ErrHandleBusy without releasing the snapshot.
func (e *Engine) Apply(ctx context.Context, prepared *PreparedOperation, decision Decision) (result Result, err error) {
	if prepared == nil || prepared.engine != e {
		return Result{}, ErrInvalidHandle
	}
	op, err := prepared.beginApply(decision)
	if err != nil {
		if errors.Is(err, ErrCancelled) {
			return Result{Operation: op, Outcome: OutcomeCancelled, Reason: "host canceled"}, err
		}
		return Result{}, err
	}
	defer func() {
		if result.Binding.ClientID != "" {
			result.Binding.TargetContext = preparedTargetContext(prepared, result.Binding.ClientID)
		}
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
	// Preserve the prepared root-identity refusal before the stored journal
	// scanner touches a root that may have been replaced by a symlink.
	if err := checkOpenCodeHostRoots(prepared); err != nil {
		result = Result{Operation: op, Outcome: OutcomeConflict, Reason: "plan_changed"}
		attachNextActions(&result)
		return result, err
	}
	if err := e.checkPreparedProfiles(ctx, prepared); err != nil {
		return Result{Operation: op, Outcome: OutcomeConflict, Reason: "profile_changed"}, err
	}
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
	if err := e.revalidateOpenCodeHost(ctx, prepared); err != nil {
		result = Result{Operation: op, Outcome: OutcomeConflict, Reason: "plan_changed"}
		attachNextActions(&result)
		return result, err
	}
	return e.prepareApplyResources(ctx, prepared, op)
}

func (e *Engine) prepareApplyResources(ctx context.Context, prepared *PreparedOperation, op Operation) (Result, error) {
	if prepared.plan.NoChange {
		return Result{}, nil
	}
	var result Result
	if op == OpInstall || op == OpUpdate || op == OpRepair || op == OpRefreshProjection {
		if _, err := e.preparedHelper(prepared); err != nil {
			result = Result{Operation: op, Outcome: OutcomeIncomplete, Reason: err.Error()}
			attachNextActions(&result)
			return result, err
		}
	}
	if err := e.checkPreparedProfiles(ctx, prepared); err != nil {
		return result, err
	}
	if err := e.ensureDirs(); err != nil {
		result = Result{Operation: op, Outcome: OutcomeIncomplete, Reason: err.Error()}
		attachNextActions(&result)
		return result, err
	}
	return Result{}, nil
}

func (e *Engine) applyPreparedOperation(ctx context.Context, prepared *PreparedOperation, op Operation) (Result, error) {
	var result Result
	var err error
	e.report(ProgressPreflight)
	if err := e.confirmDeliveries(ctx, prepared); err != nil {
		result = Result{Operation: op, Outcome: OutcomeConflict, Reason: "plan_changed"}
		attachNextActions(&result)
		return result, err
	}
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
		case OpRefreshProjection:
			result, err = e.applyRefreshProjection(ctx, prepared)
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

func (e *Engine) applyRefreshProjection(ctx context.Context, prepared *PreparedOperation) (Result, error) {
	return e.applyMutatingPackage(ctx, prepared, func(svc usecase.Service, in usecase.AddInput) (usecase.AddResult, error) {
		return svc.RefreshProjection(ctx, in)
	})
}
