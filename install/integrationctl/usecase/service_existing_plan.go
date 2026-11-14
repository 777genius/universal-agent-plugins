package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type plannedExistingTarget struct {
	TargetID domain.TargetID
	Current  domain.TargetInstallation
	Delivery domain.Delivery
	Adapter  ports.TargetAdapter
	Inspect  ports.InspectResult
	Plan     ports.AdapterPlan
	Manifest *domain.IntegrationManifest
	Resolved *ports.ResolvedSource
	Report   domain.TargetReport
	Adopted  bool
}

func (s Service) executeExisting(ctx context.Context, in NamedDryRunInput, action string) (domain.Report, error) {
	return s.planExisting(ctx, in, action)
}

func (s Service) planExisting(ctx context.Context, in NamedDryRunInput, action string) (domain.Report, error) {
	record, err := s.loadExistingPlanRecord(ctx, in.Name)
	if err != nil {
		return domain.Report{}, err
	}
	report := newExistingPlanReport(action, record.IntegrationID, s.now())
	sharedResolved, sharedManifest, cleanupShared, err := s.resolveExistingSharedSource(ctx, record, action, in.DryRun)
	if err != nil {
		return domain.Report{}, err
	}
	defer cleanupShared()
	selectedTargetIDs, err := s.selectExistingTargets(record, in.Target, action)
	if err != nil {
		return domain.Report{}, err
	}
	planned, err := s.planSelectedExistingTargets(ctx, record, selectedTargetIDs, action, sharedResolved, sharedManifest)
	if err != nil {
		return domain.Report{}, err
	}
	appendExistingTargetReports(&report, planned)
	adopted, warnings, err := s.planAdoptedExistingTargets(ctx, record, action, sharedResolved, sharedManifest)
	if err != nil {
		cleanupPlannedExisting(planned)
		return domain.Report{}, err
	}
	planned = append(planned, adopted...)
	appendExistingTargetReports(&report, adopted)
	report.Warnings = append(report.Warnings, warnings...)
	report = finalizeExistingPlanReport(report)
	if in.DryRun {
		cleanupPlannedExisting(planned)
		return report, nil
	}
	return s.applyExisting(ctx, record, action, planned)
}
