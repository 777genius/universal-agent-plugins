package usecase

import (
	"context"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (s Service) loadManagedSyncInstallations(ctx context.Context) ([]domain.InstallationRecord, error) {
	state, err := s.StateStore.Load(ctx)
	if err != nil {
		return nil, err
	}
	return sortedSyncInstallations(state.Installations), nil
}

func sortedSyncInstallations(installations []domain.InstallationRecord) []domain.InstallationRecord {
	copied := append([]domain.InstallationRecord(nil), installations...)
	sort.Slice(copied, func(i, j int) bool { return copied[i].IntegrationID < copied[j].IntegrationID })
	return copied
}
