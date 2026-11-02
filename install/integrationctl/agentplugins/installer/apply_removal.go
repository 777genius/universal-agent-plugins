package installer

import (
	"context"
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

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
	if err := e.removalPreflight(ctx, prepared.client, installation.DeclaredName, binding, receipt); err != nil {
		return Result{Operation: OpRemove, Outcome: OutcomeIncomplete, Reason: err.Error()}, err
	}
	helper, _ := e.helper()
	svc := e.removalLifecycle(ctx, prepared, e.lifecycle(helper, prepared.facts, prepared.detected))
	e.report(ProgressStage)
	if err := e.confirmRemoveBindings(ctx, prepared); err != nil {
		return removalConflict(Result{Operation: OpRemove, Binding: prepared.facts}, err)
	}
	removed, err := svc.Remove(ctx, usecase.RemoveInput{
		Selector: prepared.plan.InstallationID, Client: prepared.client, Scope: e.scope(),
		Confirmed: true, OperationID: prepared.req.OperationID, BackendExecutable: prepared.req.ClientExecutable,
		ExternalUninstalled: prepared.req.ExternalUninstalled, SelectedDelivery: prepared.plan.SelectedDelivery,
		EnvironmentAuthority: prepared.req.EnvironmentAuthority,
	})
	result := Result{Operation: OpRemove, InstallationID: removed.InstallationID, Binding: prepared.facts, Mutated: removed.Mutated, Client: removalClientResult(prepared.facts)}
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
		e.inspectRemovalOutcome(&result, prepared.plan.BindingID)
	} else {
		result.Outcome = OutcomeIncomplete
		result.Reason = err.Error()
	}
	return result, err
}

func (e *Engine) liveBinding(prepared *PreparedOperation) (Result, domain.ClientBinding, bool) {
	result := Result{Operation: prepared.req.Operation, Binding: prepared.facts}
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
	result.Binding = BindingFacts{ProfileAuthority: domain.CloneProfileAuthority(binding.ProfileAuthority),
		InstallationID: result.InstallationID, ClientID: string(prepared.client.ClientID),
		SelectedDelivery: binding.SelectedDelivery,
		BindingID:        binding.ClientBindingID, Scope: binding.Scope, TargetPath: binding.TargetLocator,
		DataRoot: receipt.Locator, DataReceiptID: binding.DataReceiptID,
		OperationID: prepared.req.OperationID, TreeDigest: recordedBindingDigest(binding, prepared.plan.TreeDigest),
	}
	result.Client = liveClientResult(binding, prepared.req.RequiredComponents, prepared.plan.TreeDigest)
	return result, binding, true
}
