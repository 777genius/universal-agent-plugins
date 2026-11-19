package grok

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.PreparedSetupCompleter = (*Adapter)(nil)

// Membership freshly reobserves saved Project Skills through the narrow
// verifier, and User registrations through the existing bounded plugin list.
// This also refreshes converged User repeats and online previews. Grok owns no
// automatic setup completion: false prevents an acting trust/enablement repair.
func (*Adapter) CanCompletePreparedSetup(domain.ActivationRequest, domain.ActivationOutcome) bool {
	return false
}
