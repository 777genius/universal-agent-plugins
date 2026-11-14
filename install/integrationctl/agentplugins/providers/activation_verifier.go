package providers

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// VerifierAvailable reports whether this backend can observe the delivered plan
// with an exact client-side verifier. Every supported client answers through
// its adapter; a missing registry or a client without ReadOnlyVerifier is
// fail-closed.
func (activator Activator) VerifierAvailable(client domain.DetectedClient, plan domain.DeliveryPlan, backendExecutable string) bool {
	if activator.requireRegistry() != nil {
		return false
	}
	verifier, ok := clients.As[clients.ReadOnlyVerifier](activator.Registry, client.ClientID)
	if !ok {
		return false
	}
	return verifier.VerifierAvailable(client, plan, backendExecutable)
}
