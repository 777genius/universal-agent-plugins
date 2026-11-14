package installer

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func observedNativeAttempt(observed []ports.NativeTransitionObservation, installationID string, binding domain.ClientBinding) bool {
	found := false
	for _, item := range observed {
		if item.OperationID != binding.NativeActivationAttempt {
			continue
		}
		if found || item.Root == "" || item.Digest == "" || item.AuthorityDigest == "" ||
			item.InstallationID != installationID || item.BindingID != binding.ClientBindingID || item.TargetPath != binding.TargetLocator {
			return false
		}
		found = true
	}
	return found
}
