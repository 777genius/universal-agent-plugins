//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
)

func (c *copilotProjectCycle) skills(project string) map[string]map[string]any {
	want := map[string]map[string]any{}
	add := func(name, description, source, path string, invocable bool) {
		want[name] = map[string]any{"name": name, "commandName": name, "description": description, "source": source, "path": path, "userInvocable": invocable, "enabled": true}
	}
	add("uap-foreign-user", "Original user skill must remain unchanged", "personal-copilot", filepath.Join(c.f.base.Home, ".copilot", "skills", "uap-foreign-user", "SKILL.md"), true)
	if project == c.f.other {
		add("uap-foreign-b", "Original project B skill must remain unchanged", "project", filepath.Join(project, ".github", "skills", "uap-foreign-b", "SKILL.md"), true)
	}
	for _, key := range []string{"user", "a", "b"} {
		r := c.recipes[key]
		if r.Installation.InstallationID == "" || r.Removed || key != "user" && r.Project != project {
			continue
		}
		source := "project"
		if key == "user" {
			source = "plugin"
		}
		add(r.Skill, fmt.Sprintf("Disposable Copilot %s %s %s", r.Key, r.Revision, r.Nonce), source, r.SkillPath, true)
		if key == "user" {
			want[r.Skill]["commandName"] = "native-proof:" + r.Skill
		}
	}
	for _, name := range []string{"customize-cloud-agent", "github-pr-media"} {
		add(name, "", "builtin", filepath.Join(c.f.base.Root, "xdg-cache", "copilot", "pkg", "linux-x64", "1.0.103", "builtin", name, "SKILL.md"), false)
	}
	return want
}

func (c *copilotProjectCycle) metadata(s *copilotReadinessStream, project string, r *copilotCycleRecipe) error {
	offset := 3
	if r == nil {
		offset = 0
	}
	if r != nil {
		for i, want := range []map[string]any{{"serverName": r.Skill, "status": "pending"}, {"serverName": r.Skill, "status": "connected", "configSource": "workspace"}, {"serverName": r.Skill, "toolCount": json.Number("1")}} {
			if !reflect.DeepEqual(s.data[i], want) {
				return fmt.Errorf("Copilot cycle exact workspace MCP startup")
			}
		}
	}
	if !reflect.DeepEqual(s.data[offset+1], map[string]any{"infoType": "configuration", "message": copilotReadinessDisabled}) || !reflect.DeepEqual(s.data[offset+3], map[string]any{"model": "uap-fixture-model"}) {
		return fmt.Errorf("Copilot cycle whole tool configuration/model")
	}
	if err := c.skillCatalog(s.data[offset]["skills"], project); err != nil {
		return err
	}
	servers := []any{}
	for _, name := range []string{"WorkIQ", "browser-use", "github-copilot-browser", "github-mcp-server", "githubiq"} {
		servers = append(servers, map[string]any{"name": name, "status": "disabled"})
	}
	if r != nil {
		servers = append(servers, map[string]any{"name": r.Skill, "status": "connected", "source": "workspace", "serverMetadata": map[string]any{"instructions": nil}})
	}
	if !reflect.DeepEqual(s.data[offset+2]["servers"], servers) {
		return fmt.Errorf("Copilot cycle complete native server inventory")
	}
	return nil
}

func (c *copilotProjectCycle) skillCatalog(value any, project string) error {
	rows, ok := value.([]any)
	want := c.skills(project)
	if !ok || len(rows) != len(want) {
		return fmt.Errorf("Copilot cycle complete native skill inventory")
	}
	seen := map[string]bool{}
	hashes := map[string]string{"customize-cloud-agent": "b96d547993e3bf5142f5c01773688c2cad8c12a4cfddd3ca54cddf43838b101f", "github-pr-media": "6911e8b48dad09cfe8ed00b26792dc96939bc2ec9ab7b523d1d41e1109d8c444"}
	for _, value := range rows {
		row, err := copilotReadinessObject(value, "name commandName description source userInvocable enabled path")
		if err != nil {
			return err
		}
		name, ok := row["name"].(string)
		expected, exists := want[name]
		if !ok || !exists || seen[name] {
			return fmt.Errorf("Copilot cycle unknown/duplicate skill")
		}
		seen[name] = true
		if hash, builtin := hashes[name]; builtin {
			description, ok := row["description"].(string)
			if !ok || copilotProjectConfigHash([]byte(description)) != hash {
				return fmt.Errorf("Copilot cycle builtin description changed")
			}
			expected["description"] = description
		}
		if !reflect.DeepEqual(row, expected) {
			return fmt.Errorf("Copilot cycle skill scope/path/source/version bytes differ")
		}
	}
	return nil
}
