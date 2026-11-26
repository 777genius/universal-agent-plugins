package dependencymanifest

import (
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func localRequirement(alias string, raw rawDependency) (environment.Requirement, error) {
	if raw.Kind != environment.SkillKind || raw.Skill == "" {
		return environment.Requirement{}, errors.New("local source requires kind=skill and an exact skill name")
	}
	if raw.Path != "" || raw.Ref != "" || raw.Commit != "" || raw.SHA256 != "" {
		return environment.Requirement{}, errors.New("local Skill source cannot contain Git or archive fields")
	}
	return environment.Requirement{Alias: alias, Local: &environment.LocalSource{Path: raw.Local, Skill: raw.Skill}}, nil
}
