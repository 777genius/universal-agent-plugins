package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (s Service) runSync(ctx context.Context, dryRun bool) (domain.Report, error) {
	lock, state, current, err := s.loadSyncWorkspaceState(ctx)
	if err != nil {
		return domain.Report{}, err
	}
	return s.runWorkspaceSyncPhases(ctx, dryRun, lock.Integrations, state.Installations, current), nil
}

func (s Service) runWorkspaceSyncPhases(
	ctx context.Context,
	dryRun bool,
	items []domain.WorkspaceLockIntegration,
	installations []domain.InstallationRecord,
	current map[string]domain.InstallationRecord,
) domain.Report {
	report := s.newWorkspaceSyncReport(len(items))
	desiredIDs := newSyncDesiredIDs()
	s.syncDesiredIntegrations(ctx, dryRun, items, current, desiredIDs, &report)
	s.syncUndesiredIntegrations(ctx, dryRun, installations, desiredIDs, &report)
	finalizeWorkspaceSyncReport(&report)
	return report
}

func (s Service) syncDesiredIntegrations(
	ctx context.Context,
	dryRun bool,
	items []domain.WorkspaceLockIntegration,
	current map[string]domain.InstallationRecord,
	desiredIDs map[string]struct{},
	report *domain.Report,
) {
	for _, item := range items {
		s.syncDesiredItem(ctx, dryRun, item, current, desiredIDs, report)
	}
}

func (s Service) syncDesiredItem(
	ctx context.Context,
	dryRun bool,
	item domain.WorkspaceLockIntegration,
	current map[string]domain.InstallationRecord,
	desiredIDs map[string]struct{},
	report *domain.Report,
) {
	s.syncDesiredIntegration(ctx, dryRun, item, current, desiredIDs, report)
}
