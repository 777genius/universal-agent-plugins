package skillsource

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

// Location preserves historical GitHub labels and identifies other endpoints.
func (source Source) Location() string {
	if source.URL != "" {
		return source.URL
	}
	return source.Repository
}

func (source Source) Git() sourcespec.Git {
	return sourcespec.Git{Repository: source.Repository, URL: source.URL, Path: source.RequestedPath}
}

func nativeSourceInput(input string) bool {
	if strings.HasPrefix(input, "git+") || strings.HasPrefix(input, "ssh://") || strings.Contains(input, "#") || strings.Contains(input, ".git//") {
		return true
	}
	if !strings.Contains(input, "://") {
		return strings.Contains(input, ":")
	}
	u, err := url.Parse(input)
	return err == nil && !strings.EqualFold(u.Hostname(), "github.com") && !strings.EqualFold(u.Hostname(), "raw.githubusercontent.com")
}

func (a Acquirer) resolveNativeSource(ctx context.Context, git sourcespec.Git, explicitRef string) (Source, error) {
	source := Source{Repository: git.Repository, URL: git.URL, RequestedPath: git.Path, ExactPath: git.Path != ""}
	if source.Repository != "" {
		source.URL = ""
	}
	if explicitRef != "" {
		if (git.Ref != "" && git.Ref != explicitRef) || (git.Commit != "" && git.Commit != explicitRef) {
			return source, fmt.Errorf("source selector conflicts with --ref")
		}
		if git.Browser != nil {
			short := strings.TrimPrefix(strings.TrimPrefix(explicitRef, "refs/heads/"), "refs/tags/")
			tail := git.Browser.Suffix
			if tail != short && !strings.HasPrefix(tail, short+"/") {
				return source, fmt.Errorf("URL does not match explicit ref")
			}
			git.Path = strings.TrimPrefix(strings.TrimPrefix(tail, short), "/")
			git.Browser = nil
		}
		git.Ref, git.Commit = explicitRef, ""
	}
	resolved, err := a.ResolveGitSource(ctx, git)
	if err != nil {
		return source, err
	}
	source.RequestedPath, source.Commit = resolved.Git.Path, resolved.Resolution.Commit
	source.ExactPath = source.RequestedPath != ""
	source.RequestedSelector = resolved.Resolution.RequestedRef
	if resolved.Resolution.ResolvedRef != "" && resolved.Resolution.DefaultBranch == "" {
		source.RequestedSelector = resolved.Resolution.ResolvedRef
	}
	if resolved.Resolution.DefaultBranch != "" {
		source.RequestedSelector = "default"
	}
	return source, nil
}

func sourceRootName(source Source) string {
	location := source.Location()
	if source.URL != "" {
		if u, err := url.Parse(source.URL); err == nil && u.Scheme != "" && u.Host != "" {
			location = u.Path
		} else if _, after, ok := strings.Cut(location, ":"); ok {
			location = after
		}
	}
	return strings.TrimSuffix(path.Base(location), ".git")
}
