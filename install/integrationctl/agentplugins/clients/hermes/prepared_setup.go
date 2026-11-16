package hermes

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.PreparedSetupCompleter = (*Adapter)(nil)

// CanCompletePreparedSetup admits only an exact Skills-only Project install,
// whose confirmed repair can ask the detected Hermes CLI for project trust.
// Project MCP sessions and User installs have no trust step to finish.
func (*Adapter) CanCompletePreparedSetup(request domain.ActivationRequest, outcome domain.ActivationOutcome) bool {
	if outcome.Activation != domain.ActivationPrepared || outcome.Verification != domain.VerificationInstalled || outcome.Policy != domain.PolicyAllowed ||
		request.Plan.Scope != domain.ScopeProject || request.Plan.TargetContext.Scope != domain.ScopeProject || request.Plan.InstallIntent != domain.InstallIntentAutomatic {
		return false
	}
	profile, project := request.Client.ConfigRoot, request.Plan.TargetContext.ProjectRoot
	if !cleanRoot(profile) || !cleanRoot(project) || !cleanRoot(request.Client.ExecutablePath) ||
		request.Plan.NativeRegistryRoot != filepath.Join(project, ".hermes") ||
		request.Plan.TargetContext.UserConfigRoot != "" && request.Plan.TargetContext.UserConfigRoot != profile {
		return false
	}
	if len(request.Plan.Components) == 0 {
		return false
	}
	for _, component := range request.Plan.Components {
		if component.Kind != domain.ComponentSkill || component.Support != domain.SupportNative {
			return false
		}
	}
	return true
}
