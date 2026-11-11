package generate

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/skills/domain"
)

func TestSkillRenderersPreserveClientTemplatesAndPaths(t *testing.T) {
	doc := domain.SkillDocument{
		Spec: domain.SkillSpec{
			Description:  "Inspect one authored skill",
			Command:      "echo",
			Args:         []string{"two words"},
			Runtime:      domain.RuntimeShell,
			AllowedTools: []string{"Read", "Search"},
			Compatibility: domain.CompatibilitySpec{
				Requires:        []string{"git"},
				SupportedOS:     []string{"linux"},
				RepoRequired:    true,
				NetworkRequired: true,
				Notes:           []string{"verify local inputs"},
			},
		},
		Body: "Follow these canonical instructions.",
	}
	tests := []struct {
		name, relPath string
		render        func(string, domain.SkillDocument) ([]domain.Artifact, error)
	}{
		{"claude", "generated/skills/claude/sample/SKILL.md", ClaudeRenderer{}.Generate},
		{"codex", "generated/skills/codex/sample/SKILL.md", CodexRenderer{}.Generate},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			artifacts, err := tc.render("sample", doc)
			if err != nil {
				t.Fatal(err)
			}
			if len(artifacts) != 1 || artifacts[0].RelPath != tc.relPath {
				t.Fatalf("artifact path/count changed: %+v", artifacts)
			}
			// Fixed output assertions catch projection regressions independently of
			// the helper's own field mapping and template-data construction.
			rendered := string(artifacts[0].Content)
			for _, expected := range []string{
				"# sample",
				"Inspect one authored skill",
				"- Runtime: `shell`",
				"- Invocation: `echo 'two words'`",
				"- Requires: git",
				"- Supported OS: linux",
				"- Requires a repository checkout",
				"- May require network access",
				"- verify local inputs",
				"- `Read`",
				"- `Search`",
			} {
				if !strings.Contains(rendered, expected) {
					t.Fatalf("%s output missing %q", tc.name, expected)
				}
			}
			if !strings.Contains(string(artifacts[0].Content), doc.Body) {
				t.Fatal("canonical instructions missing")
			}
		})
	}
}

func TestRenderSkillArtifactFailsWithoutPartialOutput(t *testing.T) {
	artifacts, err := renderSkillArtifact("sample", domain.SkillDocument{}, "absent-template.tmpl", "generated/skills/claude/sample/SKILL.md")
	if err == nil || artifacts != nil {
		t.Fatalf("template failure must not emit a partial artifact: artifacts=%v, err=%v", artifacts, err)
	}
}
