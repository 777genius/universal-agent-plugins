package gemini

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Exact intact native objects need health/trust completion, not replacement.
// Missing or changed objects retain the ordinary guarded repair path.
func intactGeminiProjectProjection(env clients.Env, request domain.ActivationRequest, root string) bool {
	desired := GeminiObjects(request.Delivery.NativeObjects)
	return request.Plan.Scope == domain.ScopeProject && shared.IntactNativeReplacement(request, GeminiObjects(request.PreviousNativeObjects), desired, func() error {
		return VerifyGeminiNativeObjects(root, desired, false, env.NativeConfig)
	})
}
