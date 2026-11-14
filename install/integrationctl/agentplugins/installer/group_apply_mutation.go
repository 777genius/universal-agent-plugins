package installer

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func (e *Engine) mutateGroup(ctx context.Context, prepared *PreparedOperation) (*usecase.GroupResult, error) {
	if _, err := e.preparedHelper(prepared); err != nil {
		return nil, err
	}
	if err := e.checkPreparedProfiles(ctx, prepared); err != nil {
		return nil, err
	}
	if err := e.ensureDirs(); err != nil {
		return nil, err
	}
	helper, err := e.preparedHelper(prepared)
	if err != nil {
		return nil, err
	}
	svc := confirmationLifecycle(prepared, e.lifecycle(helper, prepared.facts, prepared.detected), false)
	envelopes := prepared.envelopes
	if len(envelopes) == 0 {
		envelopes = make([]domain.PackageEnvelope, len(prepared.req.Targets))
		for i := range envelopes {
			envelopes[i] = prepared.envelope
		}
	}
	req := prepared.req
	req.InstallationID = firstNonEmpty(req.InstallationID, prepared.plan.InstallationID)
	inputs, _, err := e.groupAddInputs(req, envelopes, false, true)
	if err != nil {
		return nil, err
	}
	for i := range inputs {
		inputs[i].Client = physicalClient(prepared.req, inputs[i].Client)
	}
	e.report(ProgressStage)
	// Recheck every target after host observers, before the group can lock or
	// discover the first native identity. Keep each actual replan guarded too.
	if err := e.confirmDeliveries(ctx, prepared); err != nil {
		return nil, err
	}
	checks, err := e.groupCompatibilityChecks(req, inputs)
	if err != nil {
		return nil, err
	}
	group := usecase.GroupInput{
		Targets: inputs, CompatibilityChecks: checks,
		OperationGroupID: firstNonEmpty(prepared.req.OperationID, "group"),
		Confirmed:        true, Repair: prepared.req.Operation == OpRepair,
		Progress: e.reportGroupActivation,
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

func (e *Engine) reportGroupActivation(event usecase.GroupProgressEvent) {
	if (event.Phase != usecase.GroupProgressActivated && event.Phase != usecase.GroupProgressActivating) || e.cfg.Progress == nil {
		return
	}
	item := ClientResult{ClientID: string(event.ClientID), GroupPhase: string(event.Result.GroupPhase), Activation: string(event.Result.Activation.Activation)}
	if event.Phase == usecase.GroupProgressActivating {
		item.GroupPhase = "external_started"
	}
	if event.Result.Failure != nil {
		item.Failure = &ClientFailure{Stage: event.Result.Failure.Stage}
	}
	e.cfg.Progress(ProgressEvent{Phase: ProgressActivate, Client: &item})
}
