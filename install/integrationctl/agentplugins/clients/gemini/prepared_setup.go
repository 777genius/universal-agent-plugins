package gemini

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
		// Transport cannot be established by this pure classifier. An existing
		// initialized mixed package is handled by the read-only verifier instead.
		if component.Kind != domain.ComponentSkill || component.Support != domain.SupportNative {
			return false
		}
	}
	return true
}
