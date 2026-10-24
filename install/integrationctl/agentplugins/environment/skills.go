package environment

import (
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func validateSkillSource(source *GitSource) error {
	if source.Kind == "" {
		if source.Skill != "" {
			return errors.New("skill requires kind=skill")
		}
		return nil
	}
	if source.Kind != SkillKind {
		return errors.New("unsupported Git source kind")
	}
	if source.URL != "" || !repositoryPattern.MatchString(source.Repository) {
		return errors.New("standalone Skills require a GitHub owner/repo source")
	}
	return validateSkillName(source.Skill)
}

func validateSkillName(name string) error {
	if !domain.ValidSkillName(name) {
		return errors.New("invalid expected Skill name")
	}
	return nil
}

func validateLockedSkill(source *LockedGitSource) error {
	if source.Kind == "" {
		if source.Skill != "" || source.ResolvedSkillPath != "" || source.RawTreeDigest != "" || source.Normalizer != "" {
			return errors.New("skill lock fields require kind=skill")
		}
		return nil
	}
	if source.Kind != SkillKind || source.URL != "" || !repositoryPattern.MatchString(source.Repository) || source.Normalizer != SkillNormalizerV1 || !validGraphDigest(source.RawTreeDigest) {
		return errors.New("unsupported or incomplete Skill normalization identity")
	}
	return validateSubpath(source.ResolvedSkillPath)
}
