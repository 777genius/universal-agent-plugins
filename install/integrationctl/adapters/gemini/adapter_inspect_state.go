package gemini

import (
	"context"
	"os"
	"os/exec"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type inspectState struct {
	settings          string
	workspaceSettings string
	enablement        string
	trusted           string
	systemSettings    []string
	installed         bool
	disabled          bool
	restrictions      []domain.EnvironmentRestrictionCode
}

func (a Adapter) inspectState(ctx context.Context, in ports.InspectInput) (inspectState, error) {
	paths := a.inspectPaths(in)
	_, cmdErr := exec.LookPath("gemini")
	_, extErr := os.Stat(paths.extensionDir)

	restrictions := []domain.EnvironmentRestrictionCode{}
	if cmdErr != nil {
		restrictions = append(restrictions, domain.RestrictionSourceToolMissing)
	}

	disabled, err := a.isDisabled(ctx, in)
	if err != nil {
		return inspectState{}, err
	}

	return inspectState{
		settings:          paths.settings,
		workspaceSettings: paths.workspaceSettings,
		enablement:        paths.enablement,
		trusted:           paths.trusted,
		systemSettings:    a.systemSettingsPaths(),
		installed:         extErr == nil,
		disabled:          disabled,
		restrictions:      restrictions,
	}, nil
}

func (s inspectState) result(targetID domain.TargetID) ports.InspectResult {
	state := domain.InstallRemoved
	if s.installed {
		if s.disabled {
			state = domain.InstallDisabled
		} else {
			state = domain.InstallInstalled
		}
	}

	settingsFiles := []string{s.settings, s.enablement, s.trusted}
	if s.workspaceSettings != "" {
		settingsFiles = append(settingsFiles, s.workspaceSettings)
	}
	settingsFiles = append(settingsFiles, s.systemSettings...)

	return ports.InspectResult{
		TargetID:                targetID,
		Installed:               s.installed,
		State:                   state,
		ActivationState:         domain.ActivationRestartPending,
		ConfigPrecedenceContext: []string{"cli", "env", "system_settings", "system_defaults", "workspace", "user"},
		EnvironmentRestrictions: s.restrictions,
		TrustResolutionSource:   s.trusted,
		SettingsFiles:           dedupeStrings(settingsFiles),
		EvidenceClass:           domain.EvidenceConfirmed,
	}
}
