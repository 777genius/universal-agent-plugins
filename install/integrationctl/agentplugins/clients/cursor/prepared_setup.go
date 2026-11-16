package cursor

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.PreparedSetupCompleter = (*Adapter)(nil)

func (*Adapter) CanCompletePreparedSetup(request domain.ActivationRequest, outcome domain.ActivationOutcome) bool {
	if outcome.Activation != domain.ActivationPrepared || outcome.Verification != domain.VerificationInstalled || outcome.Policy != domain.PolicyAllowed || request.Plan.Scope != domain.ScopeProject || request.Plan.TargetContext.Scope != domain.ScopeProject || request.Plan.InstallIntent != domain.InstallIntentAutomatic {
		return false
	}
	root := request.Client.ConfigRoot
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Base(root) != ".cursor" {
		return false
	}
	project := request.Plan.TargetContext.ProjectRoot
	if !filepath.IsAbs(project) || filepath.Clean(project) != project || filepath.Dir(project) == project || request.Plan.NativeRegistryRoot != filepath.Join(project, ".cursor") {
		return false
	}
	names, allMCP := cursorSelectedMCP(request.Plan.Components)
	if !allMCP || len(names) == 0 {
		return false
	}
	for _, surface := range request.Client.Surfaces {
		if surface.ID == "cursor_agent" && surface.Detected {
			return true
		}
	}
	return false
}
