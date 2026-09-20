package generate

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/skills/adapters/filesystem"
	"github.com/777genius/plugin-kit-ai/cli/internal/skills/domain"
)

type CodexRenderer struct{}

func (CodexRenderer) Target() string { return "codex" }

func (CodexRenderer) Generate(name string, doc domain.SkillDocument) ([]domain.Artifact, error) {
	skillBody, err := filesystem.RenderTemplate("generate.codex.skill.md.tmpl", filesystem.TemplateData{
		SkillName:            name,
		Description:          doc.Spec.Description,
		CommandLine:          filesystem.CommandLine(doc.Spec),
		Runtime:              string(doc.Spec.Runtime),
		AllowedTools:         doc.Spec.AllowedTools,
		CompatibilitySummary: compatibilitySummary(doc.Spec.Compatibility),
		Body:                 doc.Body,
	})
	if err != nil {
		return nil, err
	}
	return []domain.Artifact{
		{
			RelPath: "generated/skills/codex/" + name + "/SKILL.md",
			Content: skillBody,
		},
	}, nil
}
