package codex

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

func ParseRegistry(body []byte, name, expectedMarketplace string, owned bool) clients.RegistryFinding {
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	value, err := shared.DecodeUniqueJSONValue(decoder)
	if err != nil {
		return clients.RegistryIndeterminate
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return clients.RegistryIndeterminate
	}
	document, ok := value.(map[string]any)
	if !ok {
		return clients.RegistryIndeterminate
	}
	entries, ok := document["installed"].([]any)
	if !ok {
		return clients.RegistryIndeterminate
	}
	finding := clients.RegistryClear
	seen := map[string]bool{}
	for _, raw := range entries {
		entry, ok := raw.(map[string]any)
		if !ok {
			return clients.RegistryIndeterminate
		}
		entryName, nameOK := entry["name"].(string)
		marketplace, marketplaceOK := entry["marketplaceName"].(string)
		pluginID, idOK := entry["pluginId"].(string)
		_, installedOK := entry["installed"].(bool)
		_, enabledOK := entry["enabled"].(bool)
		if !nameOK || !marketplaceOK || !idOK || !installedOK || !enabledOK || entryName == "" || marketplace == "" || pluginID != entryName+"@"+marketplace || seen[pluginID] {
			return clients.RegistryIndeterminate
		}
		seen[pluginID] = true
		if entryName != name {
			continue
		}
		if marketplace == expectedMarketplace {
			if !owned {
				return clients.RegistryCollision
			}
			finding = clients.RegistryExpected
		}
		// A different non-empty marketplace is positive namespace evidence and
		// can coexist with the managed marketplace.
	}
	return finding
}
