// Package skillsource adapts explicit raw Agent Skills sources into the existing
// standard package acquisition boundary. It never executes Skill content.
package skillsource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
)

type Acquirer struct {
	sourceacquisition.Acquirer
	Policy ScanPolicy
}
type Source struct {
	Repository, RequestedSelector, RequestedPath, Commit string
	ExactPath                                            bool
}
type sourceRequest struct {
	source  Source
	urlTail string
	file    bool
}

// Resolve disambiguates every possible ref/path split against actual Git refs.
// An explicit ref fixes the boundary; competing valid splits fail closed.
func (a Acquirer) Resolve(ctx context.Context, input, explicitRef string) (Source, error) {
	ctx, cancel := context.WithTimeout(ctx, a.Policy.bounded().Deadline)
	defer cancel()
	request, err := parseSourceInput(input)
	if err != nil {
		return Source{}, err
	}
	var source Source
	if request.urlTail != "" {
		source, err = a.resolveURLSource(ctx, request, explicitRef)
	} else {
		source, err = a.resolveRepositorySource(ctx, request.source, explicitRef)
	}
	if err != nil {
		return source, err
	}
	if err = sourceacquisition.ValidateSkillPath(source.RequestedPath); err != nil {
		return source, err
	}
	return source, nil
}
func parseSourceInput(input string) (sourceRequest, error) {
	var request sourceRequest
	if len(input) > 8192 || strings.TrimSpace(input) != input {
		return request, fmt.Errorf("invalid GitHub source")
	}
	var err error
	if strings.Contains(input, "://") {
		request, err = parseGitHubURL(input)
	} else {
		base, sub, has := strings.Cut(input, "//")
		request.source.RequestedPath = sub
		request.source.ExactPath = has
		repo, selector, hasRef := strings.Cut(base, "@")
		request.source.Repository = strings.TrimSuffix(repo, ".git")
		if hasRef && selector == "" {
			return request, fmt.Errorf("git selector is required after @")
		}
		request.source.RequestedSelector = selector
	}
	if err != nil {
		return request, err
	}
	request.source.Repository, err = sourceacquisition.CanonicalGitHubRepository(request.source.Repository)
	return request, err
}
func parseGitHubURL(input string) (sourceRequest, error) {
	var request sourceRequest
	u, err := url.Parse(input)
	if err != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return request, fmt.Errorf("use a public HTTPS GitHub repository, folder or SKILL.md URL")
	}
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), "/"), "/")
	if len(parts) < 2 {
		return request, fmt.Errorf("GitHub source needs owner/repository")
	}
	request.source.Repository = parts[0] + "/" + strings.TrimSuffix(parts[1], ".git")
	switch strings.ToLower(u.Host) {
	case "github.com":
		if len(parts) == 2 {
			return request, nil
		}
		if len(parts) < 4 || (parts[2] != "tree" && parts[2] != "blob") {
			return request, fmt.Errorf("unsupported GitHub URL")
		}
		request.file = parts[2] == "blob"
		request.urlTail = strings.Join(parts[3:], "/")
	case "raw.githubusercontent.com":
		if len(parts) < 4 {
			return request, fmt.Errorf("raw URL requires a ref and SKILL.md path")
		}
		request.file = true
		request.urlTail = strings.Join(parts[2:], "/")
	default:
		return request, fmt.Errorf("only public GitHub sources are supported")
	}
	return request, nil
}
func (a Acquirer) resolveURLSource(ctx context.Context, request sourceRequest, explicitRef string) (Source, error) {
	source, tail := request.source, request.urlTail
	if request.file {
		if path.Base(tail) != "SKILL.md" {
			return source, fmt.Errorf("file URL must select SKILL.md")
		}
		source.ExactPath = true
		tail = strings.TrimSuffix(tail, "/SKILL.md")
	}
	resolved, p, err := a.ResolveGitHubURLPath(ctx, source.Repository, tail, explicitRef)
	if err != nil {
		return source, err
	}
	source.RequestedSelector = resolved.RequestedRef
	if resolved.ResolvedRef != "" {
		source.RequestedSelector = resolved.ResolvedRef
	}
	source.RequestedPath = p
	source.Commit = resolved.Commit
	return source, nil
}
func (a Acquirer) resolveRepositorySource(ctx context.Context, source Source, explicitRef string) (Source, error) {
	if explicitRef != "" {
		if source.RequestedSelector != "" && source.RequestedSelector != explicitRef {
			return source, fmt.Errorf("source selector conflicts with --ref")
		}
		source.RequestedSelector = explicitRef
	}
	if source.RequestedSelector == "" {
		source.RequestedSelector = "default"
	}
	var resolved sourceacquisition.RefResolution
	var err error
	if explicitRef != "" {
		resolved, err = a.ResolveGitHubRef(ctx, source.Repository, explicitRef)
	} else {
		resolved, err = a.ResolveGitHubSelector(ctx, source.Repository, source.RequestedSelector)
	}
	if err != nil {
		return source, err
	}
	if explicitRef != "" {
		source.RequestedSelector = resolved.ResolvedRef
	} else if resolved.DefaultBranch != "" {
		source.RequestedSelector = "default"
	}
	source.Commit = resolved.Commit
	return source, nil
}
func digest(value []byte) string {
	sum := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(sum[:])
}
