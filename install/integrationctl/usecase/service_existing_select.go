package usecase

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (s Service) selectExistingTargets(record domain.InstallationRecord, requestedTarget string, action string) ([]domain.TargetID, error) {
	if strings.TrimSpace(requestedTarget) != "" {
		targetID := domain.TargetID(strings.TrimSpace(requestedTarget))
		if _, ok := record.Targets[targetID]; !ok {
			return nil, domain.NewError(domain.ErrStateConflict, "target missing from installation record: "+string(targetID), nil)
		}
		return []domain.TargetID{targetID}, nil
	}
	if action != "enable_target" && action != "disable_target" {
		return sortedTargets(record.Targets), nil
	}
	return s.selectTogglableExistingTargets(record, action)
}

func (s Service) selectTogglableExistingTargets(record domain.InstallationRecord, action string) ([]domain.TargetID, error) {
	var togglable []domain.TargetID
	for _, targetID := range sortedTargets(record.Targets) {
		adapter, ok := s.Adapters[targetID]
		if !ok {
			continue
		}
		if _, ok := adapter.(ports.ToggleTargetAdapter); ok {
			togglable = append(togglable, targetID)
		}
	}
	switch len(togglable) {
	case 0:
		return nil, domain.NewError(domain.ErrUnsupportedTarget, "no installed targets for "+record.IntegrationID+" support "+strings.TrimSuffix(action, "_target"), nil)
	case 1:
		return togglable, nil
	default:
		return nil, domain.NewError(domain.ErrUsage, "multiple installed targets support "+strings.TrimSuffix(action, "_target")+"; rerun with --target", nil)
	}
}
