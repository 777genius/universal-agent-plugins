package codex

import (
	"path/filepath"
	"testing"
)

func TestCodexProjectSkillNamesCanCoexistWithSystemSkills(t *testing.T) {
	root := t.TempDir()
	expected := filepath.Join(root, "project", ".agents", "skills", "skill-creator", "SKILL.md")
	builtin := filepath.Join(root, "profile", "skills", ".system", "skill-creator", "SKILL.md")
	for _, scenario := range []string{
		"system-after", "system-before", "system-disabled", "system-only", "owned-disabled",
		"owned-duplicate", "owned-foreign-path", "owned-plugin", "owned-wrong-scope",
		"user-name-conflict", "system-claims-owned-path", "system-plugin", "system-relative-path",
		"system-missing-path", "malformed-entry", "user-plugin-discovery",
	} {
		t.Run(scenario, func(t *testing.T) {
			owned := map[string]any{"name": "skill-creator", "path": expected, "scope": "repo", "enabled": true, "pluginId": nil}
			system := map[string]any{"name": "skill-creator", "path": builtin, "scope": "system", "enabled": true, "pluginId": nil}
			skills := []any{owned, system}
			scope, pluginID := "repo", ""
			switch scenario {
			case "system-before":
				skills = []any{system, owned}
			case "system-disabled":
				system["enabled"] = false
			case "system-only":
				skills = []any{system}
			case "owned-disabled":
				owned["enabled"] = false
			case "owned-duplicate":
				skills = []any{owned, system, owned}
			case "owned-foreign-path":
				owned["path"] = filepath.Join(root, "other-project", "SKILL.md")
			case "owned-plugin":
				owned["pluginId"] = "foreign@marketplace"
			case "owned-wrong-scope":
				owned["scope"] = "user"
			case "user-name-conflict":
				system["scope"] = "user"
			case "system-claims-owned-path":
				system["path"] = expected
			case "system-plugin":
				system["pluginId"] = "foreign@marketplace"
			case "system-relative-path":
				system["path"] = "relative/SKILL.md"
			case "system-missing-path":
				delete(system, "path")
			case "malformed-entry":
				skills = []any{owned, system, nil}
			case "user-plugin-discovery":
				scope, pluginID = "user", "selected@marketplace"
				owned["scope"], owned["pluginId"] = scope, pluginID
			}
			err := verifyCodexSkillEntries(skills, map[string]string{"skill-creator": expected}, scope, pluginID)
			wantReady := scenario == "system-after" || scenario == "system-before" || scenario == "system-disabled"
			if (err == nil) != wantReady {
				t.Fatalf("native inventory ready=%v, want %v: %v", err == nil, wantReady, err)
			}
		})
	}
}
