package installer

import (
	"context"
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
		for _, target := range prepared.plan.Targets {
			result.Targets = append(result.Targets, ClientResult{
				ClientID: target.ClientID, BindingID: target.BindingID, TreeDigest: target.TreeDigest,
			})
		}
		return result, nil
	}
	got, err := e.mutateGroup(ctx, prepared)
	if err != nil && got == nil {
		return Result{Operation: prepared.req.Operation, Outcome: OutcomeIncomplete, Reason: err.Error()}, err
	}
	return e.finishGroup(prepared, result, pending, *got, err)
}

func (e *Engine) finishGroup(prepared *PreparedOperation, result Result, pending bool, got usecase.GroupResult, err error) (Result, error) {
	err = wrapLifecycleError(err)
	result.InstallationID = firstNonEmpty(got.InstallationID, prepared.plan.InstallationID)
	state, loadErr := e.store.Load()
	if loadErr != nil && err == nil && !got.Mutated && !prepared.plan.NoChange {
		result.Outcome = OutcomeIncomplete
		result.Reason = "committed state is unknown"
		return result, loadErr
	}
	if loadErr == nil {
		if installation, ok := findInstall(state, result.InstallationID); ok {
			e.attachLiveGroupResult(&result, prepared, installation)
		}
	}
	if err != nil {
		result.Outcome = OutcomeIncomplete
		result.Reason = err.Error()
		if strings.Contains(err.Error(), "run update separately") {
			result.Outcome = OutcomeConflict
			result.Reason = "update_required"
		}
		return result, err
	}
	if pending && e.anyGroupHostHandoffPending(prepared) {
		result.Outcome = OutcomeIncomplete
		result.Reason = "committed binding is not activated"
		return result, fmt.Errorf("%s", result.Reason)
	}
	if !pending && (prepared.plan.NoChange || (!got.Mutated && groupPreviewUnchanged(got))) {
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

func (e *Engine) mutateGroup(ctx context.Context, prepared *PreparedOperation) (*usecase.GroupResult, error) {
	if _, err := e.helper(); err != nil {
		return nil, err
	}
	if err := e.ensureDirs(); err != nil {
		return nil, err
	}
	helper, err := e.helper()
	if err != nil {
		return nil, err
	}
	svc := e.lifecycle(helper, prepared.facts, prepared.detected)
	envelopes := prepared.envelopes
	if len(envelopes) == 0 {
		envelopes = make([]domain.PackageEnvelope, len(prepared.req.Targets))
		for i := range envelopes {
			envelopes[i] = prepared.envelope
		}
	}
	inputs, _, err := e.groupAddInputs(prepared.req, envelopes, false, true)
	if err != nil {
		return nil, err
	}
	e.report(ProgressStage)
	group := usecase.GroupInput{
		Targets: inputs, CompatibilityChecks: inputs,
		OperationGroupID: firstNonEmpty(prepared.req.OperationID, "group"),
		Confirmed:        true, Repair: prepared.req.Operation == OpRepair,
	}
	var got usecase.GroupResult
	switch prepared.req.Operation {
	case OpUpdate:
		got, err = svc.UpdateGroup(ctx, group)
	case OpRepair:
		got, err = svc.RepairGroup(ctx, group)
	default:
		got, err = svc.AddGroup(ctx, group)
	}
	return &got, err
}
