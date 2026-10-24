package agentpluginscli

import (
	"errors"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
)

func validateManagementChange(input management.ChangeRequest) error {
	if err := validateManagementFields(input); err != nil {
		return err
	}
	if err := validateManagementAction(input); err != nil {
		return err
	}
	if len(input.Aliases) > 2000 || len(input.CandidateIds) > 2000 {
		return errors.New("selection limit exceeded")
	}
	seen := map[string]bool{}
	for _, alias := range input.Aliases {
		if alias == "" || seen[alias] {
			return errors.New("aliases must be unique and nonempty")
		}
		seen[alias] = true
	}
	return nil
}
func validateManagementFields(input management.ChangeRequest) error {
	if input.Action != management.Remove && input.ExternalUninstalled {
		return errors.New("externalUninstalled applies only to remove")
	}
	if input.Action != management.Recover && input.OperationId != "" {
		return errors.New("operationId applies only to recovery")
	}
	if input.Action != management.Install && (input.Source != "" || input.Alias != "" || input.Ref != "") {
		return errors.New("source, alias and ref apply only to install")
	}
	if input.Action != management.InstallSkills && (input.CatalogId != "" || len(input.CandidateIds) > 0) {
		return errors.New("catalog selections apply only to install_skills")
	}
	if input.All && (input.Action != management.Update || len(input.Aliases) > 0) {
		return errors.New("all applies to update without aliases")
	}
	return nil
}
func validateManagementAction(input management.ChangeRequest) error {
	switch input.Action {
	case management.Install:
		if input.Source == "" || len(input.Aliases) > 0 {
			return errors.New("install requires one source")
		}
	case management.InstallSkills:
		if input.CatalogId == "" || len(input.CandidateIds) == 0 || len(input.Aliases) > 0 {
			return errors.New("install_skills requires a retained catalog selection")
		}
	case management.Update:
		if !input.All && len(input.Aliases) == 0 {
			return errors.New("explicit aliases or all required")
		}
	case management.Remove, management.Repair:
		if len(input.Aliases) == 0 {
			return errors.New("explicit aliases required")
		}
	case management.Sync, management.Recover:
		if len(input.Aliases) > 0 || len(input.TargetIds) > 0 {
			return errors.New("sync and recover use the captured complete environment")
		}
	default:
		return errors.New("unsupported change action")
	}
	return nil
}
