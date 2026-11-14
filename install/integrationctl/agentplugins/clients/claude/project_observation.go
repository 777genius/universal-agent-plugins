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

type projectListQuery struct {
	name, active, project, user string
}

func (q projectListQuery) valid(body []byte) bool {
	return len(body) <= 1<<20 && q.name != "" && exactAbsolute(q.active) && exactAbsolute(q.project) && exactAbsolute(q.user) && filepath.Dir(q.active) == filepath.Join(q.project, ".claude", "skills")
}

type projectListRow struct {
	scope, path string
	enabled     bool
}

type projectListScan struct {
	query                                projectListQuery
	observation                          projectListObservation
	seen                                 map[string]bool
	selected, other, suppressed, foreign bool
}

func observeClaudeProjectList(body []byte, name, active, project, user string) projectListObservation {
	query := projectListQuery{name: name, active: active, project: project, user: user}
	if !query.valid(body) {
		return projectListObservation{}
	}
	entries, ok := decodeProjectList(body)
	if !ok {
		return projectListObservation{}
	}
	scan := projectListScan{query: query, observation: projectListObservation{state: projectListAbsent}, seen: map[string]bool{}}
	for _, value := range entries {
		entry, ok := value.(map[string]any)
		if !ok || !scan.record(entry) {
			return projectListObservation{}
		}
	}
	return scan.result()
}

func decodeProjectList(body []byte) ([]any, bool) {
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	parsed, err := shared.DecodeUniqueJSONValue(decoder)
	if err != nil {
		return nil, false
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, false
	}
	entries, ok := parsed.([]any)
	return entries, ok
}

func (s *projectListScan) record(entry map[string]any) bool {
	id, ok := entry["id"].(string)
	if !ok || id == "" {
		return false
	}
	if id == "(suppressed)@skills-dir" {
		trust, valid := projectSuppression(entry)
		if !valid || s.suppressed {
			return false
		}
		s.suppressed, s.observation.untrusted = true, trust
		return true
	}
	row, valid := parseProjectListRow(entry, s.query.project)
	if !valid {
		return false
	}
	identity := id + "\x00" + row.scope + "\x00" + row.path
	if s.seen[identity] {
		return false
	}
	s.seen[identity] = true
	if id != s.query.name+"@skills-dir" {
		return true
	}
	return s.recordCandidate(entry, row)
}

func parseProjectListRow(entry map[string]any, project string) (projectListRow, bool) {
	scope, scopeOK := entry["scope"].(string)
	enabled, enabledOK := entry["enabled"].(bool)
	path, pathOK := entry["installPath"].(string)
	if !scopeOK || !enabledOK || !pathOK || !exactAbsolute(path) || !knownClaudeScope(scope) || !validProjectListMetadata(entry, project) {
		return projectListRow{}, false
	}
	return projectListRow{scope: scope, path: path, enabled: enabled}, true
}

func (s *projectListScan) recordCandidate(entry map[string]any, row projectListRow) bool {
	switch row.scope {
	case "user":
		if filepath.Dir(row.path) != filepath.Join(s.query.user, "skills") || s.other {
			return false
		}
		s.other = true
	case "project":
		if filepath.Dir(row.path) != filepath.Join(s.query.project, ".claude", "skills") {
			return false
		}
		if row.path != s.query.active {
			s.foreign = true
			return true
		}
		if s.selected {
			return false
		}
		s.selected = true
		for _, key := range []string{"errors", "notes"} {
			if reported, exists := entry[key]; exists && len(reported.([]any)) != 0 {
				return false
			}
		}
		if row.enabled {
			s.observation.state = projectListEnabled
		} else {
			s.observation.state = projectListDisabled
		}
	default:
		// Skills-directory plugins have only the observed user/project origins.
		return false
	}
	return true
}

func (s *projectListScan) result() projectListObservation {
	if s.selected && (s.suppressed || s.other || s.foreign) {
		return projectListObservation{}
	}
	result := s.observation
	if s.foreign {
		result.state = projectListForeign
		return result
	}
	if s.suppressed {
		result.state, result.otherSource = projectListSuppressed, s.other
		return result
	}
	if s.other {
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
