package environment

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func (requirement Requirement) SourceKey() (string, error) {
	if err := requirement.Validate(); err != nil {
		return "", err
	}
	if requirement.Local != nil {
		return sourcespec.LocalSkillKey(requirement.Local.Path, requirement.Local.Skill), nil
	}
	if requirement.Web != nil {
		return "https-skill:" + requirement.Web.URL + ":" + requirement.Web.Skill, nil
	}
	if requirement.Git != nil {
		if requirement.Git.Kind == SkillKind {
			return requirement.Git.skillSourceKey(), nil
		}
		if requirement.Git.URL != "" {
			return sourcespec.Key(sourcespec.KindGit, requirement.Git.URL, requirement.Git.Path), nil
		}
		return "github:" + strings.ToLower(requirement.Git.Repository) + ":" + requirement.Git.Path, nil
	}
	if requirement.Archive != nil {
		return sourcespec.Key(sourcespec.KindArchive, requirement.Archive.URL, requirement.Archive.Path), nil
	}
	return "directory:" + strings.ToLower(requirement.Registry.Distribution), nil
}
