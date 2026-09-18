package codex

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

type Status int

const (
	StatusUnknown Status = iota
	StatusInstalled
	StatusAbsent
)

var ErrListContractUnknown = errors.New("Codex plugin list output is not recognized")

func PluginStatus(body []byte, name, marketplace string) Status {
	if len(body) == 0 {
		return StatusUnknown
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	parsed, err := shared.DecodeUniqueJSONValue(decoder)
	if err != nil {
		return StatusUnknown
	}
	if _, tokenErr := decoder.Token(); tokenErr == nil || !errors.Is(tokenErr, io.EOF) {
		return StatusUnknown
	}
	document, ok := parsed.(map[string]any)
	if !ok {
		return StatusUnknown
	}
	installedValue, ok := document["installed"]
	if !ok {
		return StatusUnknown
	}
	entries, ok := installedValue.([]any)
	if !ok {
		return StatusUnknown
	}
	expectedID := name + "@" + marketplace
	identities := make(map[string]struct{}, len(entries))
	foundExpected := false
	expectedActive := false
	required := []string{"pluginId", "name", "marketplaceName", "installed", "enabled"}
	for _, value := range entries {
		entry, ok := value.(map[string]any)
		if !ok {
			return StatusUnknown
		}
		for _, field := range required {
			if _, present := entry[field]; !present {
				return StatusUnknown
			}
		}
		pluginID, pluginIDOK := entry["pluginId"].(string)
		entryName, nameOK := entry["name"].(string)
		marketplaceName, marketplaceOK := entry["marketplaceName"].(string)
		installed, installedOK := entry["installed"].(bool)
		enabled, enabledOK := entry["enabled"].(bool)
		if !pluginIDOK || !nameOK || !marketplaceOK || !installedOK || !enabledOK ||
			pluginID == "" || entryName == "" || marketplaceName == "" ||
			pluginID != entryName+"@"+marketplaceName {
			return StatusUnknown
		}
		if _, duplicate := identities[pluginID]; duplicate {
			return StatusUnknown
		}
		identities[pluginID] = struct{}{}
		if pluginID == expectedID {
			foundExpected = true
			expectedActive = installed && enabled
		}
	}
	if foundExpected && expectedActive {
		return StatusInstalled
	}
	return StatusAbsent
}
