package kimi

import (
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.PreparedSetupCompleter = (*Adapter)(nil)

func (*Adapter) CanCompletePreparedSetup(request domain.ActivationRequest, observed domain.ActivationOutcome) bool {
	return !request.VerifyOnly && !request.ActivationComplete && !request.ConfirmPluginEnable &&
		kimiPureSkillPlan(request.Plan) && kimiSkillVersion(request.Client) &&
		kimiPlanMatches(request) && observed.Activation == domain.ActivationPrepared &&
		observed.Verification == domain.VerificationInstalled && observed.Policy == domain.PolicyAllowed &&
		!observed.ActivationAttested && !observed.AuthenticationAttested &&
		(observed.NativeEffect == "" || observed.NativeEffect == domain.NativeEffectUnchanged) && slices.Contains(observed.UserActions, kimiSkillCompletionAction)
}

func kimiPlanMatches(request domain.ActivationRequest) bool {
	_, err := kimiPlanRoot(request.Client, request.Plan)
	return err == nil && request.Client.ClientID == domain.ClientKimi && request.Delivery.ActivePath == request.Plan.ActivePath && request.Delivery.ClientID == domain.ClientKimi
}
