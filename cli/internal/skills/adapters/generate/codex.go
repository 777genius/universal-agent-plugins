package generate

import "github.com/777genius/plugin-kit-ai/cli/internal/skills/domain"

type CodexRenderer struct{}

func (CodexRenderer) Target() string { return "codex" }

func (CodexRenderer) Generate(name string, doc domain.SkillDocument) ([]domain.Artifact, error) {
	return renderSkillArtifact(name, doc, "generate.codex.skill.md.tmpl", "generated/skills/codex/"+name+"/SKILL.md")
}
