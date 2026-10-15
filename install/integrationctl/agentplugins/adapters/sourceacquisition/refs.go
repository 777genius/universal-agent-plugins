package sourceacquisition

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type RefResolution struct {
	RequestedRef  string
	ResolvedRef   string
	Commit        string
	DefaultBranch string
}

// ResolveGitHubSelector validates and resolves a direct Git selector using the
// same isolated runner as acquisition. Arguments are never shell-interpolated.
// Exact refs are compared to Git output; short refs require one branch/tag hit.
func (acquirer Acquirer) ResolveGitHubSelector(ctx context.Context, repository, selector string) (RefResolution, error) {
	return acquirer.resolveGitHubSelector(ctx, repository, selector, false)
}

// ResolveGitHubRef always treats ref as a branch/tag name, even for default or SHA-shaped names.
func (acquirer Acquirer) ResolveGitHubRef(ctx context.Context, repository, ref string) (RefResolution, error) {
	return acquirer.resolveGitHubSelector(ctx, repository, ref, true)
}

func (acquirer Acquirer) resolveGitHubSelector(ctx context.Context, repository, selector string, exactRef bool) (RefResolution, error) {
	if !repositoryPattern.MatchString(repository) {
		return RefResolution{}, fmt.Errorf("github repository must be owner/repo")
	}
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return RefResolution{}, fmt.Errorf("git selector is required")
	}
	if !exactRef && fullSHA.MatchString(selector) {
		return RefResolution{RequestedRef: selector, Commit: selector}, nil
	}
	url := "https://github.com/" + repository + ".git"
	if acquirer.URLForRepo != nil {
		url = acquirer.URLForRepo(repository)
	}
	run := func(args ...string) ([]byte, error) {
		return acquirer.runner().Run(ctx, Command{Dir: acquirer.TempRoot, Args: args, MaxOutputBytes: maxGitHubDiscoveryMetadataBytes})
	}
	if !exactRef && (selector == "HEAD" || selector == "default") {
		output, err := run("ls-remote", "--symref", url, "HEAD")
		if err != nil {
			return RefResolution{}, gitAcquisitionFailure(repository, selector, "resolve repository default branch")
		}
		branch, commit, err := parseDefaultBranch(output)
		if err != nil {
			return RefResolution{}, err
		}
		return RefResolution{RequestedRef: selector, ResolvedRef: branch, Commit: commit, DefaultBranch: branch}, nil
	}
	return acquirer.resolveNamedGitRef(ctx, repository, selector, url, run)
}

type remoteRef struct{ object, name string }

func parseRefs(output []byte) []remoteRef {
	var refs []remoteRef
	for _, line := range strings.Split(string(output), "\n") {
		object, name, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if ok && fullSHA.MatchString(object) && strings.HasPrefix(name, "refs/") {
			refs = append(refs, remoteRef{object: object, name: name})
		}
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].name < refs[j].name })
	return refs
}

func parseDefaultBranch(output []byte) (string, string, error) {
	branch, commit := "", ""
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "ref: ") {
			ref, rest, ok := strings.Cut(strings.TrimSpace(strings.TrimPrefix(line, "ref: ")), "\t")
			if ok && rest == "HEAD" && strings.HasPrefix(ref, "refs/heads/") && len(ref) > len("refs/heads/") {
				branch = ref
			}
		}
		object, name, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if ok && name == "HEAD" && fullSHA.MatchString(object) {
			commit = object
		}
	}
	if branch == "" || commit == "" {
		return "", "", fmt.Errorf("git did not report one default branch and commit")
	}
	return branch, commit, nil
}

