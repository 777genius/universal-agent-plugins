package gemini

import (
	"path/filepath"
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.PreparedSetupCompleter = (*Adapter)(nil)

func (*Adapter) CanCompletePreparedSetup(request domain.ActivationRequest, outcome domain.ActivationOutcome) bool {
	if outcome.Activation != domain.ActivationPrepared || outcome.Verification != domain.VerificationInstalled || outcome.Policy != domain.PolicyAllowed || request.Plan.Scope != domain.ScopeProject || request.Plan.TargetContext.Scope != domain.ScopeProject || request.Plan.InstallIntent != domain.InstallIntentAutomatic {
		return false
	}
	root := request.Client.ConfigRoot
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Base(root) != ".gemini" {
		return false
	}
	project := request.Plan.TargetContext.ProjectRoot
	if !filepath.IsAbs(project) || filepath.Clean(project) != project || filepath.Dir(project) == project || request.Plan.NativeRegistryRoot != filepath.Join(project, ".gemini") || !filepath.IsAbs(request.Client.ExecutablePath) {
		return false
	}
	if len(request.Plan.Components) == 0 {
		return false
	}
	for _, component := range request.Plan.Components {
		if !geminiPreparedComponentReady(component, outcome.UserActions) {
			return false
		}
	}
	return true
}

func geminiPreparedComponentReady(component domain.ComponentDecision, actions []string) bool {
	if component.Support != domain.SupportNative {
		return false
	}
	switch component.Kind {
	case domain.ComponentSkill:
		return true
	case domain.ComponentMCPServer:
		// Only a fresh verifier that checked exact local owned entries may
		// offer the existing confirmed repair path for this acting check.
		return slices.Contains(actions, geminiMCPConfirmationAction)
	default:
		return false
	}
}
