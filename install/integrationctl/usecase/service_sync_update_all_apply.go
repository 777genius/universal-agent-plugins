package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (s Service) runBatchUpdateResults(
	ctx context.Context,
	dryRun bool,
	installations []domain.InstallationRecord,
	report *domain.Report,
) int {
	successes := 0
	for _, record := range installations {
		if s.appendBatchUpdateResult(ctx, dryRun, record, report) {
			successes++
		}
	}
	return successes
}
