package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (s Service) planBatchUpdateResult(ctx context.Context, dryRun bool, record domain.InstallationRecord) (domain.Report, error) {
	return s.planExisting(ctx, NamedDryRunInput{Name: record.IntegrationID, DryRun: dryRun}, "update_version")
}
