package usecase

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (s Service) newBatchUpdateReport(count int) domain.Report {
	return domain.Report{
		OperationID: operationID("batch_update", "all", s.now()),
		Summary:     fmt.Sprintf("Processed update for %d managed integration(s).", count),
	}
}

func finalizeBatchUpdateReport(report *domain.Report, successes int) {
	if successes == 0 && len(report.Warnings) > 0 {
		report.Summary = "No managed integrations were updated successfully."
	}
}
