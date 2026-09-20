package installer

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (e *Engine) confirmPreparedPlan(ctx context.Context, prepared *PreparedOperation) error {
	plan := prepared.plan
	if len(plan.Targets) > 1 {
		return e.confirmGroupPlan(ctx, prepared)
	}
	if err := e.confirmInstallTarget(ctx, prepared); err != nil {
		return err
	}
	state, err := e.store.Load()
	if err != nil {
		return fmt.Errorf("read installation state: %w", err)
	}
	installation, ok := findInstall(state, plan.InstallationID)
	if !ok {
		if plan.Operation == OpRemove {
			return fmt.Errorf("%w: installation is not installed", ErrPlanChanged)
		}
		return nil
	}
	binding, _, ok := findBinding(installation, prepared.client.ClientID)
	if plan.Operation == OpRemove {
		return confirmRemovalTarget(PlanTarget{NoChange: plan.NoChange, BindingID: plan.BindingID, TargetPath: plan.TargetPath}, binding, ok)
	}
	if !ok {
		return nil
	}
	if plan.TargetPath != "" && binding.TargetLocator != "" && binding.TargetLocator != plan.TargetPath {
		return fmt.Errorf("%w: live target does not match confirmed plan", ErrPlanChanged)
	}
	if plan.BindingID != "" && binding.ClientBindingID != "" && binding.ClientBindingID != plan.BindingID {
		return fmt.Errorf("%w: live binding does not match confirmed plan", ErrPlanChanged)
	}
	return nil
}

func (e *Engine) confirmInstallTarget(ctx context.Context, prepared *PreparedOperation) error {
	plan := prepared.plan
	if plan.Operation == OpInstall {
		if err := e.refuseRecordedDigestRewrite(plan.InstallationID, plan.TreeDigest); err != nil {
			return err
		}
		if prepared.artifact != "" {
			target, err := e.planner().ResolveTarget(ctx, prepared.client, domain.ScopeUser, prepared.artifact)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrPlanChanged, err)
			}
			if plan.TargetPath != "" && target.ActivePath != plan.TargetPath {
				return fmt.Errorf("%w: live target does not match confirmed plan", ErrPlanChanged)
			}
		}
	}
	return nil
}

func confirmRemovalTarget(target PlanTarget, binding domain.ClientBinding, found bool) error {
	if target.NoChange {
		if found {
			return fmt.Errorf("%w: live target does not match confirmed plan", ErrPlanChanged)
		}
		return nil
	}
	if !found {
		return fmt.Errorf("%w: client is not installed", ErrPlanChanged)
	}
	if target.BindingID != "" && binding.ClientBindingID != target.BindingID {
		return fmt.Errorf("%w: live binding does not match confirmed plan", ErrPlanChanged)
	}
	if target.TargetPath != "" && binding.TargetLocator != target.TargetPath {
		return fmt.Errorf("%w: live target does not match confirmed plan", ErrPlanChanged)
	}
	return nil
}
