package installer

import (
	"path"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

// The stable key is authority only for the accepted normalized GitHub Skill
// contract, never a replacement for a direct transport's derived source key.
func acquiredSkillSourceKey(source domain.SourceIdentity, portable bool) (string, error) {
	facts := source.Skill
	if !portable || facts == nil || source.WebSkill != nil || source.SourceBindingHint != "uap-dependency-skill" || source.SourceKind != "" || source.SourceURL != "" || source.ArchiveSHA256 != "" {
		return "", ErrInvalidRequest
	}
	_, repository, err := sourcespec.NormalizeGitURL("https://github.com/" + source.Repository + ".git")
	if err != nil || !strings.EqualFold(repository, source.Repository) || !domain.ValidSkillName(facts.Name) {
		return "", ErrInvalidRequest
	}
	if facts.Normalizer != "agent-skill-package/v1" || !strings.HasPrefix(facts.RawTreeDigest, "sha256:") || !sourcespec.ValidSHA256(strings.TrimPrefix(facts.RawTreeDigest, "sha256:")) {
		return "", ErrInvalidRequest
	}
	if !validAcquiredSkillPaths(*facts, source.PackageSubpath) {
		return "", ErrInvalidRequest
	}
	key := "github-skill:" + strings.ToLower(repository) + ":" + facts.Name + ":" + facts.RequestedPath
	if source.StableSourceKey != key {
		return "", ErrInvalidRequest
	}
	return key, nil
}

func validAcquiredSkillPaths(facts domain.SkillSourceFacts, packageSubpath string) bool {
	for _, value := range []string{facts.RequestedPath, facts.ResolvedPath} {
		if normalized, err := sourcespec.NormalizeSubpath(value); err != nil || normalized != value {
			return false
		}
	}
	return facts.ResolvedPath == packageSubpath && (facts.ResolvedPath == "" || path.Base(facts.ResolvedPath) == facts.Name)
}

func acquiredWebSkillSourceKey(source domain.SourceIdentity, portable bool) (string, error) {
	facts := source.WebSkill
	if !portable || facts == nil || source.Skill != nil || source.SourceBindingHint != "uap-dependency-web-skill" || source.SourceKind != "https-skill" || source.ArchiveSHA256 != "" || source.Repository != "" || source.PackageSubpath != "" || domain.ValidateWebSkillSourceFacts(*facts) != nil {
		return "", ErrInvalidRequest
	}
	key := "https-skill:" + facts.URL + ":" + facts.Skill
	if source.SourceURL != facts.URL || source.StableSourceKey != key || source.CanonicalSource != key || source.RequestedSource != key || source.ResolvedRevision != facts.RawTreeDigest {
		return "", ErrInvalidRequest
	}
	return key, nil
}
