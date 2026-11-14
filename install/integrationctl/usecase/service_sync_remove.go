package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (s Service) syncUndesiredIntegrations(ctx context.Context, dryRun bool, installations []domain.InstallationRecord, desiredIDs map[string]struct{}, report *domain.Report) {
	for _, record := range installations {
		if syncKeepUndesiredRecord(record, desiredIDs) {
			continue
		}
		if handled := s.syncUndesiredRecord(ctx, dryRun, record, report); handled {
			continue
		}
	}
}

func syncKeepUndesiredRecord(record domain.InstallationRecord, desiredIDs map[string]struct{}) bool {
	_, keep := desiredIDs[record.IntegrationID]
	return keep
}

func (s Service) syncUndesiredRecord(ctx context.Context, dryRun bool, record domain.InstallationRecord, report *domain.Report) bool {
	if warning, skip := syncUndesiredScopeWarning(record); skip {
		report.Warnings = append(report.Warnings, warning)
		return true
	}
	itemReport, err := s.Remove(ctx, NamedDryRunInput{Name: record.IntegrationID, DryRun: dryRun})
	if err != nil {
		report.Warnings = append(report.Warnings, syncUndesiredRemoveFailureWarning(record, err))
		return true
	}
	report.Targets = append(report.Targets, itemReport.Targets...)
	return false
}

func syncUndesiredScopeWarning(record domain.InstallationRecord) (string, bool) {
	if record.Policy.Scope == "project" {
		return "", false
	}
	return "Sync skipped unmanaged-scope removal for " + record.IntegrationID + ": scope=" + record.Policy.Scope, true
}

func syncUndesiredRemoveFailureWarning(record domain.InstallationRecord, err error) string {
	return "Sync remove failed for " + record.IntegrationID + ": " + err.Error()
}
