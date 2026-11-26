package environment

import (
	"errors"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
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
	if err := source.ValidateEndpoint(); err != nil {
		return err
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
	endpoint := GitSource{Repository: source.Repository, URL: source.URL, Path: source.Path}
	if source.Kind != SkillKind || endpoint.ValidateEndpoint() != nil || source.Normalizer != SkillNormalizerV1 || !validGraphDigest(source.RawTreeDigest) {
		return errors.New("unsupported or incomplete Skill normalization identity")
	}
	return validateSubpath(source.ResolvedSkillPath)
}

func (source GitSource) skillSourceKey() string {
	if source.URL != "" {
		return sourcespec.GitSkillKey(source.URL, source.Skill, source.Path)
	}
	return "github-skill:" + strings.ToLower(source.Repository) + ":" + source.Skill + ":" + source.Path
}
