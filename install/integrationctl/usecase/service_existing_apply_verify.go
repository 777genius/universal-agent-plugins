package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (s Service) verifyPostApply(ctx context.Context, integrationID string, policy domain.InstallPolicy, record *domain.InstallationRecord, adapter ports.TargetAdapter, action string) (ports.InspectResult, error) {
	inspect, err := adapter.Inspect(ctx, ports.InspectInput{
		IntegrationID: integrationID,
		Record:        record,
		Scope:         policy.Scope,
	})
	if err != nil {
		return ports.InspectResult{}, domain.NewError(domain.ErrMutationApply, "post-apply verify inspect failed", err)
	}
	if err := validatePostApplyInspectState(inspect, action); err != nil {
		return inspect, err
	}
	return inspect, nil
}

func validatePostApplyInspectState(inspect ports.InspectResult, action string) error {
	switch action {
	case "add", "update_version", "repair_drift":
		if inspect.State == "" || inspect.State == domain.InstallRemoved {
			return domain.NewError(domain.ErrMutationApply, "post-apply verify did not observe an installed target state", nil)
		}
	case "enable_target":
		if inspect.State != domain.InstallInstalled {
			return domain.NewError(domain.ErrMutationApply, "post-apply verify did not observe an enabled installed target state", nil)
		}
	case "disable_target":
		if inspect.State != domain.InstallDisabled {
			return domain.NewError(domain.ErrMutationApply, "post-apply verify did not observe a disabled target state", nil)
		}
	case "remove_orphaned_target":
		if inspect.State != domain.InstallRemoved {
			return domain.NewError(domain.ErrMutationApply, "post-apply verify still observes the target after remove", nil)
		}
	}
	return nil
}
