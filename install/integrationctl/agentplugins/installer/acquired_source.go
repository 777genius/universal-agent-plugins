package installer

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func acquiredDirectSourceKey(source domain.SourceIdentity, portable bool) (string, error) {
	var endpoint string
	var err error
	switch source.SourceKind {
	case "git":
		endpoint, _, err = sourcespec.NormalizeGitURL(source.SourceURL)
		if !sourcespec.ValidCommit(source.ResolvedRevision) || source.ArchiveSHA256 != "" {
			return "", ErrInvalidRequest
		}
	case "archive":
		endpoint, err = sourcespec.NormalizeArchiveURL(source.SourceURL)
		if !portable && filepath.IsAbs(source.SourceURL) {
			endpoint, err = filepath.Clean(source.SourceURL), nil
		}
		if !sourcespec.ValidSHA256(source.ArchiveSHA256) || source.ResolvedRevision != "sha256:"+source.ArchiveSHA256 {
			return "", ErrInvalidRequest
		}
	default:
		return "", ErrInvalidRequest
	}
	if err != nil || endpoint != source.SourceURL || source.Repository != "" {
		return "", ErrInvalidRequest
	}
	if _, err := sourcespec.NormalizeSubpath(source.PackageSubpath); err != nil {
		return "", ErrInvalidRequest
	}
	key := sourcespec.Key(sourcespec.Kind(source.SourceKind), endpoint, source.PackageSubpath)
	if key != source.CanonicalSource {
		return "", fmt.Errorf("%w: acquired direct source identity differs", ErrInvalidRequest)
	}
	return key, nil
}

func validateAcquiredSource(acquired *domain.AcquiredSnapshot, portable bool) (string, error) {
	source := acquired.Source
	if source.SourceKind == "https-skill" || source.WebSkill != nil || source.SourceBindingHint == "uap-dependency-web-skill" {
		if acquired.OriginMode != domain.OriginModeDirect {
			return "", ErrInvalidRequest
		}
		return acquiredWebSkillSourceKey(source, portable)
	}
	if source.SourceKind != "" {
		if acquired.OriginMode != domain.OriginModeDirect || source.Skill != nil || source.StableSourceKey != "" || source.SourceBindingHint == "uap-dependency-skill" {
			return "", ErrInvalidRequest
		}
		return acquiredDirectSourceKey(source, portable)
	}
	expected := "github:" + source.Repository + "@" + source.ResolvedRevision
	if source.PackageSubpath != "" {
		expected += "//" + source.PackageSubpath
	}
	if source.Repository == "" || !sourcespec.ValidCommit(source.ResolvedRevision) || (acquired.OriginMode == domain.OriginModeDirect && source.RequestedSource != expected) || source.CanonicalSource != expected {
		return "", fmt.Errorf("%w: acquired source coordinates disagree", ErrInvalidRequest)
	}
	if source.Skill != nil || source.StableSourceKey != "" || source.SourceBindingHint == "uap-dependency-skill" {
		if acquired.OriginMode != domain.OriginModeDirect {
			return "", ErrInvalidRequest
		}
		return acquiredSkillSourceKey(source, portable)
	}
	return "github:" + strings.ToLower(source.Repository) + ":" + source.PackageSubpath, nil
}
