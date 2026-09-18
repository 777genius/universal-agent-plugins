package claude

import (
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

// ListStatus is the recognized shape of a Claude Code plugin listing.
type ListStatus int

const (
	StatusUnknown ListStatus = iota
	StatusInstalled
	StatusAbsent
	StatusCollision
)

// PluginStatusFromList classifies a `plugin list --json` document for the
// exact managed @skills-dir identity.
func PluginStatusFromList(body []byte, name, activePath string) ListStatus {
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
	return classifyClaudeEntries(entries, name+"@skills-dir", filepath.Clean(activePath))
}

func classifyClaudeEntries(entries []any, expectedID, expectedPath string) ListStatus {
	seen := map[string]struct{}{}
	found := false
	for _, value := range entries {
		status, matched, ok := classifyClaudeEntry(value, expectedID, expectedPath, seen)
		if !ok {
			return status
		}
		if !matched {
			continue
		}
		if found {
			return StatusUnknown
		}
		found = true
	}
	if found {
		return StatusInstalled
	}
	return StatusAbsent
}

func classifyClaudeEntry(value any, expectedID, expectedPath string, seen map[string]struct{}) (ListStatus, bool, bool) {
	entry, ok := value.(map[string]any)
	if !ok {
		return StatusUnknown, false, false
	}
	id, idOK := entry["id"].(string)
	scope, scopeOK := entry["scope"].(string)
	enabled, enabledOK := entry["enabled"].(bool)
	installPath, pathOK := entry["installPath"].(string)
	if !idOK || !scopeOK || !enabledOK || !pathOK || id == "" || scope == "" || !filepath.IsAbs(installPath) {
		return StatusUnknown, false, false
	}
	identity := id + "\x00" + scope + "\x00" + filepath.Clean(installPath)
	if _, duplicate := seen[identity]; duplicate {
		return StatusUnknown, false, false
	}
	seen[identity] = struct{}{}
	if id != expectedID {
		return StatusUnknown, false, true
	}
	if filepath.Clean(installPath) != expectedPath || scope != "user" {
		return StatusCollision, false, false
	}
	return StatusUnknown, enabled, true
}
