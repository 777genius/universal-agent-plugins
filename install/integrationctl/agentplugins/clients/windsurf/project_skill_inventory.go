package windsurf

import (
	"encoding/json"
	"io"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

func devinProjectSkillsObserved(body string, expected map[string]devinSkillMetadata) bool {
	if len(body) > 1<<20 || len(expected) == 0 {
		return false
	}
	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.UseNumber()
	value, err := shared.DecodeUniqueJSONValue(decoder)
	if err != nil {
		return false
	}
	if _, err := decoder.Token(); err != io.EOF {
		return false
	}
	rows, ok := value.([]any)
	if !ok || len(rows) > 256 {
		return false
	}
	ownedPaths := make(map[string]bool, len(expected))
	for _, skill := range expected {
		ownedPaths[skill.path] = true
	}
	seen, found := map[string]bool{}, 0
	for _, value := range rows {
		row, ok := value.(map[string]any)
		if !ok || !devinSkillRow(row) {
			return false
		}
		name := row["name"].(string)
		base := row["base_dir"].(string)
		key := shared.FoldJSONKey(name)
		if seen[key] {
			return false
		}
		seen[key] = true
		if skill, selected := expected[name]; selected {
			if row["provider"] != "Devin" || base != skill.path || row["description"] != skill.description || row["display_name"] != name || !devinSkillTriggers(row["triggers"], true) {
				return false
			}
			found++
		} else if ownedPaths[base] {
			return false
		}
	}
	return found == len(expected)
}

func devinSkillFields(row map[string]any, fields ...string) bool {
	if len(row) != len(fields) {
		return false
	}
	for _, name := range fields {
		if _, exists := row[name]; !exists {
			return false
		}
	}
	return true
}

func devinEmptyArray(value any) bool {
	rows, ok := value.([]any)
	return ok && len(rows) == 0
}

func devinSkillTriggers(value any, selected bool) bool {
	rows, ok := value.([]any)
	if !ok || len(rows) < 1 || len(rows) > 2 || selected && len(rows) != 2 {
		return false
	}
	seen := map[any]bool{}
	for _, trigger := range rows {
		if trigger != "user" && trigger != "model" || seen[trigger] {
			return false
		}
		seen[trigger] = true
	}
	return true
}

func devinSkillPolicyFiles(env clients.Env, project, global string) (map[string]shared.NativeFileSnapshot, bool, error) {
	files := map[string]shared.NativeFileSnapshot{}
	roots := []string{project}
	if global != project {
		roots = append(roots, global)
	}
	for _, root := range roots {
		for _, name := range []string{"config.json", "config.local.json", "mcp_config.json", "mcp_config.local.json"} {
			path := filepath.Join(root, name)
			file, err := env.NativeConfig.ReadExactFileBounded(path, 1<<20)
			if err != nil {
				return nil, false, err
			}
			files[path] = file
			if !file.Exists || strings.HasPrefix(name, "mcp_") {
				continue
			}
			document, err := shared.DecodeCommentedJSONObject(file.Body)
			if err != nil {
				return nil, false, err
			}
			if !devinSkillPolicyKnown(document) {
				return files, false, nil
			}
		}
	}
	return files, true, nil
}

// A read-only catalog must not launch the pinned client's initial migration.
func devinSkillPolicyVersioned(file shared.NativeFileSnapshot) bool {
	if !file.Exists {
		return false
	}
	document, err := shared.DecodeCommentedJSONObject(file.Body)
	return err == nil && document["version"] == json.Number("1")
}

func devinSkillPolicyStable(before, after map[string]shared.NativeFileSnapshot, global string) bool {
	if len(before) != len(after) {
		return false
	}
	for path, previous := range before {
		current, exists := after[path]
		if !exists {
			return false
		}
		if reflect.DeepEqual(previous, current) {
			continue
		}
		if path != filepath.Join(global, "config.json") || !devinInitialConfigMigration(previous, current) {
			return false
		}
	}
	return true
}

// Pinned Devin 3000.11.205 adds version:1 on its first ordinary metadata call.
// Only that native schema marker may change; no policy or other config is
// accepted as a new baseline, and the installer never writes this migration.
func devinInitialConfigMigration(before, after shared.NativeFileSnapshot) bool {
	if !devinConfigMigrationMode(before, after) {
		return false
	}
	old := map[string]any{}
	if before.Exists {
		var err error
		old, err = shared.DecodeCommentedJSONObject(before.Body)
		if err != nil || !devinSkillPolicyKnown(old) {
			return false
		}
	}
	if _, exists := old["version"]; exists {
		return false
	}
	current, err := shared.DecodeCommentedJSONObject(after.Body)
	if err != nil || current["version"] != json.Number("1") {
		return false
	}
	delete(current, "version")
	return reflect.DeepEqual(old, current)
}

func devinConfigMigrationMode(before, after shared.NativeFileSnapshot) bool {
	// The pinned writer creates/replaces config with 0600. Windows exposes
	// an ordinary writable file as 0666 through FileMode, without Unix ACLs.
	if !after.Exists || after.Mode != 0600 && (runtime.GOOS != "windows" || after.Mode != 0666) {
		return false
	}
	if !before.Exists {
		// The bounded kernel returns 0600 as a creation-mode hint for an
		// absent file. There are no original permission bits to compare.
		return len(before.Body) == 0
	}
	return before.Mode == after.Mode || before.Mode == 0644 && after.Mode == 0600
}

func devinSkillPolicyKnown(document map[string]any) bool {
	for key, value := range document {
		switch key {
		case "version":
			if value != json.Number("1") {
				return false
			}
		case "auto_update":
			if value != false {
				return false
			}
		case "mcpServers":
			entries, ok := value.(map[string]any)
			if !ok || len(entries) != 0 {
				return false
			}
		case "permissions":
			// The existing native fixture uses this exact deny-all execution
			// profile. It still exposes inert Skills metadata without a login.
			permissions, ok := value.(map[string]any)
			deny := []any{"read", "edit", "write", "grep", "glob", "exec", "fetch", "web_fetch", "mcp__*"}
			if !ok || !devinSkillFields(permissions, "allow", "deny", "ask") ||
				!devinEmptyArray(permissions["allow"]) || !devinEmptyArray(permissions["ask"]) || !reflect.DeepEqual(permissions["deny"], deny) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func devinSkillRow(row map[string]any) bool {
	if !devinSkillFields(row, "name", "description", "provider", "base_dir", "display_name", "triggers", "warnings", "errors") {
		return false
	}
	name, named := row["name"].(string)
	base, located := row["base_dir"].(string)
	_, described := row["description"].(string)
	display, displayed := row["display_name"].(string)
	if !named || name == "" || !located || !described || !displayed || display == "" || !devinEmptyArray(row["warnings"]) || !devinEmptyArray(row["errors"]) || !devinSkillTriggers(row["triggers"], false) {
		return false
	}
	return devinSkillOrigin(row["provider"], base)
}

func devinSkillOrigin(provider any, base string) bool {
	if provider != "Builtin" && provider != "Devin" && provider != "Windsurf" && provider != "Claude" && provider != "Copilot" {
		return false
	}
	return base == "" && provider == "Builtin" || base != "" && filepath.IsAbs(base) && filepath.Clean(base) == base
}
