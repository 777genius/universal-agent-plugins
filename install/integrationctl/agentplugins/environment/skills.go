package environment

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
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
	name := source.Skill
	if n := utf8.RuneCountInString(name); n < 1 || n > 64 || strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") || strings.Contains(name, "--") {
		return errors.New("invalid expected Skill name")
	}
	for _, r := range name {
		if r != '-' && !unicode.IsLower(r) && !unicode.IsNumber(r) {
			return errors.New("invalid expected Skill name")
		}
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
	if source.Kind != SkillKind || source.Normalizer != SkillNormalizerV1 || !validGraphDigest(source.RawTreeDigest) {
		return errors.New("unsupported or incomplete Skill normalization identity")
	}
	return validateSubpath(source.ResolvedSkillPath)
}
