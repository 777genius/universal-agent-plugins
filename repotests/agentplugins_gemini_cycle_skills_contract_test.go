//go:build linux

package pluginkitairepo_test

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeminiCycleActualPinnedCatalogRetainsOnlyExactBuiltinAndSelectedRows(t *testing.T) {
	encoded, err := os.ReadFile(filepath.Join("testdata", "gemini-cycle-2ff", "research-configured-a-skills.stdout.b64"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	if geminiCycleSHA(raw) != "7b91c209fae18fd87b0d8813ba38bd570dd7ca2bb7418aa571d2a0eae7c19fae" {
		t.Fatal("actual2ff catalog source bytes changed")
	}
	root := "/home/runner/work/_temp/project-native-gemini"
	want := map[string]geminiCycleSkillRow{
		"uap-gemini-research-a":    {Description: "Inert source marker", Path: root + "/project-a/.gemini/skills/uap-gemini-research-a/SKILL.md", Enabled: true},
		"uap-gemini-research-user": {Description: "Inert source marker", Path: root + "/home/.gemini/skills/uap-gemini-research-user/SKILL.md", Enabled: true},
		"antigravity-support":      {Description: "Use when the user asks questions, seeks help, or requests instructions related to installing, setting up, or migrating to Antigravity CLI. This skill provides the latest up to date details, requirements, and commands sourced from the official Antigravity CLI documentation.", Path: root + "/client/builtin/antigravity-support/SKILL.md", Enabled: true, Builtin: true},
		"skill-creator":            {Description: "Guide for creating effective skills. This skill should be used when users want to create a new skill (or update an existing skill) that extends Gemini CLI's capabilities with specialized knowledge, workflows, or tool integrations.", Path: root + "/client/builtin/skill-creator/SKILL.md", Enabled: true, Builtin: true},
	}
	if geminiCycleSkills(raw, want) != nil {
		t.Fatal("actual pinned4-row catalog refused")
	}
	for _, kind := range []string{"builtin-path", "builtin-name", "builtin-description", "builtin-enabled", "builtin-flag", "missing-builtin", "extra-builtin", "duplicate-owned", "foreign-owned", "wrong-owned-path", "control"} {
		t.Run(kind, func(t *testing.T) {
			changed := string(raw)
			switch kind {
			case "builtin-path":
				changed = strings.Replace(changed, "/builtin/skill-creator/", "/builtin/foreign/", 1)
			case "builtin-name":
				changed = strings.Replace(changed, "skill-creator [Enabled]", "other [Enabled]", 1)
			case "builtin-description":
				changed = strings.Replace(changed, "Guide for creating effective skills.", "Foreign guide.", 1)
			case "builtin-enabled":
				changed = strings.Replace(changed, "skill-creator [Enabled]", "skill-creator [Disabled]", 1)
			case "builtin-flag":
				changed = strings.Replace(changed, "skill-creator [Enabled] [Built-in]", "skill-creator [Enabled]", 1)
			case "missing-builtin":
				head, _, found := strings.Cut(changed, "skill-creator [Enabled]")
				if !found {
					t.Fatal("measured builtin row missing from positive control")
				}
				changed = head
			case "extra-builtin":
				changed += "other [Enabled] [Built-in]\n  Description: Other\n  Location:    /foreign/SKILL.md\n"
			case "duplicate-owned":
				changed += "uap-gemini-research-a [Enabled]\n  Description: Inert source marker\n  Location:    /foreign/SKILL.md\n"
			case "foreign-owned":
				changed = strings.ReplaceAll(changed, "uap-gemini-research-a", "foreign-a")
			case "wrong-owned-path":
				changed = strings.Replace(changed, "/project-a/.gemini/", "/project-b/.gemini/", 1)
			case "control":
				changed += "\x1b[OTHER"
			}
			if geminiCycleSkills([]byte(changed), want) == nil {
				t.Fatal("unclosed pinned/selected catalog accepted")
			}
		})
	}
}
