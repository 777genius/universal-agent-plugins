package codex

import (
	"encoding/json"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (d *marketplaceDoc) UnmarshalJSON(body []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return err
	}
	d.Extra = map[string]any{}
	if value, ok := raw["name"].(string); ok {
		d.Name = strings.TrimSpace(value)
	}
	if value, ok := raw["interface"].(map[string]any); ok {
		d.Interface = value
	}
	if items, ok := raw["plugins"].([]any); ok {
		d.Plugins = make([]map[string]any, 0, len(items))
		for _, item := range items {
			doc, ok := item.(map[string]any)
			if !ok {
				return domain.NewError(domain.ErrMutationApply, "Codex marketplace plugins entries must be JSON objects", nil)
			}
			d.Plugins = append(d.Plugins, doc)
		}
	}
	delete(raw, "name")
	delete(raw, "interface")
	delete(raw, "plugins")
	for key, value := range raw {
		d.Extra[key] = value
	}
	return nil
}

func (d marketplaceDoc) MarshalJSON() ([]byte, error) {
	raw := map[string]any{}
	for key, value := range d.Extra {
		raw[key] = value
	}
	if strings.TrimSpace(d.Name) != "" {
		raw["name"] = d.Name
	}
	if len(d.Interface) > 0 {
		raw["interface"] = d.Interface
	}
	raw["plugins"] = d.Plugins
	return json.Marshal(raw)
}
