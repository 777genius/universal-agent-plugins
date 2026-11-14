package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (s Service) appendBatchUpdateResult(ctx context.Context, dryRun bool, record domain.InstallationRecord, report *domain.Report) bool {
	item, err := s.planBatchUpdateResult(ctx, dryRun, record)
	if err != nil {
		appendBatchUpdateSkipWarning(report, record.IntegrationID, err)
		return false
	}
	appendBatchUpdateTargets(report, item)
	return true
}
