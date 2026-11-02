package agentpluginscli

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// Carry the host reviewed before consent into the same operation's apply.
// A fresh operation may prepare new evidence; confirmed apply must revalidate
// the reviewed snapshot instead of accepting a newly selected version.
func retainPlannedClient(input *usecase.AddInput, plan domain.DeliveryPlan) {
	input.Client.OpenCodeHost = plan.OpenCodeHost
	input.Client.ProfileAuthority = plan.ProfileAuthority()
	input.Client.ProfileNamespace = plan.ProfileNamespace()
	input.OnlinePreview = false
}

func retainPreparedClients(inputs []usecase.AddInput, clients []domain.DetectedClient) {
	for i := range inputs {
		for _, client := range clients {
			if client.ClientID != inputs[i].Client.ClientID || client.ExecutablePath != inputs[i].Client.ExecutablePath {
				continue
			}
			// Keep the selected root spelling so apply must verify its
			// relationship to the frozen canonical profile, including symlinks.
			inputs[i].Client.OpenCodeHost = client.OpenCodeHost
			inputs[i].Client.ProfileAuthority = domain.CloneProfileAuthority(client.ProfileAuthority)
			inputs[i].Client.ProfileNamespace = client.ProfileNamespace
			inputs[i].OnlinePreview = false
			break
		}
	}
}
