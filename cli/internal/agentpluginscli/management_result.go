package agentpluginscli

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func (p *managementPrepared) Apply(ctx context.Context, progress func(management.Progress)) (management.Outcome, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.applied {
		return management.Outcome{}, installer.ErrPlanChanged
	}
	p.applied = true
	adapter := &dependencyInstaller{app: p.app, opts: &options{externalUninstalled: p.input.ExternalUninstalled}, detected: p.request.detected, operationIDs: p.operationIDs}
	completed := 0
	adapter.onResult = func(action environment.PreparedBinding, result installer.Result) {
		completed++
		if progress != nil {
			progress(management.Progress{Phase: "applying", Message: "Observed native binding result", Completed: completed, Total: len(p.request.prepared), Items: []management.OperationItem{p.operationItem(action, result)}})
		}
	}
	if progress != nil {
		progress(management.Progress{Phase: "preflight", Message: "Rechecking retained preparation", Total: len(p.request.prepared)})
	}
	applied, err := applyPreparedDependencies(ctx, p.app, &p.request, adapter, p.check)
	out := p.outcome(applied, err)
	if err == nil && p.input.Action == management.Recover {
		out = p.backend.proveManagementRecovery(ctx, p.preview, out)
	}
	return out, err
}
func (p *managementPrepared) operationItem(action dependencyAction, result installer.Result) management.OperationItem {
	return management.OperationItem{Alias: action.Alias, TargetId: p.backend.targetID(action.ClientID), Status: string(result.Outcome), DestinationPath: p.paths[action.Alias+"\x00"+action.ClientID], Message: managementNativeMessage(result)}
}
func (p *managementPrepared) outcome(applied dependencyApplicationResult, applyErr error) management.Outcome {
	result := management.OperationResult{Outcome: "completed", Mutated: applied.Published, Items: []management.OperationItem{}, NextActions: []management.NextAction{}, Message: "Environment change completed"}
	cursor := 0
	for _, action := range p.request.prepared {
		item := management.OperationItem{Alias: action.Alias, TargetId: p.backend.targetID(action.ClientID), Status: "not_attempted", DestinationPath: p.paths[action.Alias+"\x00"+action.ClientID]}
		if action.Action == environment.ActionUnchanged {
			item.Status = "unchanged"
		} else if cursor < len(applied.Results) {
			native := applied.Results[cursor]
			cursor++
			item = p.operationItem(action, native)
			result.Mutated = result.Mutated || native.Mutated
			result.DataRetained = result.DataRetained || native.DataRetained
			for _, next := range native.NextActions {
				result.NextActions = append(result.NextActions, managementNativeNextAction(next.Kind, action.Alias, item.TargetId))
			}
			if len(native.ManualActions) > 0 {
				result.NextActions = append(result.NextActions, managementNativeNextAction("manual", action.Alias, item.TargetId))
			}
		}
		result.Items = append(result.Items, item)
	}
	out := management.Outcome{State: management.Succeeded, Result: result, Diagnostics: []management.Diagnostic{}}
	if applyErr == nil {
		return out
	}
	out.State = management.Failed
	out.Result.Outcome = "failed"
	out.Result.Message = "Environment change stopped; inspect its result before retrying"
	if result.Mutated {
		out.State = management.Partial
		out.Result.Outcome = "incomplete"
	}
	if !result.Mutated && (errors.Is(applyErr, context.Canceled) || errors.Is(applyErr, context.DeadlineExceeded)) {
		out.State = management.Canceled
		out.Result.Outcome = "canceled"
	}
	out.Result.NextActions = append(out.Result.NextActions, management.NextAction{Kind: "inspect", Reason: "Inspect desired files and owned binding receipts, then prepare recovery or locked synchronization"})
	return out
}
