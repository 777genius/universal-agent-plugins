//go:build linux

package pluginkitairepo_test

import (
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

func windCycleSkill(name, description, provider, base string, triggers []any) map[string]any {
	return map[string]any{"name": name, "description": description, "provider": provider, "base_dir": base, "display_name": name, "triggers": triggers, "warnings": []any{}, "errors": []any{}}
}
func (c *windProjectCycle) skills(scope string, owned bool) map[string]map[string]any {
	w := c.w
	both := []any{"model", "user"}
	rows := map[string]map[string]any{
		"devin-cli":              windCycleSkill("devin-cli", "Look up Devin CLI documentation (skills, extensibility, configuration, commands, models, troubleshooting)", "Builtin", filepath.Join(filepath.Dir(filepath.Dir(w.f.client)), "share", "devin"), []any{"model"}),
		"declarative-repo-setup": windCycleSkill("declarative-repo-setup", "Generate and verify a working environment.yaml (Devin snapshot-setup blueprint) for a repo", "Builtin", "", both),
		"upload-secrets":         windCycleSkill("upload-secrets", "Securely upload local secrets (dotenv files, env vars, API keys) to the Devin Cloud secrets manager — values never enter the conversation", "Builtin", "", both),
		"foreign-user":           windCycleSkill("foreign-user", "Harmless foreign marker", "Windsurf", filepath.Join(w.recipes["user"].Profile, "skills", "foreign-user"), both),
	}
	if scope != "user" {
		rows["foreign-"+scope] = windCycleSkill("foreign-"+scope, "Harmless foreign marker", "Devin", filepath.Join(w.recipes[scope].Profile, "skills", "foreign-"+scope), both)
	}
	if owned && scope != "user" {
		r := w.recipes[scope]
		rows[windComponentName] = windCycleSkill(windComponentName, r.Revision, "Devin", filepath.Join(r.Profile, "skills", windComponentName), both)
	}
	return rows
}
func (c *windProjectCycle) native(label string, raw []byte) error {
	if label == "version" {
		return windCycleRequired(string(raw) == c.w.f.spec.versionOutput, "exact version")
	}
	command, err := c.selectCommand(label)
	if err != nil {
		return err
	}
	scope := command.Scope
	r := c.w.recipes[scope]
	owned := r.Active != "" && !c.removed[scope] && label != "damaged-a-skills"
	if strings.HasSuffix(label, "-skills") {
		value, err := projectNativeRuntimeJSON(raw)
		rows, ok := value.([]any)
		expected := c.skills(scope, owned)
		if err != nil || !ok || len(rows) != len(expected) {
			return fmt.Errorf("Windsurf skills whole catalog")
		}
		seen := map[string]bool{}
		for _, value := range rows {
			row, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("Windsurf skill row")
			}
			name, ok := row["name"].(string)
			triggers, valid := row["triggers"].([]any)
			if !ok || seen[name] || !valid || len(triggers) > 2 {
				return fmt.Errorf("Windsurf skill name/triggers")
			}
			copied := map[string]any{}
			for k, v := range row {
				copied[k] = v
			}
			sorted := append([]any{}, triggers...)
			for _, v := range sorted {
				if v != "user" && v != "model" {
					return fmt.Errorf("Windsurf skill trigger")
				}
			}
			sort.Slice(sorted, func(i, j int) bool { return sorted[i].(string) < sorted[j].(string) })
			copied["triggers"] = sorted
			if !reflect.DeepEqual(copied, expected[name]) {
				return fmt.Errorf("Windsurf selected skill source/revision/catalog")
			}
			seen[name] = true
		}
		return nil
	}
	selected := r
	source := r.Config
	if scope == "user" {
		source = "~/.codeium/windsurf/mcp_config.json"
	}
	if !owned {
		selected = c.w.recipes["user"]
		source = "~/.codeium/windsurf/mcp_config.json"
	}
	entry := c.entry(selected)
	args := []string{}
	if selected.Active != "" {
		for _, v := range entry["args"].([]any) {
			args = append(args, v.(string))
		}
	}
	rendered, _ := entry["command"].(string)
	rendered += " " + strings.Join(args, " ")
	if strings.HasSuffix(label, "-get") {
		expected := "Server: " + windComponentName + "\nSource: " + source + "\n    Command: " + rendered + "\n    Env: PLUGIN_DATA=<redacted>, PLUGIN_ROOT=<redacted>, UAP_TEST_EVENTS=<redacted>, UAP_TEST_NONCE=<redacted>, UAP_TEST_REVISION=<redacted>, UAP_TEST_ROOT=<redacted>\n"
		return windCycleRequired(selected.Active != "" && string(raw) == expected, "exact MCP configuration get")
	}
	expected := map[string]bool{}
	if selected.Active != "" {
		expected["  • "+windComponentName+"\n    Command: "+rendered+"\n    Source: "+source] = true
	}
	for _, foreign := range []string{"user", scope} {
		source := "~/.codeium/windsurf/mcp_config.json"
		if foreign != "user" {
			source = c.w.recipes[foreign].Config
		}
		expected["  ✗ foreign-"+foreign+"  (disabled)\n    URL: http://127.0.0.1:9/foreign-"+foreign+"\n    Source: "+source] = true
	}
	body := string(raw)
	if !strings.HasPrefix(body, "Configured MCP servers\n\n") || !strings.HasSuffix(body, "\n\n") {
		return fmt.Errorf("Windsurf MCP header/framing")
	}
	blocks := strings.Split(strings.TrimSuffix(strings.TrimPrefix(body, "Configured MCP servers\n\n"), "\n\n"), "\n\n")
	if len(blocks) != len(expected) {
		return fmt.Errorf("Windsurf MCP complete catalog")
	}
	for _, block := range blocks {
		if !expected[block] {
			return fmt.Errorf("Windsurf MCP source/command/foreign block")
		}
		delete(expected, block)
	}
	return nil
}

func (c *windProjectCycle) entry(r *windComponentRecipe) map[string]any {
	entry := c.w.expectedEntry(r)
	if r.Revision == "a-two" {
		args := entry["args"].([]any)
		args[7] = "Owned Windsurf a-two"
	}
	return entry
}
