package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (s Service) planExistingTarget(ctx context.Context, record domain.InstallationRecord, targetID domain.TargetID, action string, sharedResolved *ports.ResolvedSource, sharedManifest *domain.IntegrationManifest) (plannedExistingTarget, error) {
	base, err := s.loadExistingTargetBase(ctx, record, targetID)
	if err != nil {
		return plannedExistingTarget{}, err
	}
	return s.dispatchExistingTargetPlan(ctx, record, action, base, sharedResolved, sharedManifest)
}

func (s Service) dispatchExistingTargetPlan(
	ctx context.Context,
	record domain.InstallationRecord,
	action string,
	base plannedExistingTarget,
	sharedResolved *ports.ResolvedSource,
	sharedManifest *domain.IntegrationManifest,
) (plannedExistingTarget, error) {
	switch action {
	case "remove_orphaned_target":
		return s.planExistingRemoval(ctx, record, base, sharedResolved, sharedManifest)
	case "enable_target":
		return s.planExistingToggle(ctx, record, base, true)
	case "disable_target":
		return s.planExistingToggle(ctx, record, base, false)
	case "update_version":
		return s.planExistingMutation(ctx, record, base, sharedResolved, sharedManifest, false)
	case "repair_drift":
		return s.planExistingMutation(ctx, record, base, sharedResolved, sharedManifest, true)
	default:
		return plannedExistingTarget{}, domain.NewError(domain.ErrUsage, "unsupported existing lifecycle action "+action, nil)
	}
}
