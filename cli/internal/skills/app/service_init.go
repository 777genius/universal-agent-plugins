package app

import (
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/skills/adapters/filesystem"
)

func (s Service) Init(opts InitOptions) (string, error) {
	name := strings.TrimSpace(opts.Name)
	if err := validateName(name); err != nil {
		return "", err
	}
	root := strings.TrimSpace(opts.OutputDir)
	if root == "" {
		root = "."
	}
	desc := strings.TrimSpace(opts.Description)
	if desc == "" {
		desc = "plugin-kit-ai skill"
	}
	command := strings.TrimSpace(opts.Command)
	if command == "" {
		command = "replace-me"
	}
	if err := s.Repo.InitSkill(root, filesystem.TemplateData{
		SkillName:   name,
		Description: desc,
		Command:     command,
		CommandLine: command,
	}, opts.Template, opts.Force); err != nil {
		return "", err
	}
	return filepath.Join(root, "skills", name), nil
}
