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

var ErrListContractUnknown = errors.New("the Codex plugin list output is not recognized")

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
	for _, value := range entries {
		matched, active, err := inspectCodexListEntry(value, identities, expectedID)
		if err != nil {
			return StatusUnknown
		}
		if matched {
			foundExpected = true
			expectedActive = active
		}
	}
	if foundExpected && expectedActive {
		return StatusInstalled
	}
	return StatusAbsent
}

func inspectCodexListEntry(value any, identities map[string]struct{}, expectedID string) (bool, bool, error) {
	entry, ok := value.(map[string]any)
	if !ok {
		return false, false, errCodexListUnknown
	}
	required := []string{"pluginId", "name", "marketplaceName", "installed", "enabled"}
	for _, field := range required {
		if _, present := entry[field]; !present {
			return false, false, errCodexListUnknown
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
		return false, false, errCodexListUnknown
	}
	if _, duplicate := identities[pluginID]; duplicate {
		return false, false, errCodexListUnknown
	}
	identities[pluginID] = struct{}{}
	if pluginID == expectedID {
		return true, installed && enabled, nil
	}
	return false, false, nil
}

var errCodexListUnknown = errors.New("codex list entry is not recognized")
