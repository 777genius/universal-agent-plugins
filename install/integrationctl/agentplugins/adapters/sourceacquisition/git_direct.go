package sourceacquisition

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func (acquirer Acquirer) gitEndpoint(source sourcespec.Git) (string, string, error) {
	endpoint := source.URL
	if endpoint == "" && source.Repository != "" {
		endpoint = "https://github.com/" + source.Repository + ".git"
	}
	normalized, repository, err := sourcespec.NormalizeGitURL(endpoint)
	if err != nil {
		return "", "", err
	}
	if source.Repository != "" && !strings.EqualFold(repository, source.Repository) {
		return "", "", errors.New("git source repository disagrees with its endpoint")
	}
	return normalized, repository, nil
}

func (acquirer Acquirer) directEndpoint(endpoint, repository string) string {
	if repository != "" && acquirer.URLForRepo != nil {
		return acquirer.URLForRepo(repository)
	}
	return endpoint
}

func (acquirer Acquirer) AcquireGit(ctx context.Context, source sourcespec.Git, commit, expectedTreeDigest string) (domain.PackageSnapshot, error) {
	endpoint, repository, err := acquirer.gitEndpoint(source)
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	if !sourcespec.ValidCommit(commit) || source.Browser != nil {
		return domain.PackageSnapshot{}, errors.New("direct Git acquisition requires resolved immutable coordinates")
	}
	packagePath, err := sourcespec.NormalizeSubpath(source.Path)
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	snapshot, err := acquirer.acquireGitTransport(ctx, acquirer.directEndpoint(endpoint, repository), endpoint, commit, packagePath, true)
	if err != nil {
		if ctx.Err() != nil {
			return domain.PackageSnapshot{}, ctx.Err()
		}
		var selected *gitPackagePathError
		if errors.As(err, &selected) {
			return domain.PackageSnapshot{}, fmt.Errorf("acquire direct Git source: package path %q: %s", packagePath, selected.reason)
		}
		return domain.PackageSnapshot{}, fmt.Errorf("acquire direct Git source: immutable fetch or package snapshot failed; verify native Git credentials and repository access")
	}
	if expectedTreeDigest != "" && snapshot.TreeDigest != expectedTreeDigest {
		_ = packagedigest.Remove(snapshot)
		return domain.PackageSnapshot{}, errors.New("direct Git package tree does not match the recorded digest")
	}
	snapshot.Source = domain.SourceIdentity{SourceKind: "git", SourceURL: endpoint, RequestedSource: gitInput(endpoint, commit, packagePath), CanonicalSource: sourcespec.Key(sourcespec.KindGit, endpoint, packagePath), Repository: repository, PackageSubpath: packagePath, ResolvedRevision: commit, SourceBindingHint: "direct-git"}
	return snapshot, nil
}

func gitInput(endpoint, selector, packagePath string) string {
	value := endpoint + "#" + selector
	if packagePath != "" {
		value += "//" + packagePath
	}
	return value
}

func (acquirer Acquirer) DiscoverGitPackages(ctx context.Context, source sourcespec.Git, commit string) ([]string, error) {
	endpoint, repository, err := acquirer.gitEndpoint(source)
	if err != nil {
		return nil, err
	}
	if !sourcespec.ValidCommit(commit) || source.Browser != nil {
		return nil, errors.New("package discovery requires a full immutable Git commit")
	}
	paths, err := acquirer.discoverGitPackages(ctx, acquirer.directEndpoint(endpoint, repository), endpoint, commit, true)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("inspect direct Git source package roots failed; choose an explicit package path or verify native Git access")
	}
	return paths, nil
}

func (acquirer Acquirer) ResolveGitSource(ctx context.Context, source sourcespec.Git) (ResolvedGit, error) {
	if source.Browser != nil {
		return acquirer.resolveBrowserGit(ctx, source)
	}
	selector, exact := source.Ref, true
	if source.Commit != "" {
		selector, exact = source.Commit, false
	} else if selector == "" {
		selector, exact = "default", false
	}
	resolution, err := acquirer.ResolveGit(ctx, source, selector, exact)
	if err != nil {
		return ResolvedGit{}, err
	}
	source.Commit = resolution.Commit
	return ResolvedGit{Git: source, Resolution: resolution}, nil
}

