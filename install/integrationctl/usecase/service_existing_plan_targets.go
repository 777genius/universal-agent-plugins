package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (s Service) planSelectedExistingTargets(
	ctx context.Context,
	record domain.InstallationRecord,
	targetIDs []domain.TargetID,
	action string,
	sharedResolved *ports.ResolvedSource,
	sharedManifest *domain.IntegrationManifest,
) ([]plannedExistingTarget, error) {
	planned := make([]plannedExistingTarget, 0, len(targetIDs))
	for _, targetID := range targetIDs {
		item, err := s.planExistingTarget(ctx, record, targetID, action, sharedResolved, sharedManifest)
		if err != nil {
			cleanupPlannedExisting(planned)
			return nil, err
		}
		planned = append(planned, item)
	}
	return planned, nil
}
