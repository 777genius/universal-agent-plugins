package codex

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

type marketplaceDoc struct {
	Name      string           `json:"name,omitempty"`
	Interface map[string]any   `json:"interface,omitempty"`
	Plugins   []map[string]any `json:"plugins,omitempty"`
	Extra     map[string]any   `json:"-"`
}

type pluginConfigDoc struct {
	Plugins map[string]pluginConfigEntry `toml:"plugins"`
}

type pluginConfigEntry struct {
	Enabled *bool `toml:"enabled"`
}

type pluginConfigState struct {
	Present  bool
	Disabled bool
}

func policyFromEntry(entry map[string]any) *domain.CatalogPolicySnapshot {
	policy, _ := entry["policy"].(map[string]any)
	out := &domain.CatalogPolicySnapshot{}
	if value, ok := policy["installation"].(string); ok {
		out.Installation = strings.TrimSpace(value)
	}
	if value, ok := policy["authentication"].(string); ok {
		out.Authentication = strings.TrimSpace(value)
	}
	if value, ok := entry["category"].(string); ok {
		out.Category = strings.TrimSpace(value)
	}
	if out.Installation == "" && out.Authentication == "" && out.Category == "" {
		return nil
	}
	return out
}