type directGitWorkspace struct {
	root, repository string
	acquirer         Acquirer
}

func (acquirer Acquirer) newDirectWorkspace(ctx context.Context) (directGitWorkspace, error) {
	root, err := os.MkdirTemp(acquirer.TempRoot, "agentplugins-direct-ref-*")
	if err != nil {
		return directGitWorkspace{}, errors.New("create private Git transport workspace failed")
	}
	workspace := directGitWorkspace{root: root, repository: filepath.Join(root, "repository"), acquirer: acquirer}
	if _, err := workspace.run(ctx, false, "init", "--quiet", workspace.repository); err != nil {
		_ = os.RemoveAll(root)
		return directGitWorkspace{}, errors.New("initialize private Git transport workspace failed")
	}
	return workspace, nil
}
func (workspace directGitWorkspace) run(ctx context.Context, native bool, args ...string) ([]byte, error) {
	return workspace.acquirer.runner().Run(ctx, Command{Dir: workspace.root, Args: args, NativeCredentials: native, mappedLocal: native && workspace.acquirer.mappedLocalArgs(args), MaxOutputBytes: maxGitHubDiscoveryMetadataBytes})
}
func (workspace directGitWorkspace) remote(ctx context.Context, args ...string) ([]byte, error) {
	return workspace.run(ctx, true, append([]string{"-C", workspace.repository}, args...)...)
}

func (acquirer Acquirer) ResolveGit(ctx context.Context, source sourcespec.Git, selector string, exactRef bool) (RefResolution, error) {
	endpoint, repository, err := acquirer.gitEndpoint(source)
	if err != nil {
		return RefResolution{}, err
	}
	if !exactRef && sourcespec.ValidCommit(selector) {
		return RefResolution{RequestedRef: selector, Commit: selector}, nil
	}
	if err := sourcespec.ValidateRef(selector); err != nil {
		return RefResolution{}, err
	}
	workspace, err := acquirer.newDirectWorkspace(ctx)
	if err != nil {
		return RefResolution{}, err
	}
	defer func() { _ = os.RemoveAll(workspace.root) }()
	endpoint = acquirer.directEndpoint(endpoint, repository)
	if !exactRef && (selector == "default" || selector == "HEAD") {
		output, err := workspace.remote(ctx, "ls-remote", "--symref", endpoint, "HEAD")
		if err != nil {
			return RefResolution{}, directRefError(ctx)
		}
		branch, commit, err := parseDefaultBranch(output)
		if err != nil {
			return RefResolution{}, err
		}
		return RefResolution{RequestedRef: selector, ResolvedRef: branch, Commit: commit, DefaultBranch: branch}, nil
	}
	patterns := []string{selector}
	resolvedRef := selector
	if !strings.HasPrefix(selector, "refs/") {
		patterns = []string{"refs/heads/" + selector, "refs/tags/" + selector}
		resolvedRef = ""
	} else if !strings.HasPrefix(selector, "refs/heads/") && !strings.HasPrefix(selector, "refs/tags/") {
		return RefResolution{}, errors.New("only branches and tags are supported Git refs")
	}
	for _, ref := range patterns {
		if _, err := workspace.run(ctx, false, "-C", workspace.repository, "check-ref-format", ref); err != nil {
			return RefResolution{}, errors.New("invalid exact Git ref")
		}
	}
	output, err := workspace.remote(ctx, append([]string{"ls-remote", "--refs", endpoint}, patterns...)...)
	if err != nil {
		return RefResolution{}, directRefError(ctx)
	}
	match, err := selectExactGitRef(output, patterns, selector, resolvedRef)
	if err != nil {
		return RefResolution{}, err
	}
	commit, err := workspace.commit(ctx, endpoint, match)
	if err != nil {
		return RefResolution{}, err
	}
	return RefResolution{RequestedRef: selector, ResolvedRef: match.name, Commit: commit}, nil
}

