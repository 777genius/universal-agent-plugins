package hermes

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.PreparedSetupCompleter = (*Adapter)(nil)

// Existing confirmed repair owns native completion of pure User Skills. Only
// explicit Project add consent can additionally ask the CLI for project trust;
// maintenance and Project MCP never acquire that consent here.
func (*Adapter) CanCompletePreparedSetup(request domain.ActivationRequest, outcome domain.ActivationOutcome) bool {
	if request.VerifyOnly || outcome.Activation != domain.ActivationPrepared || outcome.Verification != domain.VerificationInstalled || outcome.Policy != domain.PolicyAllowed ||
		request.Plan.Scope != request.Plan.TargetContext.Scope || request.Plan.InstallIntent != domain.InstallIntentAutomatic {
		return false
	}
	profile, project := request.Client.ConfigRoot, request.Plan.TargetContext.ProjectRoot
	if !cleanRoot(profile) || !cleanRoot(request.Client.ExecutablePath) ||
		request.Plan.TargetContext.UserConfigRoot != "" && request.Plan.TargetContext.UserConfigRoot != profile {
		return false
	}
	if request.Plan.Scope == domain.ScopeUser {
		return hermesUserSkillSetup(request, outcome)
	}
	if request.Plan.Scope != domain.ScopeProject || !request.ProjectTrustConsent || !cleanRoot(project) ||
		request.Plan.NativeRegistryRoot != filepath.Join(project, ".hermes") {
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

func hermesUserSkillSetup(request domain.ActivationRequest, outcome domain.ActivationOutcome) bool {
	return !request.ProjectTrustConsent && request.Plan.TargetContext.ProjectRoot == "" &&
		request.Plan.NativeRegistryRoot == request.Client.ConfigRoot && !request.ActivationComplete &&
		!outcome.ActivationAttested && !outcome.AuthenticationAttested && outcome.NativeEffect != domain.NativeEffectUncertain &&
		hermesSkillCLISelected(request.Client) && hermesPureSkillPlan(request.Plan)
}
