package filesystem

import (
	"embed"

	"github.com/777genius/plugin-kit-ai/cli/internal/skills/domain"
)

//go:embed templates/*.tmpl
var tmplFS embed.FS

type InitTemplate string

const (
	TemplateGoCommand  InitTemplate = "go-command"
	TemplateCLIWrapper InitTemplate = "cli-wrapper"
	TemplateDocsOnly   InitTemplate = "docs-only"
)

type TemplateData struct {
	SkillName            string
	Description          string
	Command              string
	CommandLine          string
	Runtime              string
	AllowedTools         []string
	CompatibilitySummary []string
	ExecutionNotes       []string
	Body                 string
}

type Repository struct{}

func (Repository) WriteArtifacts(root string, artifacts []domain.Artifact) error {
	return writeRepositoryArtifacts(root, artifacts)
}

func (Repository) RemoveArtifacts(root string, relPaths []string) error {
	return removeRepositoryArtifacts(root, relPaths)
}
