package claude

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type inspectionState struct {
	installed    bool
	state        domain.InstallState
	restrictions []domain.EnvironmentRestrictionCode
	settings     []string
}

func (a Adapter) inspectState(ctx context.Context, in ports.InspectInput) (inspectionState, error) {
	inspect := a.newInspectContext(in)
	restrictions, settingsFiles := a.inspectRestrictions(inspect, in.Record)
	state, err := a.inspectInstalledState(ctx, inspect, in.Record)
	if err != nil {
		return inspectionState{}, err
	}
	return inspectionState{
		installed:    inspect.installed,
		state:        state,
		restrictions: restrictions,
		settings:     settingsFiles,
	}, nil
}

func (s inspectionState) result(targetID domain.TargetID) ports.InspectResult {
	return ports.InspectResult{
		TargetID:                targetID,
		Installed:               s.installed,
		State:                   s.state,
		ActivationState:         domain.ActivationReloadPending,
		ConfigPrecedenceContext: []string{"project", "user", "managed"},
		EnvironmentRestrictions: s.restrictions,
		SettingsFiles:           s.settings,
		EvidenceClass:           domain.EvidenceConfirmed,
	}
}

func (a Adapter) inspectInstalledState(ctx context.Context, inspect inspectContext, record *domain.InstallationRecord) (domain.InstallState, error) {
	if inspect.cliAvailable && inspect.integrationID != "" {
		if nativeState, ok, err := a.inspectPluginList(ctx, inspect, record); err != nil {
			return "", err
		} else if ok {
			return nativeState, nil
		}
	}
	if persisted, ok := persistedClaudeState(record); ok {
		switch persisted {
		case domain.InstallDegraded, domain.InstallActivationPending, domain.InstallAuthPending, domain.InstallDisabled:
			return persisted, nil
		case domain.InstallInstalled, domain.InstallRemoved:
			if !inspect.cliAvailable {
				return persisted, nil
			}
		}
	}
	if inspect.installed || inspect.cliAvailable {
		return domain.InstallInstalled, nil
	}
	return domain.InstallRemoved, nil
}

func persistedClaudeState(record *domain.InstallationRecord) (domain.InstallState, bool) {
	if record == nil {
		return "", false
	}
	target, ok := record.Targets[domain.TargetClaude]
	if !ok || target.State == "" {
		return "", false
	}
	return target.State, true
}
