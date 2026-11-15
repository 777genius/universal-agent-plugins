package installer

import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"

func (a seamActivator) CanCompletePreparedSetup(request domain.ActivationRequest, observed domain.ActivationOutcome) bool {
	return a.inner.CanCompletePreparedSetup(request, observed)
}
