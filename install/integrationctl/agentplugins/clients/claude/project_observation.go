package claude

import (
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

// Project discovery and scoped filesystem ownership are different observations.
// In particular, suppression does not identify any particular installed package.
type projectListState uint8

const (
	projectListUnknown projectListState = iota
	projectListAbsent
	projectListEnabled
	projectListDisabled
	projectListSuppressed
	projectListOtherSource
	projectListForeign
)

type projectListObservation struct {
	state       projectListState
	untrusted   bool
	otherSource bool
}

const claudeUntrustedProjectNote = "1 project-scope directory under ./.claude/skills/ that may load as a plugin was skipped because this workspace was not trusted when plugins were scanned. After accepting the trust dialog, run /reload-plugins (or relaunch) to load what qualifies."

func observeClaudeProjectList(body []byte, name, active, project, user string) projectListObservation {
	unknown := projectListObservation{}
	if len(body) > 1<<20 || name == "" || !exactAbsolute(active) || !exactAbsolute(project) || !exactAbsolute(user) || filepath.Dir(active) != filepath.Join(project, ".claude", "skills") {
		return unknown
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	parsed, err := shared.DecodeUniqueJSONValue(decoder)
	if err != nil {
		return unknown
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return unknown
	}
	entries, ok := parsed.([]any)
	if !ok {
		return unknown
	}
	result := projectListObservation{state: projectListAbsent}
	seen := map[string]bool{}
	selected, other, suppressed, foreign := false, false, false, false
	for _, value := range entries {
		entry, ok := value.(map[string]any)
		if !ok {
			return unknown
		}
		id, ok := entry["id"].(string)
		if !ok || id == "" {
			return unknown
		}
		if id == "(suppressed)@skills-dir" {
			trust, valid := projectSuppression(entry)
			if !valid || suppressed {
				return unknown
			}
			suppressed, result.untrusted = true, trust
			continue
		}
		scope, scopeOK := entry["scope"].(string)
		enabled, enabledOK := entry["enabled"].(bool)
		path, pathOK := entry["installPath"].(string)
		if !scopeOK || !enabledOK || !pathOK || !exactAbsolute(path) || !knownClaudeScope(scope) || !validProjectListMetadata(entry, project) {
			return unknown
		}
		identity := id + "\x00" + scope + "\x00" + path
		if seen[identity] {
			return unknown
		}
		seen[identity] = true
		if id != name+"@skills-dir" {
			continue
		}
		switch scope {
		case "user":
			if filepath.Dir(path) != filepath.Join(user, "skills") || other {
				return unknown
			}
			other = true
		case "project":
			if filepath.Dir(path) != filepath.Join(project, ".claude", "skills") {
				return unknown
			}
			if path != active {
				foreign = true
				continue
			}
			if selected {
				return unknown
			}
			selected = true
			if reported, exists := entry["errors"]; exists && len(reported.([]any)) != 0 {
				return unknown
			}
			if enabled {
				result.state = projectListEnabled
			} else {
				result.state = projectListDisabled
			}
		default:
			// Skills-directory plugins have only the observed user/project origins.
			return unknown
		}
	}
	if selected && (suppressed || other || foreign) {
		return unknown
	}
	if foreign {
		result.state = projectListForeign
		return result
	}
	if suppressed {
		result.state, result.otherSource = projectListSuppressed, other
		return result
	}
	if other {
		result.state, result.otherSource = projectListOtherSource, true
	}
	return result
}

func exactAbsolute(path string) bool { return filepath.IsAbs(path) && filepath.Clean(path) == path }

func knownClaudeScope(scope string) bool {
	switch scope {
	case "user", "project", "local", "managed":
		return true
	}
	return false
}

func boundedProjectStrings(value any, required bool) bool {
	values, ok := value.([]any)
	if !ok || len(values) > 64 || required && len(values) == 0 {
		return false
	}
	total := 0
	for _, item := range values {
		text, ok := item.(string)
		if !ok || strings.TrimSpace(text) == "" || len(text) > 4096 {
			return false
		}
		total += len(text)
	}
	return total <= 16384
}

func validProjectListMetadata(entry map[string]any, project string) bool {
	if version, exists := entry["version"]; exists {
		if text, ok := version.(string); !ok || text == "" {
			return false
		}
	}
	if path, exists := entry["projectPath"]; exists {
		if text, ok := path.(string); !ok || !exactAbsolute(text) || text != project {
			return false
		}
	}
	for _, key := range []string{"errors", "notes"} {
		if value, exists := entry[key]; exists && !boundedProjectStrings(value, false) {
			return false
		}
	}
	return true
}

func projectSuppression(entry map[string]any) (bool, bool) {
	if len(entry) != 6 || entry["version"] != "unknown" || entry["scope"] != "project" || entry["enabled"] != false || entry["installPath"] != "" || !boundedProjectStrings(entry["notes"], true) {
		return false, false
	}
	trust := false
	for _, note := range entry["notes"].([]any) {
		if note == claudeUntrustedProjectNote {
			trust = true
		}
	}
	return trust, true
}

func (o projectListObservation) action() string {
	switch o.state {
	case projectListSuppressed:
		if o.untrusted {
			return "Claude Code skipped project plugins because the selected folder is not trusted; review its trust dialog yourself, then start Claude Code from the project root or run /reload-plugins and retry verification"
		}
		return "Claude Code suppressed project plugin discovery; inspect /plugin in the selected project and retry verification from its root"
	case projectListDisabled:
		return "the exact Claude Code project plugin is disabled; review its existing enablement settings and retry verification from the project root"
	case projectListOtherSource:
		return "Claude Code lists a same-name user plugin instead of the selected project copy; preserve that installation and review native name precedence before retrying project verification"
	}
	return "start Claude Code from the selected project root and retry exact project plugin verification"
}
