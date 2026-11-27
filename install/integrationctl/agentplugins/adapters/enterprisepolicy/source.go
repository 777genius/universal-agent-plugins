package enterprisepolicy

import (
	"errors"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

type normalizedSource struct {
	facts           domain.InstallPolicySource
	authority       string
	repositoryKnown bool
}

var errSourceAuthority = errors.New("source has no unambiguous policy authority")
var errSourceMismatch = errors.New("request source disagrees with the actual package source")

func normalizePolicySource(source domain.SourceIdentity, origin domain.OriginMode) (normalizedSource, error) {
	if origin == domain.OriginModeDirectory {
		return normalizeDirectory(source)
	}
	switch source.SourceKind {
	case "git":
		return normalizePolicyGit(source)
	case "archive":
		if localPolicyPath(source.SourceURL) {
			return localPolicySource(source.SourceURL), nil
		}
		return normalizePolicyRemote("archive", source.SourceURL)
	case "https-skill", "web":
		raw := source.SourceURL
		if source.WebSkill != nil {
			if raw != "" && raw != source.WebSkill.URL {
				return normalizedSource{}, errSourceAuthority
			}
			raw = source.WebSkill.URL
		}
		return normalizePolicyRemote("web", raw)
	case "local", sourcespec.LocalSkillKind:
		return normalizePolicyLocal(source)
	case "":
		return normalizeLegacySource(source)
	default:
		return normalizedSource{}, errSourceAuthority
	}
}

func normalizeLegacySource(source domain.SourceIdentity) (normalizedSource, error) {
	if source.SourceBindingHint == "direct-local" || source.SourceBindingHint == "uap-dependency-local-skill" {
		return normalizePolicyLocal(source)
	}
	if source.WebSkill != nil {
		return normalizePolicyRemote("web", source.WebSkill.URL)
	}
	if source.Repository != "" || source.SourceURL != "" {
		return normalizePolicyGit(source)
	}
	return normalizedSource{}, errSourceAuthority
}

func normalizeDirectory(source domain.SourceIdentity) (normalizedSource, error) {
	if source.SourceKind != "" && source.SourceKind != "directory" && source.SourceKind != "registry" {
		return normalizedSource{}, errSourceAuthority
	}
	if source.Repository == "" {
		return normalizedSource{}, errSourceAuthority
	}
	normalized, err := normalizePolicyGit(source)
	if err != nil || normalized.facts.Host != "github.com" || !normalized.repositoryKnown {
		return normalizedSource{}, errSourceAuthority
	}
	expected, ok := canonicalPolicyRepository("github.com/" + source.Repository)
	if !ok || normalized.facts.Repository != expected {
		return normalizedSource{}, errSourceAuthority
	}
	normalized.facts.Kind = "directory"
	return normalized, nil
}

func normalizePolicyLocal(source domain.SourceIdentity) (normalizedSource, error) {
	for _, candidate := range []string{source.SourceURL, source.CanonicalSource} {
		if localPolicyPath(candidate) {
			return localPolicySource(candidate), nil
		}
	}
	return normalizedSource{}, errSourceAuthority
}

func localPolicyPath(value string) bool {
	return sourcespec.ValidateLocalSkillPath(filepath.ToSlash(value)) == nil
}

func localPolicySource(path string) normalizedSource {
	return normalizedSource{facts: domain.InstallPolicySource{Kind: "local"}, authority: filepath.ToSlash(path)}
}

func normalizePolicyRemote(kind, raw string) (normalizedSource, error) {
	endpoint, err := sourcespec.NormalizeArchiveURL(raw)
	if err != nil {
		return normalizedSource{}, errSourceAuthority
	}
	u, err := url.Parse(endpoint)
	if err != nil || !domain.ValidPolicyHost(strings.ToLower(u.Hostname())) {
		return normalizedSource{}, errSourceAuthority
	}
	return normalizedSource{facts: domain.InstallPolicySource{Kind: kind, Host: strings.ToLower(u.Hostname())}, authority: endpoint}, nil
}

func normalizePolicyGit(source domain.SourceIdentity) (normalizedSource, error) {
	raw := source.SourceURL
	if raw == "" && source.Repository != "" {
		raw = "https://github.com/" + source.Repository + ".git"
	}
	endpoint, _, err := sourcespec.NormalizeGitURL(strings.TrimPrefix(raw, "git+"))
	if err != nil {
		return normalizedSource{}, errSourceAuthority
	}
	u, err := policyGitURL(endpoint)
	if err != nil || !domain.ValidPolicyHost(strings.ToLower(u.Hostname())) {
		return normalizedSource{}, errSourceAuthority
	}
	host := strings.ToLower(u.Hostname())
	result := normalizedSource{facts: domain.InstallPolicySource{Kind: "git", Host: host}, authority: endpoint}
	repository, ok := policyRepositoryFromURL(u)
	if ok {
		result.facts.Repository, result.repositoryKnown, result.authority = repository, true, repository
	}
	return result, nil
}

func policyGitURL(endpoint string) (*url.URL, error) {
	if strings.Contains(endpoint, "://") {
		return url.Parse(endpoint)
	}
	host, repo, ok := strings.Cut(endpoint, ":")
	if !ok {
		return nil, errSourceAuthority
	}
	if _, after, hasUser := strings.Cut(host, "@"); hasUser {
		host = after
	}
	return &url.URL{Scheme: "ssh", Host: host, Path: "/" + strings.TrimPrefix(repo, "/")}, nil
}

func policyRepositoryFromURL(u *url.URL) (string, bool) {
	defaultPort := "443"
	if u.Scheme == "ssh" {
		defaultPort = "22"
	}
	if u.Port() != "" && u.Port() != defaultPort {
		return "", false
	}
	if u.Scheme == "https" && u.User != nil {
		return "", false
	}
	// Policy names are intentionally narrower than arbitrary remote URL paths.
	// Never decode an escaped slash into a different repository boundary.
	if strings.Contains(u.EscapedPath(), "%") || !validRepositoryPath(strings.TrimPrefix(u.Path, "/"), false) {
		return "", false
	}
	return canonicalPolicyRepository(strings.ToLower(u.Hostname()) + "/" + strings.TrimPrefix(u.Path, "/"))
}

func sourceIdentityPresent(s domain.SourceIdentity) bool {
	return s.SourceKind != "" || s.SourceURL != "" || s.Repository != "" || s.CanonicalSource != "" ||
		s.RequestedSource != "" || s.SourceBindingHint != "" || s.WebSkill != nil || s.Skill != nil
}

func policyRequestSource(req domain.InstallPolicyRequest) (domain.SourceIdentity, normalizedSource, error) {
	source := req.Source
	if !sourceIdentityPresent(source) && req.Envelope != nil {
		source = req.Envelope.Source
	}
	normalized, err := normalizePolicySource(source, req.OriginMode)
	if err != nil {
		return source, normalized, err
	}
	if req.Envelope != nil && sourceIdentityPresent(req.Source) && sourceIdentityPresent(req.Envelope.Source) {
		other, otherErr := normalizePolicySource(req.Envelope.Source, req.OriginMode)
		if otherErr != nil || !samePolicySource(req.Source, normalized, req.Envelope.Source, other) {
			return source, normalizedSource{}, errSourceMismatch
		}
	}
	return source, normalized, nil
}

func samePolicySource(a domain.SourceIdentity, an normalizedSource, b domain.SourceIdentity, bn normalizedSource) bool {
	if an.facts.Kind != bn.facts.Kind || an.authority != bn.authority || a.PackageSubpath != b.PackageSubpath {
		return false
	}
	for _, pair := range [][2]string{{a.ResolvedRevision, b.ResolvedRevision}, {a.ArchiveSHA256, b.ArchiveSHA256}} {
		if pair[0] != "" && pair[1] != "" && pair[0] != pair[1] {
			return false
		}
	}
	return true
}
