package installer

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func (e *Engine) prepareRemoveGroup(ctx context.Context, req Request) (*PreparedOperation, error) {
	detected, err := e.detectedClients(req)
	if err != nil {
		return nil, err
	}
	detected = physicalDetected(req, detected)
	selector := firstNonEmpty(req.Selector, req.InstallationID)
	if selector == "" {
		return nil, fmt.Errorf("%w: remove requires InstallationID or Selector", ErrInvalidRequest)
	}
	state, err := e.store.Load()
	if err != nil {
		return nil, err
	}
	installation, ok := findInstall(state, selector)
	if !ok {
		return nil, fmt.Errorf("%w: installation %s is not installed", ErrInvalidRequest, selector)
	}
	handle := &PreparedOperation{engine: e, req: req, detected: detected, recorded: state}
	helperVersion, helperDigest := e.helperIdentity()
	handle.plan = Plan{
		Operation: OpRemove, InstallationID: installation.InstallationID,
		HelperVersion: helperVersion, HelperDigest: helperDigest,
	}
	handle, err = e.planRemoveGroup(ctx, handle, installation)
	if err != nil {
		return nil, err
	}
	e.report(ProgressPrepare)
	e.report(ProgressPreflight)
	return handle, nil
}

func (e *Engine) applyRemoveGroup(ctx context.Context, prepared *PreparedOperation) (Result, error) {
	result := Result{Operation: OpRemove, InstallationID: prepared.plan.InstallationID, Binding: prepared.facts, Client: removalClientResult(prepared.facts)}
	live, targets := removeGroupInputs(prepared)
	result.Targets = targets
	if len(live) == 0 {
		result.Outcome = OutcomeUnchanged
		result.Reason = "already_absent"
		result.NoChange = true
		result.DataRetained = true
		return result, nil
	}
	if err := e.preflightRemoveGroup(ctx, prepared); err != nil {
		result.Outcome = OutcomeIncomplete
		result.Reason = err.Error()
		return result, err
	}
	helper, _ := e.helper()
	svc := e.removalLifecycle(ctx, prepared, e.lifecycle(helper, prepared.facts, prepared.detected))
	e.report(ProgressStage)
	if err := e.confirmRemoveBindings(ctx, prepared); err != nil {
		return removalConflict(result, err)
	}
	got, err := svc.RemoveGroup(ctx, usecase.RemoveGroupInput{
		Selector: prepared.plan.InstallationID, Targets: live,
		OperationGroupID: firstNonEmpty(prepared.req.OperationID, "remove-group"), Confirmed: true,
	})
	result.InstallationID = firstNonEmpty(got.InstallationID, prepared.plan.InstallationID)
	result.Mutated = got.Mutated
	if err != nil {
		result.Outcome = OutcomeIncomplete
		result.Reason = err.Error()
		return result, wrapLifecycleError(err)
	}
	if !got.Mutated {
		result.Outcome = OutcomeUnchanged
		result.NoChange = true
	} else {
		result.Outcome = OutcomeCompleted
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
	return result, nil
}
