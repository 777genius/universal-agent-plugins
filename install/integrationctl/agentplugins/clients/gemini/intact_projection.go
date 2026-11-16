package gemini

import (
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Exact intact native objects need health/trust completion, not replacement.
// Missing or changed objects retain the ordinary guarded repair path.
func intactGeminiProjectProjection(env clients.Env, request domain.ActivationRequest, root string) bool {
	if request.VerifyOnly || !request.Replacing || request.Plan.Scope != domain.ScopeProject {
		return false
	}
	previous, desired := GeminiObjects(request.PreviousNativeObjects), GeminiObjects(request.Delivery.NativeObjects)
	return len(previous) > 0 && reflect.DeepEqual(previous, desired) && VerifyGeminiNativeObjects(root, desired, false, env.NativeConfig) == nil
}
