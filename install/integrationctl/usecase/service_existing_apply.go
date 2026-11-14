package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type removedExistingTarget struct {
	Planned plannedExistingTarget
	Result  ports.ApplyResult
}

func (s Service) applyExisting(ctx context.Context, record domain.InstallationRecord, action string, planned []plannedExistingTarget) (domain.Report, error) {
	if action == "remove_orphaned_target" {
		return s.applyRemoveExisting(ctx, record, planned)
	}
	if action == "repair_drift" {
		return s.applyRepairExisting(ctx, record, planned)
	}
	if action == "update_version" {
		return s.applyUpdateExisting(ctx, record, planned)
	}
	return s.applyToggleExisting(ctx, record, action, planned)
}
