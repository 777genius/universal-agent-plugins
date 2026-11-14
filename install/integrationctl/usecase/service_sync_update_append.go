package usecase

import "github.com/777genius/plugin-kit-ai/install/integrationctl/domain"

func appendBatchUpdateSkipWarning(report *domain.Report, integrationID string, err error) {
	report.Warnings = append(report.Warnings, batchUpdateSkipWarning(integrationID, err))
}

func appendBatchUpdateTargets(report *domain.Report, item domain.Report) {
	report.Targets = append(report.Targets, item.Targets...)
}
