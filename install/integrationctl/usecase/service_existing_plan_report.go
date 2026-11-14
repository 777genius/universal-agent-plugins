package usecase

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func appendExistingTargetReports(report *domain.Report, planned []plannedExistingTarget) {
	for _, item := range planned {
		report.Targets = append(report.Targets, item.Report)
	}
}

func finalizeExistingPlanReport(report domain.Report) domain.Report {
	sort.Slice(report.Targets, func(i, j int) bool { return report.Targets[i].TargetID < report.Targets[j].TargetID })
	return report
}

func (s Service) loadExistingPlanRecord(ctx context.Context, name string) (domain.InstallationRecord, error) {
	state, err := s.StateStore.Load(ctx)
	if err != nil {
		return domain.InstallationRecord{}, err
	}
	record, ok := findInstallation(state.Installations, name)
	if !ok {
		return domain.InstallationRecord{}, domain.NewError(domain.ErrStateConflict, "integration not found in state: "+name, nil)
	}
	return record, nil
}

func newExistingPlanReport(action, integrationID string, now time.Time) domain.Report {
	return domain.Report{
		OperationID: operationID("plan_"+action, integrationID, now),
		Summary:     fmt.Sprintf("Dry-run %s plan for %q.", action, integrationID),
	}
}
