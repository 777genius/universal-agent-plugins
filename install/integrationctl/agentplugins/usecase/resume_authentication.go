package usecase

import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"

// Native readiness can resolve an unknown requirement to no authentication.
// This does not acknowledge or replace an explicit authentication decision.
func (service Service) observedAuthenticationNotRequired(input AddInput, plan domain.DeliveryPlan, client domain.ClientBinding, outcome domain.ActivationOutcome, activationErr error) bool {
	return client.Authentication == domain.AuthenticationNotChecked && activationErr == nil &&
		!input.AuthComplete && !input.ActivationComplete && !outcome.AuthenticationAttested && !outcome.ActivationAttested &&
		outcome.Activation == domain.ActivationActive && outcome.Verification == domain.VerificationInstalled &&
		outcome.Policy == domain.PolicyAllowed && outcome.Authentication == domain.AuthenticationNotRequired &&
		service.clientVerifierAvailable(input, plan)
}
