package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (s Service) runUpdateAll(ctx context.Context, dryRun bool) (domain.Report, error) {
	installations, err := s.loadManagedSyncInstallations(ctx)
	if err != nil {
		return domain.Report{}, err
	}
	if len(installations) == 0 {
		return domain.Report{Summary: "No managed integrations to update."}, nil
	}
	report := s.newBatchUpdateReport(len(installations))
	successes := s.runBatchUpdateResults(ctx, dryRun, installations, &report)
	finalizeBatchUpdateReport(&report, successes)
	sortSyncReportTargets(&report)
	return report, nil
}
