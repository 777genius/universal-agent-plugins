package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (s Service) applyExistingToggleMutation(ctx context.Context, action string, record domain.InstallationRecord, target plannedExistingTarget) (ports.ApplyResult, error) {
	toggle := target.Adapter.(ports.ToggleTargetAdapter)
	input := buildExistingToggleApplyInput(record, target)
	apply, err := existingToggleMutationApply(toggle, action)
	if err != nil {
		return ports.ApplyResult{}, err
	}
	return apply(ctx, input)
}

func buildExistingToggleApplyInput(record domain.InstallationRecord, target plannedExistingTarget) ports.ApplyInput {
	return ports.ApplyInput{
		Plan:    target.Plan,
		Policy:  record.Policy,
		Inspect: target.Inspect,
		Record:  &record,
	}
}

func existingToggleMutationApply(toggle ports.ToggleTargetAdapter, action string) (func(context.Context, ports.ApplyInput) (ports.ApplyResult, error), error) {
	switch action {
	case "enable_target":
		return toggle.ApplyEnable, nil
	case "disable_target":
		return toggle.ApplyDisable, nil
	default:
		return nil, domain.NewError(domain.ErrUsage, "unsupported existing lifecycle action "+action, nil)
	}
}
