package claude

import (
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

type Status int

const (
	StatusUnknown Status = iota
	StatusInstalled
	StatusAbsent
	StatusCollision
)

func PluginStatus(body []byte, name, activePath string) Status {
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	parsed, err := shared.DecodeUniqueJSONValue(decoder)
	if err != nil {
		return StatusUnknown
	}
	if _, tokenErr := decoder.Token(); !errors.Is(tokenErr, io.EOF) {
		return StatusUnknown
	}
	entries, ok := parsed.([]any)
	if !ok {
		return StatusUnknown
	}
	expectedID := name + "@skills-dir"
	expectedPath := filepath.Clean(activePath)
	seen := map[string]struct{}{}
	found := false
	for _, value := range entries {
		entry, ok := value.(map[string]any)
		if !ok {
			return StatusUnknown
		}
		id, idOK := entry["id"].(string)
		scope, scopeOK := entry["scope"].(string)
		enabled, enabledOK := entry["enabled"].(bool)
		installPath, pathOK := entry["installPath"].(string)
		if !idOK || !scopeOK || !enabledOK || !pathOK || id == "" || scope == "" || !filepath.IsAbs(installPath) {
			return StatusUnknown
		}
		identity := id + "\x00" + scope + "\x00" + filepath.Clean(installPath)
		if _, duplicate := seen[identity]; duplicate {
			return StatusUnknown
		}
		seen[identity] = struct{}{}
		if id != expectedID {
			continue
		}
		if filepath.Clean(installPath) != expectedPath || scope != "user" {
			return StatusCollision
		}
		if found {
			return StatusUnknown
		}
		found = enabled
	}
	if found {
		return StatusInstalled
	}
	return StatusAbsent
}
