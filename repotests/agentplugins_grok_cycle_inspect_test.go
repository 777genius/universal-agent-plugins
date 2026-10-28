//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
)

type grokCycleNativePlugin struct {
	Name, Scope, Path, Description string
	Skills                         int
	Loaded                         bool
}

func (c *grokProjectCycle) expectedInspection(scope string, damaged bool) []grokCycleNativePlugin {
	name := "uap-grok-original-user"
	result := []grokCycleNativePlugin{{Name: name, Scope: "user", Path: filepath.Join(c.f.base.Home, ".grok", "plugins", name), Description: "Original user " + c.f.base.Nonce, Skills: 1, Loaded: true}}
	if scope == "outside" {
		return result
	}
	r := c.recipes[scope]
	foreign := "uap-grok-foreign-" + scope
	result = append(result, grokCycleNativePlugin{Name: foreign, Scope: "project", Path: filepath.Join(r.Project, ".grok", "plugins", foreign), Description: "Foreign inert marker", Skills: 1})
	if r.Active != "" && !r.Removed && !damaged {
		result = append(result, grokCycleNativePlugin{Name: r.Name, Scope: "project", Path: r.Active, Description: r.Revision + " " + c.f.base.Nonce, Skills: 1, Loaded: true})
	}
	return result
}

func grokCycleInspect(raw []byte, cwd string, trusted bool, expected []grokCycleNativePlugin) error {
	value, err := projectNativeRuntimeJSON(raw)
	if err != nil {
		return err
	}
	object, ok := value.(map[string]any)
	if !ok || !projectGrokIdentityKeys(object, "grokVersion cwd projectRoot projectTrusted plugins skills mcpServers") || object["grokVersion"] != "1.0.70" || object["cwd"] != cwd || (object["projectRoot"] != cwd && object["projectRoot"] != cwd+"/") || object["projectTrusted"] != trusted {
		return fmt.Errorf("Grok cycle fresh inspection project identity differs")
	}
	plugins, p := object["plugins"].([]any)
	skills, s := object["skills"].([]any)
	mcp, m := object["mcpServers"].([]any)
	if !p || !s || !m || len(mcp) != 0 || len(plugins) != len(expected) {
		return fmt.Errorf("Grok cycle complete native inventory differs")
	}
	wanted := map[string]grokCycleNativePlugin{}
	for _, plugin := range expected {
		wanted[plugin.Name] = plugin
	}
	seen := map[string]bool{}
	for _, value := range plugins {
		entry, ok := value.(map[string]any)
		name, named := entry["name"].(string)
		want, found := wanted[name]
		provides := map[string]any{"skills": json.Number(fmt.Sprint(want.Skills)), "agents": json.Number("0"), "hooks": false, "mcpServers": json.Number("0")}
		identity := map[string]any{"name": name, "scope": want.Scope, "path": want.Path, "enabled": true, "provides": provides}
		if !ok || !named || !found || seen[name] || !reflect.DeepEqual(entry, identity) {
			return fmt.Errorf("Grok cycle exact plugin source/catalog differs")
		}
		seen[name] = true
	}
	seen = map[string]bool{}
	loaded := 0
	for _, plugin := range expected {
		if plugin.Loaded {
			loaded++
		}
	}
	if len(skills) != loaded {
		return fmt.Errorf("Grok cycle complete loaded skill set differs")
	}
	for _, value := range skills {
		entry, ok := value.(map[string]any)
		name, named := entry["name"].(string)
		want, found := wanted[name]
		if !ok || !named || !found || !want.Loaded || seen[name] || strings.TrimSpace(name) == "" {
			return fmt.Errorf("Grok cycle foreign/duplicate/disabled skill loaded")
		}
		expectedSkill := map[string]any{"name": name, "description": want.Description, "source": map[string]any{"type": "plugin", "plugin_name": name, "path": filepath.Join(want.Path, "skills", name, "SKILL.md")}, "userInvocable": true}
		if disabled, exists := entry["disabled"]; exists {
			if disabled != false {
				return fmt.Errorf("Grok cycle loaded skill is disabled")
			}
			expectedSkill["disabled"] = false
		}
		if !reflect.DeepEqual(entry, expectedSkill) {
			return fmt.Errorf("Grok cycle loaded skill source or nonce/revision differs")
		}
		seen[name] = true
	}
	return nil
}
