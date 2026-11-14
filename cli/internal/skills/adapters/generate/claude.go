package generate

import "github.com/777genius/plugin-kit-ai/cli/internal/skills/domain"

type ClaudeRenderer struct{}

func (ClaudeRenderer) Target() string { return "claude" }

func (ClaudeRenderer) Generate(name string, doc domain.SkillDocument) ([]domain.Artifact, error) {
	return renderSkillArtifact(name, doc, "generate.claude.md.tmpl", "generated/skills/claude/"+name+"/SKILL.md")
}
