package usecase

import (
	"context"
	"os"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (s Service) loadWorkspaceLockForSync(ctx context.Context) (domain.WorkspaceLock, error) {
	if s.WorkspaceLock == nil {
		return domain.WorkspaceLock{}, domain.NewError(domain.ErrUsage, "workspace lock store is not configured", nil)
	}
	lock, err := s.WorkspaceLock.Load(ctx)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.WorkspaceLock{}, domain.NewError(domain.ErrUsage, "workspace lock not found: "+s.WorkspaceLock.Path(), err)
		}
		return domain.WorkspaceLock{}, err
	}
	if strings.TrimSpace(lock.APIVersion) != "" && strings.TrimSpace(lock.APIVersion) != "v1" {
		return domain.WorkspaceLock{}, domain.NewError(domain.ErrUsage, "workspace lock api_version must be v1", nil)
	}
	return lock, nil
}

func (s Service) loadCurrentInstallations(ctx context.Context) (ports.StateFile, map[string]domain.InstallationRecord, error) {
	state, err := s.StateStore.Load(ctx)
	if err != nil {
		return ports.StateFile{}, nil, err
	}
	current := map[string]domain.InstallationRecord{}
	for _, record := range state.Installations {
		current[record.IntegrationID] = record
	}
	return state, current, nil
}
