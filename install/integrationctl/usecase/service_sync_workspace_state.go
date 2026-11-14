package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (s Service) loadSyncWorkspaceState(ctx context.Context) (domain.WorkspaceLock, ports.StateFile, map[string]domain.InstallationRecord, error) {
	lock, err := s.loadWorkspaceLockForSync(ctx)
	if err != nil {
		return domain.WorkspaceLock{}, ports.StateFile{}, nil, err
	}
	state, current, err := s.loadCurrentInstallations(ctx)
	if err != nil {
		return domain.WorkspaceLock{}, ports.StateFile{}, nil, err
	}
	return lock, state, current, nil
}

func newSyncDesiredIDs() map[string]struct{} {
	return map[string]struct{}{}
}
