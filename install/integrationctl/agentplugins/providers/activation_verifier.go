package providers

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// VerifierAvailable reports whether this backend can observe the delivered plan
// with an exact client-side verifier. Registry-backed clients answer through
// the adapter; native-config clients stay in the leftover switch until 7c.
func (activator Activator) VerifierAvailable(client domain.DetectedClient, plan domain.DeliveryPlan, backendExecutable string) bool {
	if activator.requireRegistry() != nil {
		return false
	}
	if verifier, ok := clients.As[clients.ReadOnlyVerifier](activator.Registry, client.ClientID); ok {
		return verifier.VerifierAvailable(client, plan, backendExecutable)
	}
	switch client.ClientID {
	case domain.ClientGemini, domain.ClientOpenCode, domain.ClientCline, domain.ClientWindsurf:
		// These clients expose an exact, read-only native configuration verifier.
		// It is intentionally independent of an optional client executable.
		return len(plan.Components) > 0
	}
	return false
}
