package installer

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (e *Engine) reconcileGroupHostHandoff(ctx context.Context, prepared *PreparedOperation, result *Result) (bool, error) {
	state, err := e.store.Load()
	if err != nil {
		return false, fmt.Errorf("read installation state: %w", err)
	}
	installationID := firstNonEmpty(prepared.req.InstallationID, prepared.plan.InstallationID)
	installation, ok := findInstall(state, installationID)
	if !ok {
		return false, nil
	}
	pending := false
	for _, client := range prepared.clients {
		binding, receipt, found := findBinding(installation, client.ClientID)
		if !found {
			continue
		}
		if hostHandoffPending(binding) {
			pending = true
		}
		if e.cfg.OnCommittedBinding == nil {
			continue
		}
		facts := BindingFacts{
			InstallationID: firstNonEmpty(installation.InstallationID, installationID),
			ClientID:       binding.ClientID,
			BindingID:      binding.ClientBindingID,
			Scope:          binding.Scope,
			TargetPath:     binding.TargetLocator,
			DataRoot:       receipt.Locator,
			DataReceiptID:  binding.DataReceiptID,
			OperationID:    prepared.req.OperationID,
			TreeDigest:     recordedBindingDigest(binding, planClientDigest(prepared.plan, binding.ClientID)),
		}
		if err := e.cfg.OnCommittedBinding(ctx, facts); err != nil {
			e.attachLiveGroupResult(result, prepared, installation)
			if result.Client.ClientID == "" {
				result.Client = liveClientResult(binding, prepared.req.RequiredComponents, facts.TreeDigest)
				result.Binding = facts
			}
			return true, err
		}
	}
	return pending, nil
}

func (e *Engine) anyGroupHostHandoffPending(prepared *PreparedOperation) bool {
	state, err := e.store.Load()
	if err != nil {
		return false
	}
	installation, ok := findInstall(state, firstNonEmpty(prepared.req.InstallationID, prepared.plan.InstallationID))
	if !ok {
		return false
	}
	for _, client := range prepared.clients {
		binding, _, found := findBinding(installation, client.ClientID)
		if found && hostHandoffPending(binding) {
			return true
		}
	}
	return false
}

func (e *Engine) attachLiveGroupResult(result *Result, prepared *PreparedOperation, installation domain.Installation) {
	result.InstallationID = firstNonEmpty(installation.InstallationID, result.InstallationID)
	result.Targets = nil
	for _, client := range prepared.clients {
		binding, receipt, found := findBinding(installation, client.ClientID)
		if !found {
			continue
		}
		item := liveClientResult(binding, prepared.req.RequiredComponents, planClientDigest(prepared.plan, binding.ClientID))
		result.Targets = append(result.Targets, item)
		if result.Client.ClientID == "" {
			result.Client = item
			result.Binding = BindingFacts{
				InstallationID: result.InstallationID, ClientID: binding.ClientID,
				BindingID: binding.ClientBindingID, Scope: binding.Scope, TargetPath: binding.TargetLocator,
				DataRoot: receipt.Locator, DataReceiptID: binding.DataReceiptID,
				OperationID: prepared.req.OperationID, TreeDigest: item.TreeDigest,
			}
		}
	}
}

func (e *Engine) confirmGroupPlan(ctx context.Context, prepared *PreparedOperation) error {
	plan := prepared.plan
	if plan.Operation == OpInstall {
		if err := e.refuseRecordedDigestRewrite(plan.InstallationID, plan.TreeDigest); err != nil {
			return err
		}
	}
	state, err := e.store.Load()
	if err != nil {
		return fmt.Errorf("read installation state: %w", err)
	}
	installation, ok := findInstall(state, plan.InstallationID)
	if !ok {
		if plan.Operation == OpRemove && !plan.NoChange {
			return fmt.Errorf("%w: installation is not installed", ErrPlanChanged)
		}
		return nil
	}
	for i, target := range plan.Targets {
		binding, _, found := findBinding(installation, domain.ClientID(target.ClientID))
		if plan.Operation == OpRemove {
			if err := confirmRemovalTarget(target, binding, found); err != nil {
				return err
			}
			continue
		}
		if err := e.confirmMutatingGroupTarget(ctx, prepared, i, target, binding, found); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) confirmMutatingGroupTarget(ctx context.Context, prepared *PreparedOperation, i int, target PlanTarget, binding domain.ClientBinding, found bool) error {
	plan := prepared.plan
	if !found {
		return nil
	}
	if target.TargetPath != "" && binding.TargetLocator != "" && binding.TargetLocator != target.TargetPath {
		return fmt.Errorf("%w: live target does not match confirmed plan", ErrPlanChanged)
	}
	if i == 0 && prepared.artifact != "" && plan.Operation == OpInstall {
		client := prepared.clients[i]
		resolved, resolveErr := e.planner().ResolveTarget(ctx, client, domain.ScopeUser, prepared.artifact)
		if resolveErr != nil {
			return fmt.Errorf("%w: %w", ErrPlanChanged, resolveErr)
		}
		if target.TargetPath != "" && resolved.ActivePath != target.TargetPath {
			return fmt.Errorf("%w: live target does not match confirmed plan", ErrPlanChanged)
		}
	}
	return nil
}
