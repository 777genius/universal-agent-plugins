package installer

import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"

func (a seamActivator) PreparedSetupVerifierAvailable(client domain.DetectedClient, plan domain.DeliveryPlan, executable string) bool {
	return a.inner.PreparedSetupVerifierAvailable(client, plan, executable)
}

func (a seamActivator) CanCompletePreparedSetup(request domain.ActivationRequest, observed domain.ActivationOutcome) bool {
	return a.inner.CanCompletePreparedSetup(request, observed)
}

func (a seamActivator) PreparedMaterializationVerified(request domain.ActivationRequest, observed domain.ActivationOutcome) bool {
	return a.inner.PreparedMaterializationVerified(request, observed)
}

func (a seamActivator) PreparedMaterializationVerifierAvailable(client domain.DetectedClient) bool {
	return a.inner.PreparedMaterializationVerifierAvailable(client)
}
