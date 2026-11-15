package installer

import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"

func (a seamActivator) PreparedSetupVerifierAvailable(client domain.DetectedClient, plan domain.DeliveryPlan, executable string) bool {
	return a.inner.PreparedSetupVerifierAvailable(client, plan, executable)
}

func (a seamActivator) CanCompletePreparedSetup(request domain.ActivationRequest, observed domain.ActivationOutcome) bool {
	return a.inner.CanCompletePreparedSetup(request, observed)
}
