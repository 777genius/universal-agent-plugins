package grok

import (
	"encoding/json"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

// This additional 1.0.70 contract is used only for Skill completion. Legacy
// source preparation still uses the existing, separately qualified parser.
func projectSkillsObserved(native projectInspection, proof *projectSkillProof) bool {
	document := native.document
	if native.version != "1.0.70" || !projectSkillReportFields(document) || document["projectTrusted"] != true {
		return false
	}
	mcp, ok := document["mcpServers"].([]any)
	if !ok || len(mcp) != 0 || !projectSkillPluginObserved(document, proof) {
		return false
	}
	rows, ok := document["skills"].([]any)
	if !ok {
		return false
	}
	return projectSkillRowsObserved(rows, proof)
}

// projectSkillReportFields admits only the exact observed 1.0.70 inspect
// envelope. Optional diagnostic families are never activation authority; the
// selected plugin, trust, Skill origin and bytes are verified independently.
func projectSkillReportFields(document map[string]any) bool {
	required := []string{"grokVersion", "cwd", "projectRoot", "projectTrusted", "plugins", "skills", "mcpServers"}
	for _, key := range required {
		if _, ok := document[key]; !ok {
			return false
		}
	}
	for key, value := range document {
		switch key {
		case "grokVersion", "cwd", "projectRoot", "projectTrusted", "plugins", "skills", "mcpServers":
			continue
		case "channel":
			if _, ok := value.(string); !ok {
				return false
			}
		case "updateChannel":
			if value != nil {
				if _, ok := value.(string); !ok {
					return false
				}
			}
		case "projectInstructions", "hooks", "agents", "marketplaces", "lspServers", "experiments":
			if _, ok := value.([]any); !ok {
				return false
			}
		case "permissions", "loginPolicy", "configSources", "externalCompat":
			if _, ok := value.(map[string]any); !ok {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func projectSkillRowsObserved(rows []any, proof *projectSkillProof) bool {
	found, seen := 0, map[string]bool{}
	for _, value := range rows {
		row, valid := value.(map[string]any)
		name, named := row["name"].(string)
		key := shared.FoldJSONKey(name)
		if !valid || !named || name == "" || seen[key] || !projectSkillEntry(row) {
			return false
		}
		seen[key] = true
		skill, selected := proof.skills[name]
		if selected {
			if !projectSkillObserved(row, proof.name, skill) {
				return false
			}
			found++
		} else if source, valid := row["source"].(map[string]any); valid {
			if name, present := source["plugin_name"].(string); present &&
				shared.FoldJSONKey(name) == shared.FoldJSONKey(proof.name) {
				return false
			}
		}
	}
	return found == len(proof.skills)
}

func projectSkillEntry(row map[string]any) bool {
	fields := []string{"name", "description", "source", "userInvocable"}
	if disabled, exists := row["disabled"]; exists {
		if _, ok := disabled.(bool); !ok {
			return false
		}
		fields = append(fields, "disabled")
	}
	// Grok reports unrelated native Skills alongside plugin Skills.
	// Keep only the native optional inspect fields and their exact types.
	if vendor, present := row["vendor"]; present {
		if value, ok := vendor.(string); !ok || value == "" {
			return false
		}
		fields = append(fields, "vendor")
	}
	if status, present := row["compatibilityStatus"]; present {
		if status != "enabled" && status != "disabled" {
			return false
		}
		fields = append(fields, "compatibilityStatus")
	}
	if collision, present := row["collidesWith"]; present {
		if value, ok := collision.(string); !ok || value == "" {
			return false
		}
		fields = append(fields, "collidesWith")
	}
	if invoked, present := row["invocableAs"]; present {
		if value, ok := invoked.(string); !ok || value == "" {
			return false
		}
		fields = append(fields, "invocableAs")
	}
	if !projectSkillFields(row, fields...) {
		return false
	}
	if _, ok := row["description"].(string); !ok {
		return false
	}
	if _, ok := row["userInvocable"].(bool); !ok {
		return false
	}
	source, ok := row["source"].(map[string]any)
	return ok && projectSkillSource(source)
}

func projectSkillSource(source map[string]any) bool {
	path, located := source["path"].(string)
	if !located || path == "" {
		return false
	}
	switch source["type"] {
	case "plugin":
		if !projectSkillFields(source, "type", "plugin_name", "path") {
			return false
		}
		name, ok := source["plugin_name"].(string)
		return ok && name != ""
	case "project", "user", "bundled", "configToml", "server":
		// Foreign native Skills are inventory, not evidence of an owned
		// plugin. The selected Skill still needs exact plugin-origin proof.
		return projectSkillFields(source, "type", "path")
	default:
		return false
	}
}

func projectSkillPluginObserved(document map[string]any, proof *projectSkillProof) bool {
	rows, ok := document["plugins"].([]any)
	if !ok {
		return false
	}
	for _, value := range rows {
		row, valid := value.(map[string]any)
		if !valid || row["name"] != proof.name {
			continue
		}
		if !projectSkillFields(row, "name", "scope", "path", "enabled", "provides") ||
			row["scope"] != "project" || row["path"] != proof.root || row["enabled"] != true {
			return false
		}
		provides, valid := row["provides"].(map[string]any)
		return valid && projectSkillFields(provides, "skills", "agents", "hooks", "mcpServers") &&
			projectSkillCount(provides["skills"], len(proof.skills)) && projectSkillCount(provides["agents"], 0) &&
			provides["hooks"] == false && projectSkillCount(provides["mcpServers"], 0)
	}
	return false
}

func projectSkillObserved(row map[string]any, plugin string, skill projectSkillMetadata) bool {
	fields := []string{"name", "description", "source", "userInvocable"}
	if disabled, exists := row["disabled"]; exists {
		if disabled != false {
			return false
		}
		fields = append(fields, "disabled")
	}
	if !projectSkillFields(row, fields...) || row["description"] != skill.description || row["userInvocable"] != true {
		return false
	}
	source, ok := row["source"].(map[string]any)
	return ok && projectSkillFields(source, "type", "plugin_name", "path") && source["type"] == "plugin" &&
		source["plugin_name"] == plugin &&
		func() bool {
			path, ok := source["path"].(string)
			return ok && filepath.IsAbs(path) && shared.SameCleanPath(path, skill.path)
		}()
}

func projectSkillFields(object map[string]any, fields ...string) bool {
	if len(object) != len(fields) {
		return false
	}
	for _, field := range fields {
		if _, exists := object[field]; !exists {
			return false
		}
	}
	return true
}

func projectSkillCount(value any, expected int) bool {
	number, ok := value.(json.Number)
	if !ok {
		return false
	}
	count, err := number.Int64()
	return err == nil && count == int64(expected)
}
