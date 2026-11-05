package installer

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func (e *Engine) applyGroup(ctx context.Context, prepared *PreparedOperation) (Result, error) {
	if prepared.req.Operation == OpRemove {
		return e.applyRemoveGroup(ctx, prepared)
	}
	result := Result{
		Operation: prepared.req.Operation, InstallationID: prepared.plan.InstallationID,
		Binding: prepared.facts,
	}
	pending, err := e.reconcileGroupHostHandoff(ctx, prepared, &result)
	if err != nil {
		result.Outcome = OutcomeIncomplete
		result.Reason = err.Error()
		e.report(ProgressCommit)
		return result, err
	}
	if prepared.plan.NoChange && !pending {
		result.Outcome = OutcomeUnchanged
		result.NoChange = true
		state, err := e.store.Load()
		if err != nil {
			return result, err
		}
		if installation, ok := findInstall(state, prepared.plan.InstallationID); ok {
			e.attachLiveGroupResult(&result, prepared, installation)
		}
		return result, nil
	}
	got, err := e.mutateGroup(ctx, prepared)
	if err != nil && got == nil {
		outcome, reason := OutcomeIncomplete, err.Error()
		if errors.Is(err, ErrPlanChanged) {
			outcome, reason = OutcomeConflict, "plan_changed"
		}
		return Result{Operation: prepared.req.Operation, Outcome: outcome, Reason: reason}, err
	}
	return e.finishGroup(prepared, result, pending, *got, err)
}

func (e *Engine) finishGroup(prepared *PreparedOperation, result Result, pending bool, got usecase.GroupResult, err error) (Result, error) {
	result.Mutated = result.Mutated || got.Mutated
	err = wrapLifecycleError(err)
	result.InstallationID = firstNonEmpty(got.InstallationID, prepared.plan.InstallationID)
	state, loadErr := e.store.Load()
	if prepared.req.Scope != domain.ScopeProject && loadErr != nil && err == nil && !got.Mutated && !prepared.plan.NoChange {
		result.Outcome = OutcomeIncomplete
		result.Reason = "committed state is unknown"
		return result, loadErr
	}
	if loadErr == nil {
		if installation, ok := findInstall(state, result.InstallationID); ok {
			e.attachLiveGroupResult(&result, prepared, installation)
		}
	}
	attachGroupAttemptResult(&result, got)
	if err != nil {
		return failedGroupResult(result, err)
	}
	if prepared.req.Scope == domain.ScopeProject {
		if loadErr != nil {
			result.Outcome, result.Reason = OutcomeIncomplete, "committed project state is unknown"
			return result, loadErr
		}
		if completeErr := completedProjectGroup(prepared, got, state); completeErr != nil {
			result.Outcome, result.Reason = OutcomeIncomplete, completeErr.Error()
			return result, completeErr
		}
		pending = false
	}
	if pending && loadErr != nil {
		result.Outcome, result.Reason = OutcomeIncomplete, "committed group state is unknown"
		return result, loadErr
	}
	if pending && e.anyGroupHostHandoffPending(prepared) {
		if e.cfg.OnCommittedBinding == nil || !completedUserPreparedUpdate(prepared, got, state) {
			result.Outcome = OutcomeIncomplete
			result.Reason = "committed binding is not activated"
			return result, fmt.Errorf("%s", result.Reason)
		}
		// The verified file update is complete; Local may still be Prepared.
		// Never rewrite or promote its saved activation/verification evidence.
		pending = false
	}
	if !result.Mutated && !pending && (prepared.plan.NoChange || (!got.Mutated && groupPreviewUnchanged(got))) {
		result.Outcome = OutcomeUnchanged
		result.NoChange = true
		return result, nil
	}
	result.Outcome = OutcomeCompleted
	e.report(ProgressCommit)
	e.report(ProgressActivate)
	e.report(ProgressVerify)
	e.report(ProgressComplete)
	return result, nil
}

func failedGroupResult(result Result, err error) (Result, error) {
	result.Outcome = OutcomeIncomplete
	result.Reason = err.Error()
	if errors.Is(err, ErrPlanChanged) {
		result.Outcome = OutcomeConflict
		result.Reason = "plan_changed"
	}
	if strings.Contains(err.Error(), "run update separately") || errors.Is(err, usecase.ErrProjectionRefreshRequired) {
		result.Outcome = OutcomeConflict
		result.Reason = "update_required"
	}
	if errors.Is(err, usecase.ErrProjectionPinChangeRequired) {
		result.Outcome = OutcomeConflict
		result.Reason = "pin_change_required"
	}
	return result, err
}

func attachGroupAttemptResult(result *Result, got usecase.GroupResult) {
	for _, target := range got.Targets {
		if target.Plan.Scope == domain.ScopeProject {
			result.ManualActions = append(result.ManualActions, target.Activation.UserActions...)
		}
		if target.Plan.ClientID == "" || target.GroupPhase == "" {
			continue
		}
		index := -1
		for i := range result.Targets {
			if result.Targets[i].ClientID == string(target.Plan.ClientID) {
				index = i
				break
			}
		}
		if index < 0 {
			// A failed state reload must not erase observed attempt facts. Saved
			// lifecycle dimensions remain unknown until a successful inspection.
			result.Targets = append(result.Targets, ClientResult{ClientID: string(target.Plan.ClientID)})
			index = len(result.Targets) - 1
		}
		item := &result.Targets[index]
		attachClientAttemptResult(item, target)
		if result.Client.ClientID == "" || result.Client.ClientID == item.ClientID {
			result.Client = *item
		}
	}
}

func attachClientAttemptResult(item *ClientResult, target usecase.AddResult) {
	item.GroupPhase = string(target.GroupPhase)
	item.Failure = nil
	if target.Failure != nil {
		item.Failure = &ClientFailure{Stage: target.Failure.Stage, Message: target.Failure.Message}
	}
}
