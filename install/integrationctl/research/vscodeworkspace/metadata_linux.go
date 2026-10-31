//go:build linux

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func objectKeys(value any, required, optional []string) (map[string]any, error) {
	m, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("metadata object required")
	}
	allowed := map[string]bool{}
	for _, key := range required {
		if _, ok := m[key]; !ok {
			return nil, fmt.Errorf("required metadata field missing")
		}
		allowed[key] = true
	}
	for _, key := range optional {
		allowed[key] = true
	}
	for key := range m {
		if !allowed[key] {
			return nil, fmt.Errorf("unknown metadata field %q", key)
		}
	}
	return m, nil
}
func metadataText(value any) error {
	s, ok := value.(string)
	if !ok || len(s) > 8192 || !utf8.ValidString(s) || strings.ContainsRune(s, 0) {
		return fmt.Errorf("metadata string invalid or oversized")
	}
	return nil
}
func metadataStrings(value any) error {
	rows, ok := value.([]any)
	if !ok || len(rows) > 128 {
		return fmt.Errorf("metadata string array differs")
	}
	for _, row := range rows {
		if err := metadataText(row); err != nil {
			return err
		}
	}
	return nil
}
func metadataURI(value any) error {
	m, err := objectKeys(value, []string{"value", "scheme", "path", "authority"}, nil)
	if err != nil {
		return err
	}
	for _, v := range m {
		if err := metadataText(v); err != nil {
			return err
		}
	}
	return nil
}
func metadataDictionary(value any, nullable bool) error {
	m, ok := value.(map[string]any)
	if !ok || len(m) > 128 {
		return fmt.Errorf("metadata dictionary differs")
	}
	for key, v := range m {
		if err := metadataText(key); err != nil {
			return err
		}
		if nullable {
			if v == nil {
				continue
			}
			if number, ok := v.(json.Number); ok {
				parsed, err := number.Float64()
				if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
					return fmt.Errorf("metadata environment number is not finite")
				}
				continue
			}
		}
		if err := metadataText(v); err != nil {
			return err
		}
	}
	return nil
}
func metadataMCP(value any) error {
	m, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("MCP object required")
	}
	kind, ok := m["kind"].(string)
	if !ok {
		return fmt.Errorf("MCP kind missing")
	}
	var err error
	switch kind {
	case "http":
		m, err = objectKeys(m, []string{"label", "kind", "uri", "headers"}, []string{"version"})
		if err == nil {
			err = metadataURI(m["uri"])
		}
		if err == nil {
			err = metadataDictionary(m["headers"], false)
		}
	case "stdio":
		m, err = objectKeys(m, []string{"label", "kind", "command", "args", "env"}, []string{"version", "cwd"})
		if err == nil {
			err = metadataText(m["command"])
		}
		if err == nil {
			err = metadataStrings(m["args"])
		}
		if err == nil {
			err = metadataDictionary(m["env"], true)
		}
		if cwd, present := m["cwd"]; err == nil && present {
			err = metadataURI(cwd)
		}
	default:
		return fmt.Errorf("MCP kind unknown")
	}
	if err != nil {
		return err
	}
	if err = metadataText(m["label"]); err != nil {
		return err
	}
	if version, present := m["version"]; present {
		return metadataText(version)
	}
	return nil
}
func metadataSkill(value any) error {
	m, err := objectKeys(value, []string{"uri", "name", "source", "disableModelInvocation"}, []string{"description", "sessionTypes", "userInvocable", "extensionId", "pluginUri"})
	if err != nil {
		return err
	}
	if err = metadataURI(m["uri"]); err != nil {
		return err
	}
	if err = metadataText(m["name"]); err != nil {
		return err
	}
	switch m["source"] {
	case "local", "user", "extension", "plugin", "builtin":
	default:
		return fmt.Errorf("unknown skill source")
	}
	if _, ok := m["disableModelInvocation"].(bool); !ok {
		return fmt.Errorf("unknown skill invocation flag")
	}
	for _, key := range []string{"description", "extensionId"} {
		if v, present := m[key]; present {
			if err = metadataText(v); err != nil {
				return err
			}
		}
	}
	if v, present := m["sessionTypes"]; present {
		if err = metadataStrings(v); err != nil {
			return err
		}
	}
	if v, present := m["userInvocable"]; present {
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("unknown user invocation flag")
		}
	}
	if v, present := m["pluginUri"]; present {
		return metadataURI(v)
	}
	return nil
}
func metadataRows(value any, validate func(any) error) error {
	rows, ok := value.([]any)
	if !ok || len(rows) > 128 {
		return fmt.Errorf("unknown/oversized native metadata array")
	}
	for _, row := range rows {
		if err := validate(row); err != nil {
			return err
		}
	}
	return nil
}
func metadataAttribution(m map[string]any, folder string, removed bool) map[string]any {
	result := map[string]any{"mcp": "blocked", "skills": "blocked", "mcp_source": "unique sealed marker URL inference; API provides no config-file origin", "same_name": "all native rows retained; no simultaneous activation claim", "connection_attempts": "not instrumented", "native_project_qualified": false}
	if mcpAttribution(m, folder, removed) {
		result["mcp"] = "passed"
	}
	if skillAttribution(m, folder, removed) {
		result["skills"] = "passed"
	}
	return result
}
func mcpAttribution(m map[string]any, folder string, removed bool) bool {
	rows, _ := m["mcp"].([]any)
	urls := map[string]int{}
	for _, row := range rows {
		r, _ := row.(map[string]any)
		u, _ := r["uri"].(map[string]any)
		v, _ := u["value"].(string)
		urls[v]++
	}
	label := filepath.Base(folder)
	other := "B"
	if label == "B" {
		other = "A"
	}
	selected, foreign, user := "http://127.0.0.1:9/UAP-"+label, "http://127.0.0.1:9/UAP-"+other, "http://127.0.0.1:9/UAP-USER"
	return urls[user] == 1 && urls[foreign] == 0 && ((!removed && urls[selected] == 1) || (removed && urls[selected] == 0))
}
func skillAttribution(m map[string]any, folder string, removed bool) bool {
	label := filepath.Base(folder)
	other := "B"
	if label == "B" {
		other = "A"
	}
	rows, _ := m["skills"].([]any)
	owned := filepath.Join(folder, ".github", "skills", "uap-"+strings.ToLower(label), "SKILL.md")
	home := filepath.Join(filepath.Dir(filepath.Dir(folder)), "home", ".copilot", "skills", "uap-user", "SKILL.md")
	counts, allPaths, sibling := skillMarkerCounts(rows, folder, other, owned, home, label)
	return counts[home] == 1 && allPaths[home] == 1 && !sibling && ((!removed && counts[owned] == 1 && allPaths[owned] == 1) || (removed && !hasSkillPath(rows, owned)))
}
func hasSkillPath(rows []any, path string) bool {
	for _, row := range rows {
		r, _ := row.(map[string]any)
		u, _ := r["uri"].(map[string]any)
		if u["path"] == path {
			return true
		}
	}
	return false
}
func skillMarkerCounts(rows []any, folder, other, owned, home, label string) (map[string]int, map[string]int, bool) {
	counts, allPaths := map[string]int{}, map[string]int{}
	sibling := false
	for _, row := range rows {
		r, _ := row.(map[string]any)
		u, _ := r["uri"].(map[string]any)
		p, _ := u["path"].(string)
		allPaths[p]++
		if strings.HasPrefix(p, filepath.Join(filepath.Dir(folder), other)+string(filepath.Separator)) {
			sibling = true
		}
		if u["scheme"] != "file" || u["authority"] != "" || u["value"] != "file://"+p || r["disableModelInvocation"] != true {
			continue
		}
		if p == owned && r["name"] == "uap-"+strings.ToLower(label) && r["description"] == label {
			counts[p]++
		}
		if p == home && r["name"] == "uap-user" && r["description"] == "USER" {
			counts[p]++
		}
	}
	return counts, allPaths, sibling
}
