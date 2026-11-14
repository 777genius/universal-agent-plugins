package shared

import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"

// ObjectMap indexes native objects by ObjectID so a mutation can look up the
// previous and desired record of the same object. Gemini, OpenCode, Cline and
// Kiro all need this; it cannot live in any one client package.
func ObjectMap(objects []domain.NativeObjectOwnership) map[string]domain.NativeObjectOwnership {
	result := make(map[string]domain.NativeObjectOwnership, len(objects))
	for _, object := range objects {
		result[object.ObjectID] = object
	}
	return result
}