func (workspace directGitWorkspace) commit(ctx context.Context, endpoint string, ref remoteRef) (string, error) {
	if strings.HasPrefix(ref.name, "refs/heads/") {
		return ref.object, nil
	}
	if _, err := workspace.remote(ctx, "fetch", "--quiet", "--depth=1", "--no-tags", "--no-recurse-submodules", endpoint, ref.object); err != nil {
		return "", directRefError(ctx)
	}
	output, err := workspace.run(ctx, false, "-C", workspace.repository, "rev-parse", "--verify", "FETCH_HEAD^{commit}")
	if err != nil || !sourcespec.ValidCommit(strings.TrimSpace(string(output))) {
		return "", errors.New("git tag did not resolve to a commit")
	}
	return strings.TrimSpace(string(output)), nil
}

func (acquirer Acquirer) resolveBrowserGit(ctx context.Context, source sourcespec.Git) (ResolvedGit, error) {
	endpoint, repository, err := acquirer.gitEndpoint(source)
	if err != nil {
		return ResolvedGit{}, err
	}
	suffix := source.Browser.Suffix
	first, remainder, _ := strings.Cut(suffix, "/")
	if sourcespec.ValidCommit(first) {
		source.Browser = nil
		source.Commit = first
		source.Path, err = sourcespec.NormalizeSubpath(remainder)
		return ResolvedGit{Git: source, Resolution: RefResolution{RequestedRef: first, Commit: first}}, err
	}
	workspace, err := acquirer.newDirectWorkspace(ctx)
	if err != nil {
		return ResolvedGit{}, err
	}
	defer func() { _ = os.RemoveAll(workspace.root) }()
	transportEndpoint := acquirer.directEndpoint(endpoint, repository)
	output, err := workspace.remote(ctx, "ls-remote", "--refs", transportEndpoint, "refs/heads/*", "refs/tags/*")
	if err != nil {
		return ResolvedGit{}, directRefError(ctx)
	}
	type candidate struct {
		ref  remoteRef
		path string
	}
	var candidates []candidate
	for _, ref := range parseRefs(output) {
		name := strings.TrimPrefix(strings.TrimPrefix(ref.name, "refs/heads/"), "refs/tags/")
		if suffix != name && !strings.HasPrefix(suffix, name+"/") {
			continue
		}
		packagePath := strings.TrimPrefix(strings.TrimPrefix(suffix, name), "/")
		if _, err := sourcespec.NormalizeSubpath(packagePath); err == nil {
			candidates = append(candidates, candidate{ref: ref, path: packagePath})
		}
	}
	if len(candidates) != 1 {
		return ResolvedGit{}, errors.New("browser tree selector is absent or ambiguous; use the clone URL#refs/heads/branch//package")
	}
	selected := candidates[0]
	commit, err := workspace.commit(ctx, transportEndpoint, selected.ref)
	if err != nil {
		return ResolvedGit{}, err
	}
	source.Browser = nil
	source.Ref = selected.ref.name
	source.Commit = commit
	source.Path = selected.path
	return ResolvedGit{Git: source, Resolution: RefResolution{RequestedRef: selected.ref.name, ResolvedRef: selected.ref.name, Commit: commit}}, nil
}

func directRefError(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return errors.New("resolve direct Git ref failed; verify native Git credentials and repository access")
}

// URLForRepo is an explicit dependency-injection seam used by local Git fixtures;
// accepted user endpoints still pass strict HTTPS/SSH parsing before mapping.
func (acquirer Acquirer) mappedLocalEndpoint(endpoint string) bool {
	return acquirer.URLForRepo != nil && (filepath.IsAbs(endpoint) || strings.HasPrefix(endpoint, "file://"))
}
func (acquirer Acquirer) mappedLocalArgs(args []string) bool {
	for index, argument := range args {
		if index > 0 && args[index-1] == "-C" {
			continue
		}
		if acquirer.mappedLocalEndpoint(argument) {
			return true
		}
	}
	return false
}
