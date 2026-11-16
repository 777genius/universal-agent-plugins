package clients

import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"

// PreparedSetupCompleter opts a client into finishing its own prepared setup
// through the existing confirmed repair path after a fresh read-only outcome.
// Classification is observational and grants no mutation authority itself.
type PreparedSetupCompleter interface {
	CanCompletePreparedSetup(domain.ActivationRequest, domain.ActivationOutcome) bool
}
