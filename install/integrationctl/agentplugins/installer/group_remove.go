package installer

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func (e *Engine) prepareRemoveGroup(ctx context.Context, req Request) (*PreparedOperation, error) {
	detected, err := e.detectedClients(req)
	if err != nil {
		return nil, err
	}
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
	e.report(ProgressPrepare)
	e.report(ProgressPreflight)
	handle := &PreparedOperation{engine: e, req: req, detected: detected}
	helperVersion, helperDigest := e.helperIdentity()
	handle.plan = Plan{
		Operation: OpRemove, InstallationID: installation.InstallationID,
		HelperVersion: helperVersion, HelperDigest: helperDigest,
	}
	return e.planRemoveGroup(ctx, handle, installation)
}

func (e *Engine) applyRemoveGroup(ctx context.Context, prepared *PreparedOperation) (Result, error) {
	result := Result{Operation: OpRemove, InstallationID: prepared.plan.InstallationID, Binding: prepared.facts}
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
	svc := e.lifecycle(helper, prepared.facts, prepared.detected)
	e.report(ProgressStage)
	got, err := svc.RemoveGroup(ctx, usecase.RemoveGroupInput{
		Selector: prepared.plan.InstallationID, Targets: live,
		OperationGroupID: firstNonEmpty(prepared.req.OperationID, "remove-group"), Confirmed: true,
	})
	result.InstallationID = firstNonEmpty(got.InstallationID, prepared.plan.InstallationID)
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

func (e *Engine) planRemoveGroup(ctx context.Context, handle *PreparedOperation, installation domain.Installation) (*PreparedOperation, error) {
	req := handle.req
	var live int
	for _, target := range req.Targets {
		client, err := e.detectedClient(Request{
			Operation: OpRemove, ClientID: target.ClientID,
			ClientConfigRoot: target.ClientConfigRoot, ClientExecutable: firstNonEmpty(target.ClientExecutable, req.ClientExecutable),
		})
		if err != nil {
			return nil, err
		}
		handle.clients = append(handle.clients, client)
		binding, receipt, found := findBinding(installation, client.ClientID)
		item := PlanTarget{ClientID: string(client.ClientID), ConfigRoot: client.ConfigRoot, NoChange: !found}
		if found {
			if err := e.removalPreflight(ctx, client, binding, receipt); err != nil {
				return nil, err
			}
			item.TargetPath = binding.TargetLocator
			item.BindingID = binding.ClientBindingID
			live++
			if handle.plan.TargetPath == "" {
				setRemoveGroupPrimary(handle, client, item, binding, receipt)
			}
		}
		handle.plan.Targets = append(handle.plan.Targets, item)
	}
	if live == 0 {
		handle.plan.NoChange = true
	}
	return handle, nil
}

func setRemoveGroupPrimary(handle *PreparedOperation, client domain.DetectedClient, item PlanTarget, binding domain.ClientBinding, receipt domain.DataReceipt) {
	handle.plan.ClientID = item.ClientID
	handle.plan.ConfigRoot = item.ConfigRoot
	handle.plan.TargetPath = item.TargetPath
	handle.plan.BindingID = item.BindingID
	handle.client = client
	handle.artifact = binding.PhysicalArtifact
	handle.facts = BindingFacts{
		InstallationID: handle.plan.InstallationID, ClientID: item.ClientID,
		BindingID: item.BindingID, Scope: binding.Scope, TargetPath: item.TargetPath,
		DataRoot: receipt.Locator, DataReceiptID: binding.DataReceiptID,
		OperationID: handle.req.OperationID,
	}
}

func removeGroupInputs(prepared *PreparedOperation) ([]usecase.RemoveInput, []ClientResult) {
	var targets []ClientResult
	var live []usecase.RemoveInput
	for i, target := range prepared.plan.Targets {
		item := ClientResult{ClientID: target.ClientID, BindingID: target.BindingID}
		if target.NoChange {
			item.Materialization = string(domain.MaterializationAbsent)
			targets = append(targets, item)
			continue
		}
		client := prepared.clients[i]
		live = append(live, usecase.RemoveInput{
			Selector: prepared.plan.InstallationID, Client: client, Scope: domain.ScopeUser,
			Confirmed: true, OperationID: prepared.req.OperationID, BackendExecutable: client.ExecutablePath,
			ExternalUninstalled: prepared.req.Targets[i].ExternalUninstalled || prepared.req.ExternalUninstalled,
		})
		targets = append(targets, item)
	}
	return live, targets
}

func (e *Engine) preflightRemoveGroup(ctx context.Context, prepared *PreparedOperation) error {
	state, err := e.store.Load()
	if err != nil {
		return err
	}
	installation, ok := findInstall(state, prepared.plan.InstallationID)
	if !ok {
		err := fmt.Errorf("%w: installation %s is not installed", ErrInvalidRequest, prepared.plan.InstallationID)
		return err
	}
	for i, target := range prepared.plan.Targets {
		if target.NoChange {
			continue
		}
		client := prepared.clients[i]
		binding, receipt, found := findBinding(installation, client.ClientID)
		if !found {
			err := fmt.Errorf("%w: client %s is not installed", ErrInvalidRequest, client.ClientID)
			return err
		}
		if err := e.removalPreflight(ctx, client, binding, receipt); err != nil {
			return err
		}
	}
	return nil
}
