package installer

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// completedProjectGroup checks the existing fresh operation and saved identity
// before reporting file completion. It neither activates clients nor observes
// files: the core already performed those operations and retained their facts.
func completedProjectGroup(prepared *PreparedOperation, got usecase.GroupResult, state domain.StateFileV2) error {
	if got.Phase != usecase.GroupPhaseCompleted || got.InstallationID == "" ||
		got.InstallationID != prepared.plan.InstallationID || got.InstallationID != prepared.req.InstallationID || prepared.req.ProjectRoot == "" {
		return fmt.Errorf("%w: project group operation is not complete", ErrIncomplete)
	}
	size := len(prepared.plan.Targets)
	if size == 0 || len(got.Targets) != size || len(prepared.clients) != size || len(prepared.req.Targets) != size {
		return fmt.Errorf("%w: project group target set is inconsistent", ErrIncomplete)
	}
	installation, err := completedGroupInstallation(state, got.InstallationID)
	if err != nil {
		return err
	}
	targets := make(map[string]usecase.AddResult, size)
	for _, target := range got.Targets {
		id := string(target.Plan.ClientID)
		if _, duplicate := targets[id]; duplicate || id == "" {
			return fmt.Errorf("%w: project group target identity is ambiguous", ErrIncomplete)
		}
		targets[id] = target
	}
	for i, frozen := range prepared.plan.Targets {
		target, ok := targets[frozen.ClientID]
		if !ok || frozen.ClientID != string(prepared.clients[i].ClientID) || frozen.ClientID != prepared.req.Targets[i].ClientID {
			return fmt.Errorf("%w: project group target differs from preparation", ErrIncomplete)
		}
		delete(targets, frozen.ClientID)
		if err := completedProjectTarget(prepared, frozen, target, installation); err != nil {
			return err
		}
	}
	return nil
}

func completedGroupInstallation(state domain.StateFileV2, id string) (domain.Installation, error) {
	var found domain.Installation
	count := 0
	for _, installation := range state.Installations {
		if installation.InstallationID == id {
			found, count = installation, count+1
		}
	}
	if count != 1 {
		return domain.Installation{}, fmt.Errorf("%w: committed project installation is missing or ambiguous", ErrIncomplete)
	}
	return found, nil
}
