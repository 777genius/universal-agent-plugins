package platformexec

import (
	"encoding/json"

	"github.com/tailscale/hujson"
)

func decodeImportedOpenCodeConfigRaw(body []byte) (map[string]any, error) {
	body, err := hujson.Standardize(body)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}
