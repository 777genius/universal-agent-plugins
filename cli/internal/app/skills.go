package app

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/skills/adapters/filesystem"
	skillsapp "github.com/777genius/plugin-kit-ai/cli/internal/skills/app"
)

type SkillsInitOptions struct {
	Name        string
	Description string
	Template    string
	OutputDir   string
	Command     string
	Force       bool
}

type SkillsValidateOptions struct {
	Root string
}

type SkillsGenerateOptions struct {
	Root   string
	Target string
}

type SkillsService struct {
	svc            skillsapp.Service
	ExternalRunner ExternalSkillsRunner
}

func (s SkillsService) Init(opts SkillsInitOptions) (string, error) {
	return s.svc.Init(skillsapp.InitOptions{
		Name:        opts.Name,
		Description: opts.Description,
		Template:    filesystem.InitTemplate(opts.Template),
		OutputDir:   opts.OutputDir,
		Command:     opts.Command,
		Force:       opts.Force,
	})
}

func (s SkillsService) Validate(opts SkillsValidateOptions) (skillsapp.ValidationReport, error) {
	return s.svc.Validate(skillsapp.ValidateOptions{Root: opts.Root})
}

func (s SkillsService) Generate(opts SkillsGenerateOptions) ([]string, error) {
	result, err := s.svc.Generate(skillsapp.RenderOptions{Root: opts.Root, Target: opts.Target})
	if err != nil {
		return nil, err
	}
	if err := s.svc.WriteArtifacts(opts.Root, result.Artifacts); err != nil {
		return nil, err
	}
	if err := s.svc.RemoveArtifacts(opts.Root, result.StalePaths); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(result.Artifacts))
	for _, artifact := range result.Artifacts {
		out = append(out, artifact.RelPath)
	}
	return out, nil
}
