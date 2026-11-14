package app

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/skills/domain"
)

func (s Service) validateSkills(opts ValidateOptions) (ValidationReport, error) {
	names, err := s.Repo.Discover(opts.Root)
	if err != nil {
		return ValidationReport{}, err
	}
	report := ValidationReport{Skills: names}
	for _, name := range names {
		doc, err := s.Repo.LoadSkill(opts.Root, name)
		if err != nil {
			report.Failures = append(report.Failures, ValidationFailure{
				Path:    filepath.Join("skills", name, "SKILL.md"),
				Message: err.Error(),
			})
			continue
		}
		report.Failures = append(report.Failures, validateDoc(opts.Root, name, doc)...)
	}
	return report, nil
}

func validateDoc(root, name string, doc domain.SkillDocument) []ValidationFailure {
	skillPath := filepath.Join("skills", name, "SKILL.md")
	failures := validateSkillMetadata(skillPath, name, doc)
	failures = append(failures, validateSkillCollections(skillPath, doc)...)
	failures = append(failures, validateSkillAgentHints(skillPath, doc)...)
	failures = append(failures, validateSkillAgents(skillPath, doc)...)
	failures = append(failures, validateSkillExecution(root, skillPath, name, doc)...)
	failures = append(failures, validateSkillRequiredSections(skillPath, doc)...)
	return failures
}
