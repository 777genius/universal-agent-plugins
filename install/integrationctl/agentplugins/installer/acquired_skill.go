package installer

import (
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

// The stable key is authority only for the accepted normalized GitHub Skill
// contract, never a replacement for a direct transport's derived source key.
func acquiredSkillSourceKey(source domain.SourceIdentity, portable bool) (string, error) {
	facts := source.Skill
	if !portable || facts == nil || source.SourceBindingHint != "uap-dependency-skill" || source.SourceKind != "" || source.SourceURL != "" || source.ArchiveSHA256 != "" {
		return "", ErrInvalidRequest
	}
	_, repository, err := sourcespec.NormalizeGitURL("https://github.com/" + source.Repository + ".git")
	if err != nil || !strings.EqualFold(repository, source.Repository) || !validAcquiredSkillName(facts.Name) {
		return "", ErrInvalidRequest
	}
	if facts.Normalizer != "agent-skill-package/v1" || !strings.HasPrefix(facts.RawTreeDigest, "sha256:") || !sourcespec.ValidSHA256(strings.TrimPrefix(facts.RawTreeDigest, "sha256:")) {
		return "", ErrInvalidRequest
	}
	for _, value := range []string{facts.RequestedPath, facts.ResolvedPath} {
		if normalized, err := sourcespec.NormalizeSubpath(value); err != nil || normalized != value {
			return "", ErrInvalidRequest
		}
	}
	if facts.ResolvedPath != source.PackageSubpath || (facts.ResolvedPath != "" && path.Base(facts.ResolvedPath) != facts.Name) {
		return "", ErrInvalidRequest
	}
	key := "github-skill:" + strings.ToLower(repository) + ":" + facts.Name + ":" + facts.RequestedPath
	if source.StableSourceKey != key {
		return "", ErrInvalidRequest
	}
	return key, nil
}

func validAcquiredSkillName(name string) bool {
	if size := utf8.RuneCountInString(name); size < 1 || size > 64 || strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") || strings.Contains(name, "--") {
		return false
	}
	for _, character := range name {
		if character != '-' && !unicode.IsLower(character) && !unicode.IsNumber(character) {
			return false
		}
	}
	return true
}
