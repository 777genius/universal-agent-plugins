package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// codexProjectDiscoveredSkills accepts the exact repository paths returned by
// native skills/list. A plugin, disabled item, duplicate, wrong scope or other
// project's same-named skill cannot satisfy project discovery.
func codexProjectDiscoveredSkills(listing []byte, project string) (map[string]string, error) {
	var decoded struct {
		Data []struct {
			CWD    string `json:"cwd"`
			Errors []any  `json:"errors"`
			Skills []struct {
				Name, Path, Scope string
				PluginID          string `json:"pluginId"`
				Enabled           bool
			} `json:"skills"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listing, &decoded); err != nil {
		return nil, err
	}
	if len(decoded.Data) != 1 || !openCodeSamePath(decoded.Data[0].CWD, project) || len(decoded.Data[0].Errors) != 0 {
		return nil, fmt.Errorf("Codex skills/list has a different cwd or discovery errors")
	}
	found := map[string]string{}
	for _, skill := range decoded.Data[0].Skills {
		if skill.Name != commandsGuideSkill && !strings.HasPrefix(skill.Name, commandsPluginName+"-") {
			continue
		}
		expected := filepath.Join(project, ".agents", "skills", skill.Name, "SKILL.md")
		if skill.PluginID != "" || skill.Scope != "repo" || !skill.Enabled || found[skill.Name] != "" || !openCodeSamePath(skill.Path, expected) {
			return nil, fmt.Errorf("unexpected Codex project skill %q", skill.Name)
		}
		found[skill.Name] = skill.Path
	}
	return found, nil
}

func TestCodexProjectDiscoveredSkills(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	entry := func(name string) map[string]any {
		return map[string]any{"name": name, "path": filepath.Join(project, ".agents", "skills", name, "SKILL.md"), "scope": "repo", "enabled": true}
	}
	for _, name := range []string{"valid", "wrong-cwd", "errors", "disabled", "plugin", "scope", "path", "duplicate"} {
		t.Run(name, func(t *testing.T) {
			skill := entry(commandsGuideSkill)
			group := map[string]any{"cwd": project, "skills": []any{skill, entry(commandsSkill("echo"))}}
			switch name {
			case "wrong-cwd":
				group["cwd"] = project + "-other"
			case "errors":
				group["errors"] = []any{map[string]any{"message": "unreadable skill"}}
			case "disabled":
				skill["enabled"] = false
			case "plugin":
				skill["pluginId"] = "foreign@marketplace"
			case "scope":
				skill["scope"] = "user"
			case "path":
				skill["path"] = filepath.Join(project+"-other", ".agents", "skills", commandsGuideSkill, "SKILL.md")
			case "duplicate":
				group["skills"] = []any{skill, skill}
			}
			body, err := json.Marshal(map[string]any{"data": []any{group}})
			if err != nil {
				t.Fatal(err)
			}
			found, err := codexProjectDiscoveredSkills(body, project)
			if name == "valid" && (err != nil || len(found) != 2) || name != "valid" && err == nil {
				t.Fatalf("found = %v, error = %v", found, err)
			}
		})
	}
}
