package pluginkitairepo_test

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestClaudeMarketplaceEvidenceRequiresExactScopedIdentity(t *testing.T) {
	root := t.TempDir()
	f := &claudeProjectNativeFixture{base: &claudeNativeFixture{ConfigDir: filepath.Join(root, "profile")}, project: filepath.Join(root, "A")}
	managed := filepath.Join(f.project, ".claude", "skills", "owned")
	for _, test := range []struct {
		name    string
		active  bool
		mutate  func(map[string]any)
		invalid bool
	}{
		{name: "selected project", active: true},
		{name: "other project sees disabled registration", active: false},
		{name: "foreign scope", active: true, mutate: func(row map[string]any) { row["scope"] = "user" }, invalid: true},
		{name: "foreign project", active: true, mutate: func(row map[string]any) { row["projectPath"] = filepath.Join(root, "B") }, invalid: true},
		{name: "wrong cache", active: true, mutate: func(row map[string]any) { row["installPath"] = managed }, invalid: true},
		{name: "disabled in selected project", active: true, mutate: func(row map[string]any) { row["enabled"] = false }, invalid: true},
		{name: "enabled in other project", active: false, mutate: func(row map[string]any) { row["enabled"] = true }, invalid: true},
		{name: "wrong source", active: true, mutate: func(row map[string]any) { row["readFromFolder"] = root }, invalid: true},
		{name: "legacy duplicate", active: true, mutate: func(row map[string]any) { row["id"] = claudeCommandsPluginID }, invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			row := map[string]any{"id": claudeProjectMarketplaceID(f.project, managed), "scope": "project", "projectPath": f.project,
				"version": "1.0.0", "enabled": test.active, "installPath": claudeProjectMarketplaceCache(f, managed)}
			if test.mutate != nil {
				test.mutate(row)
			}
			body, err := json.Marshal([]any{row})
			if err != nil {
				t.Fatal(err)
			}
			if err := validateClaudeMarketplaceListing(body, f, managed, true, test.active); (err != nil) != test.invalid {
				t.Fatalf("native scoped observation error = %v, invalid = %v", err, test.invalid)
			}
		})
	}
}

func TestClaudeInitializationRequiresNoTokenAndExactSkillInventory(t *testing.T) {
	wanted := []string{"fixture:one", "fixture:two"}
	for _, test := range []struct {
		name, token string
		commands    []string
		present     bool
		invalid     bool
	}{
		{"loaded once", "none", wanted, true, false},
		{"other project", "none", nil, false, false},
		{"account credential", "apiKey", wanted, true, true},
		{"missing Skill", "none", []string{"fixture:one"}, true, true},
		{"duplicate Skill", "none", []string{"fixture:one", "fixture:one", "fixture:two"}, true, true},
		{"unselected Skill", "none", []string{"fixture:one", "fixture:two", "fixture:three"}, true, true},
		{"wrong project activation", "none", wanted, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			commands := []any{map[string]any{"name": "help", "builtin": true}}
			for _, name := range test.commands {
				commands = append(commands, map[string]any{"name": name})
			}
			body, err := json.Marshal(map[string]any{"type": "control_response", "response": map[string]any{
				"subtype": "success", "request_id": "native-discovery", "response": map[string]any{
					"account": map[string]any{"tokenSource": test.token}, "commands": commands,
				},
			}})
			if err != nil {
				t.Fatal(err)
			}
			if err := validateClaudeInitializedSkills(body, wanted, test.present); (err != nil) != test.invalid {
				t.Fatalf("native initialized observation error = %v, invalid = %v", err, test.invalid)
			}
		})
	}
}
