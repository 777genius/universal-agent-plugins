//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func clineCycleSkillRows(c *clineProjectCycle) []any {
	r := c.recipes["user"]
	row := func(name, description, instructions, path string) map[string]any {
		return map[string]any{"name": name, "description": description, "instructions": instructions, "path": path, "frontmatter": map[string]any{"name": name, "description": description}}
	}
	foreign := filepath.Join(c.f.base.Home, ".cline", "skills", "foreign-user", "SKILL.md")
	return []any{row("foreign-user", "Foreign inert marker", "FOREIGN_user\n", foreign), row(r.Skill, "user-one", "UAP_CLINE_user-one_"+r.Nonce+"\n", r.SkillPath)}
}
func TestClineCycleSkillsUseTypedExactAuthoredFrontmatterWithoutPanics(t *testing.T) {
	c := clineCycleInert(t)
	clineCycleStageActual(t, c, "user")
	r := c.recipes["user"]
	raw, err := json.Marshal(clineCycleSkillRows(c))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.skills(raw, r, []string{"user"}); err != nil {
		t.Fatal("actual activated source expected listing rejected", err)
	}
	for _, field := range []string{"name", "description", "instructions", "frontmatter"} {
		for _, value := range []any{map[string]any{"foreign": true}, []any{"foreign"}, nil, true, json.Number("1")} {
			rows := clineCycleSkillRows(c)
			row := rows[1].(map[string]any)
			if field == "name" || field == "description" {
				row["frontmatter"].(map[string]any)[field] = value
			}
			row[field] = value
			body, err := json.Marshal(rows)
			if err != nil {
				t.Fatal(err)
			}
			if err := c.skills(body, r, []string{"user"}); err == nil {
				t.Fatal("malformed native skill field was coerced or accepted", field)
			}
		}
	}
	for _, kind := range []string{"extra-frontmatter", "foreign-name", "foreign-description", "duplicate-path", "sibling-path"} {
		rows := clineCycleSkillRows(c)
		row := rows[1].(map[string]any)
		front := row["frontmatter"].(map[string]any)
		switch kind {
		case "extra-frontmatter":
			front["disable-model-invocation"] = true
		case "foreign-name":
			front["name"] = "foreign"
		case "foreign-description":
			front["description"] = "foreign"
		case "duplicate-path":
			rows[0] = row
		case "sibling-path":
			row["path"] = filepath.Join(c.f.other, ".cline", "skills", r.Skill, "SKILL.md")
		}
		body, err := json.Marshal(rows)
		if err != nil {
			t.Fatal(err)
		}
		if err := c.skills(body, r, []string{"user"}); err == nil {
			t.Fatal("unrecorded native skill metadata/authority accepted", kind)
		}
	}
}
