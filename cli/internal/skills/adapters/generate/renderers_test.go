package generate

import (
	"bytes"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/skills/adapters/filesystem"
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
		name, templateName, relPath string
		render                      func(string, domain.SkillDocument) ([]domain.Artifact, error)
	}{
		{"claude", "generate.claude.md.tmpl", "generated/skills/claude/sample/SKILL.md", ClaudeRenderer{}.Generate},
		{"codex", "generate.codex.skill.md.tmpl", "generated/skills/codex/sample/SKILL.md", CodexRenderer{}.Generate},
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
			want, err := filesystem.RenderTemplate(tc.templateName, filesystem.TemplateData{
				SkillName:            "sample",
				Description:          doc.Spec.Description,
				CommandLine:          filesystem.CommandLine(doc.Spec),
				Runtime:              string(doc.Spec.Runtime),
				AllowedTools:         doc.Spec.AllowedTools,
				CompatibilitySummary: compatibilitySummary(doc.Spec.Compatibility),
				Body:                 doc.Body,
			})
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(artifacts[0].Content, want) {
				t.Fatalf("%s rendered bytes differ from the existing client template", tc.name)
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
