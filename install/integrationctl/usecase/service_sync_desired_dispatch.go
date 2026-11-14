package usecase

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (s Service) applyDesiredSyncPlan(ctx context.Context, dryRun bool, plan desiredSyncPlan, report *domain.Report) {
	switch plan.Action {
	case desiredSyncActionAdd:
		s.syncDesiredAdd(ctx, dryRun, plan.IntegrationID, plan.Source, plan.DesiredPolicy, plan.Targets, report)
	case desiredSyncActionReplace:
		s.syncDesiredReplace(ctx, dryRun, plan.Record, plan.IntegrationID, plan.Source, plan.DesiredPolicy, plan.Targets, report)
	case desiredSyncActionUpdate:
		s.syncDesiredUpdate(ctx, dryRun, plan.IntegrationID, report)
	default:
		report.Warnings = append(report.Warnings, syncDesiredNoopWarning(plan.IntegrationID))
	}
}

func desiredSyncWarning(item domain.WorkspaceLockIntegration, err error) string {
	var manifestErr desiredSyncManifestErr
	if errors.As(err, &manifestErr) {
		return syncDesiredManifestWarning(manifestErr.integrationID, manifestErr.err)
	}
	return syncDesiredSourceWarning(item.Source, err)
}
