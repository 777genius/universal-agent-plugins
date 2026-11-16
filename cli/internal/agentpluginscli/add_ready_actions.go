package agentpluginscli

import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"

// Planning uncertainty is superseded by a successful native observation, while
// real activation guidance and explicit attestation remain independent.
func observedReadyActionResolved(result usecase.AddResult, action string) bool {
	if addFailureStatus(result, nil) != "" || !fullyInstalled(result.Activation) ||
		result.Activation.ActivationAttested || result.Activation.AuthenticationAttested {
		return false
	}
	return action == "" || action == "verify the plugin's authentication requirements before using it" ||
		action == "verify this plugin's authentication requirements before using it"
}
