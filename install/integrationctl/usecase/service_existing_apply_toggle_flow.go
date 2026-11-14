package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func validateSingleExistingToggleTarget(planned []plannedExistingTarget) (plannedExistingTarget, error) {
	if len(planned) != 1 {
		return plannedExistingTarget{}, domain.NewError(domain.ErrMutationApply, "non-dry-run existing lifecycle currently supports one target at a time until rollback is implemented", nil)
	}
	return planned[0], nil
}

func ensureExistingTogglePlanAllowed(target plannedExistingTarget) error {
	if target.Plan.Blocking {
		return domain.NewError(domain.ErrMutationApply, "planned mutation is blocked for target "+string(target.TargetID)+"; rerun with --dry-run to inspect manual steps", nil)
	}
	return nil
}

func (s Service) appendExistingPlanJournal(ctx context.Context, operationID string, target plannedExistingTarget) error {
	if err := s.Journal.AppendStep(ctx, operationID, domain.JournalStep{Target: string(target.TargetID), Action: "inspect", Status: "ok"}); err != nil {
		return err
	}
	if err := s.Journal.AppendStep(ctx, operationID, domain.JournalStep{Target: string(target.TargetID), Action: "plan", Status: "ok"}); err != nil {
		return err
	}
	return nil
}

func newExistingToggleTargetReport(record domain.InstallationRecord, target plannedExistingTarget, verified ports.InspectResult, applyResult ports.ApplyResult) domain.TargetReport {
	return toAppliedTargetReport(record.IntegrationID, target.Delivery, target.Inspect, verified, target.Plan, applyResult)
}
