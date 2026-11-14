package app

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/skills/adapters/filesystem"
	"github.com/777genius/plugin-kit-ai/cli/internal/skills/domain"
)

type InitOptions struct {
	Name        string
	Description string
	Template    filesystem.InitTemplate
	OutputDir   string
	Command     string
	Force       bool
}

type ValidateOptions struct {
	Root string
}

type RenderOptions struct {
	Root   string
	Target string
}

type ValidationFailure struct {
	Path    string
	Message string
}

type ValidationReport struct {
	Skills   []string
	Failures []ValidationFailure
}

type Service struct {
	Repo filesystem.Repository
}

type RenderResult struct {
	Artifacts  []domain.Artifact
	StalePaths []string
}

func (s Service) WriteArtifacts(root string, artifacts []domain.Artifact) error {
	return s.Repo.WriteArtifacts(root, artifacts)
}

func (s Service) RemoveArtifacts(root string, relPaths []string) error {
	return s.Repo.RemoveArtifacts(root, relPaths)
}
