package shared

import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"

// NativeFileSnapshot retains the native kernel's bounded file observation.
type NativeFileSnapshot = nativeconfig.FileSnapshot

// DecodeCommentedJSONObject preserves strict identity checks after removing
// the comments accepted by native client configuration files.
func DecodeCommentedJSONObject(body []byte) (map[string]any, error) {
	standard, err := nativeconfig.StandardizeCommentedJSON(body)
	if err != nil {
		return nil, err
	}
	return DecodeStrictJSONObject(standard)
}
