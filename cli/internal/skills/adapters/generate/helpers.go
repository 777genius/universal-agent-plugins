package generate

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/skills/adapters/filesystem"
	"github.com/777genius/plugin-kit-ai/cli/internal/skills/domain"
)

func compatibilitySummary(spec domain.CompatibilitySpec) []string {
	var out []string
	if len(spec.Requires) > 0 {
		out = append(out, "Requires: "+strings.Join(spec.Requires, ", "))
	}
	if len(spec.SupportedOS) > 0 {
		out = append(out, "Supported OS: "+strings.Join(spec.SupportedOS, ", "))
	}
	if spec.RepoRequired {
		out = append(out, "Requires a repository checkout")
	}
	if spec.NetworkRequired {
		out = append(out, "May require network access")
	}
	out = append(out, spec.Notes...)
	return out
}

// renderSkillArtifact owns the common authored-to-template projection.
// Each client still owns its original template and destination path.
func renderSkillArtifact(name string, doc domain.SkillDocument, templateName, relPath string) ([]domain.Artifact, error) {
	body, err := filesystem.RenderTemplate(templateName, filesystem.TemplateData{
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
	return []domain.Artifact{{RelPath: relPath, Content: body}}, nil
}
