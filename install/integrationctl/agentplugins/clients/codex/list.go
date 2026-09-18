package codex

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

// ListStatus is the recognized shape of a Codex plugin listing.
type ListStatus int

const (
	StatusUnknown ListStatus = iota
	StatusInstalled
	StatusAbsent
)

// ErrListContractUnknown marks a listing whose shape this adapter does not
// recognize. Callers treat it as a manual verification, not a proof of absence.
var ErrListContractUnknown = errors.New("the Codex plugin list output is not recognized")

// PluginStatusFromList classifies a `plugin list --json` document for the
// exact installed-and-enabled managed identity.
func PluginStatusFromList(body []byte, name, marketplace string) ListStatus {
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
	return classifyCodexEntries(entries, name+"@"+marketplace)
}

func classifyCodexEntries(entries []any, expectedID string) ListStatus {
	identities := make(map[string]struct{}, len(entries))
	foundExpected := false
	expectedActive := false
	for _, value := range entries {
		pluginID, installed, enabled, ok := parseCodexEntry(value, identities)
		if !ok {
			return StatusUnknown
		}
		if pluginID != expectedID {
			continue
		}
		foundExpected = true
		expectedActive = installed && enabled
	}
	if foundExpected && expectedActive {
		return StatusInstalled
	}
	return StatusAbsent
}

func parseCodexEntry(value any, identities map[string]struct{}) (pluginID string, installed, enabled, ok bool) {
	entry, ok := value.(map[string]any)
	if !ok {
		return "", false, false, false
	}
	for _, field := range []string{"pluginId", "name", "marketplaceName", "installed", "enabled"} {
		if _, present := entry[field]; !present {
			return "", false, false, false
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
		return "", false, false, false
	}
	if _, duplicate := identities[pluginID]; duplicate {
		return "", false, false, false
	}
	identities[pluginID] = struct{}{}
	return pluginID, installed, enabled, true
}