func (acquirer Acquirer) resolveCommit(ctx context.Context, url, repository, object string) (string, error) {
	if !fullSHA.MatchString(object) {
		return "", fmt.Errorf("git returned an invalid object ID")
	}
	tempRoot, err := os.MkdirTemp(acquirer.TempRoot, "agentplugins-ref-*")
	if err != nil {
		return "", gitAcquisitionFailure(repository, object, "create temporary ref workspace")
	}
	defer func() { _ = os.RemoveAll(tempRoot) }()
	repoRoot := filepath.Join(tempRoot, "repository")
	run := func(args ...string) ([]byte, error) {
		return acquirer.runner().Run(ctx, Command{Dir: tempRoot, Args: args, MaxOutputBytes: maxGitHubDiscoveryMetadataBytes})
	}
	if _, err := run("init", "--quiet", repoRoot); err != nil {
		return "", gitAcquisitionFailure(repository, object, "initialize ref repository")
	}
	if _, err := run("-C", repoRoot, "remote", "add", "origin", url); err != nil {
		return "", gitAcquisitionFailure(repository, object, "configure ref remote")
	}
	if _, err := run("-C", repoRoot, "fetch", "--quiet", "--depth=1", "--no-tags", "origin", object); err != nil {
		return "", gitAcquisitionFailure(repository, object, "fetch exact Git object")
	}
	output, err := run("-C", repoRoot, "rev-parse", "FETCH_HEAD^{commit}")
	if err != nil {
		return "", gitAcquisitionFailure(repository, object, "resolve annotated tag commit")
	}
	commit := strings.TrimSpace(string(output))
	if !fullSHA.MatchString(commit) {
		return "", fmt.Errorf("git returned an invalid commit for %s", object)
	}
	return commit, nil
}

func (acquirer Acquirer) resolveNamedGitRef(ctx context.Context, repository, selector, url string, run func(...string) ([]byte, error)) (RefResolution, error) {
	var patterns []string
	resolvedRef := ""
	switch {
	case strings.HasPrefix(selector, "refs/heads/"), strings.HasPrefix(selector, "refs/tags/"):
		patterns = []string{selector}
		resolvedRef = selector
	case strings.HasPrefix(selector, "refs/"):
		return RefResolution{}, fmt.Errorf("only refs/heads/ and refs/tags/ exact refs are supported")
	default:
		patterns = []string{"refs/heads/" + selector, "refs/tags/" + selector}
	}
	for _, ref := range patterns {
		if _, err := run("check-ref-format", ref); err != nil {
			return RefResolution{}, fmt.Errorf("invalid exact Git ref %q", selector)
		}
	}
	output, err := run(append([]string{"ls-remote", "--refs", url}, patterns...)...)
	if err != nil {
		return RefResolution{}, gitAcquisitionFailure(repository, selector, "resolve Git ref")
	}
	match, err := selectExactGitRef(output, patterns, selector, resolvedRef)
	if err != nil {
		return RefResolution{}, err
	}
	commit, err := acquirer.resolveCommit(ctx, url, repository, match.object)
	if err != nil {
		return RefResolution{}, err
	}
	return RefResolution{RequestedRef: selector, ResolvedRef: match.name, Commit: commit}, nil
}

func selectExactGitRef(output []byte, patterns []string, selector, resolvedRef string) (remoteRef, error) {
	matches := parseRefs(output)
	for _, match := range matches {
		exact := false
		for _, ref := range patterns {
			if ref == match.name {
				exact = true
			}
		}
		if !exact {
			return remoteRef{}, fmt.Errorf("git returned an unexpected ref %q", match.name)
		}
	}
	if len(matches) == 0 {
		return remoteRef{}, fmt.Errorf("git selector %q matched no branch or tag", selector)
	}
	if resolvedRef == "" && len(matches) != 1 {
		names := make([]string, 0, len(matches))
		for _, match := range matches {
			names = append(names, match.name)
		}
		return remoteRef{}, fmt.Errorf("git selector %q is ambiguous between %s", selector, strings.Join(names, " and "))
	}
	match := matches[0]
	if resolvedRef != "" && match.name != resolvedRef {
		return remoteRef{}, fmt.Errorf("git returned ref %q for exact selector %q", match.name, selector)
	}
	return match, nil
}
