package installer

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func acquiredLocalSkillSourceKey(source domain.SourceIdentity, portable bool) (string, error) {
	facts := source.Skill
	if !portable || facts == nil || source.WebSkill != nil || source.SourceKind != sourcespec.LocalSkillKind || source.SourceBindingHint != "uap-dependency-local-skill" || source.Repository != "" || source.PackageSubpath != "" || source.ArchiveSHA256 != "" {
		return "", ErrInvalidRequest
	}
	if sourcespec.ValidateLocalSkillPath(source.SourceURL) != nil || !domain.ValidSkillName(facts.Name) || facts.RequestedPath != "" || facts.ResolvedPath != "" || facts.Normalizer != "agent-skill-package/v1" || !validSkillRawDigest(facts.RawTreeDigest) {
		return "", ErrInvalidRequest
	}
	key := sourcespec.LocalSkillKey(source.SourceURL, facts.Name)
	if source.StableSourceKey != key || source.RequestedSource != key || source.CanonicalSource != key || source.ResolvedRevision != facts.RawTreeDigest {
		return "", ErrInvalidRequest
	}
	return key, nil
}

func validSkillRawDigest(value string) bool {
	return len(value) == len("sha256:")+64 && value[:len("sha256:")] == "sha256:" && sourcespec.ValidSHA256(value[len("sha256:"):])
}
