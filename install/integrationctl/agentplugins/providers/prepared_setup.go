package providers

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

var (
	_ ports.PreparedSetupClassifier         = Activator{}
	_ ports.PreparedMaterializationVerifier = Activator{}
)

// PreparedSetupVerifierAvailable opts in only clients that own a setup retry
// and can observe this exact scope through their existing read-only verifier.
func (activator Activator) PreparedSetupVerifierAvailable(client domain.DetectedClient, plan domain.DeliveryPlan, executable string) bool {
	if activator.requireRegistry() != nil {
		return false
	}
	_, optedIn := clients.As[clients.PreparedSetupCompleter](activator.Registry, client.ClientID)
	return optedIn && activator.VerifierAvailable(client, plan, executable)
}

// CanCompletePreparedSetup delegates only to the selected client's opt-in.
func (activator Activator) CanCompletePreparedSetup(request domain.ActivationRequest, observed domain.ActivationOutcome) bool {
	if activator.requireRegistry() != nil {
		return false
	}
	classifier, ok := clients.As[clients.PreparedSetupCompleter](activator.Registry, request.Client.ClientID)
	return ok && classifier.CanCompletePreparedSetup(request, observed)
}

// PreparedMaterializationVerifierAvailable performs only registry membership.
func (activator Activator) PreparedMaterializationVerifierAvailable(client domain.DetectedClient) bool {
	if activator.requireRegistry() != nil {
		return false
	}
	_, ok := clients.As[clients.PreparedMaterializationVerifier](activator.Registry, client.ClientID)
	return ok
}

// PreparedMaterializationVerified never borrows setup-completion authority.
func (activator Activator) PreparedMaterializationVerified(request domain.ActivationRequest, observed domain.ActivationOutcome) bool {
	if activator.requireRegistry() != nil {
		return false
	}
	verifier, ok := clients.As[clients.PreparedMaterializationVerifier](activator.Registry, request.Client.ClientID)
	return ok && verifier.PreparedMaterializationVerified(request, observed)
}
