// Package sourceacquisition resolves direct package sources into one sealed,
// digest-verified local snapshot. It never executes package content.
package sourceacquisition

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	repositoryPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38}[A-Za-z0-9])?/[A-Za-z0-9](?:[A-Za-z0-9._-]{0,98}[A-Za-z0-9])?$`)
	fullSHA           = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

const (
	maxRootManifestMetadataBytes    = 4 << 10
	maxGitHubDiscoveryMetadataBytes = 1 << 20
)

type Acquirer struct {
	TempRoot   string
	Runner     Runner
	Digester   packagedigest.Builder
	URLForRepo func(string) string
	Now        func() time.Time
	Reporter   Reporter
}

// DiscoverGitHubPackages returns repository-relative package roots at an
// immutable revision without checking out or executing repository content.
// An empty string is returned when a portable or native package manifest exists
// at repository root; callers must then validate that root and must not silently
// fall back to a nested package when the root manifest is malformed.
func (acquirer Acquirer) DiscoverGitHubPackages(ctx context.Context, repository, revision string) ([]string, error) {
	if !repositoryPattern.MatchString(repository) {
		return nil, fmt.Errorf("GitHub repository must be owner/repo")
	}
	if !fullSHA.MatchString(revision) {
		return nil, fmt.Errorf("immutable GitHub source requires a full 40-character lowercase commit SHA")
	}
	url := "https://github.com/" + repository + ".git"
	if acquirer.URLForRepo != nil {
		url = acquirer.URLForRepo(repository)
	}
	tempRoot, err := os.MkdirTemp(acquirer.TempRoot, "agentplugins-discovery-*")
	if err != nil {
		return nil, gitAcquisitionFailure(repository, revision, "create temporary discovery workspace")
	}
	defer os.RemoveAll(tempRoot)
	repoRoot := filepath.Join(tempRoot, "repository")
	run := acquirer.gitCommand(ctx, tempRoot)
	if _, err := run("init", "--quiet", repoRoot); err != nil {
		return nil, gitAcquisitionFailure(repository, revision, "initialize discovery repository")
	}
	if _, err := run("-C", repoRoot, "remote", "add", "origin", url); err != nil {
		return nil, gitAcquisitionFailure(repository, revision, "configure discovery remote")
	}
	for _, setting := range [][]string{{"fetch.recurseSubmodules", "false"}, {"core.autocrlf", "false"}} {
		if _, err := run("-C", repoRoot, "config", setting[0], setting[1]); err != nil {
			return nil, gitAcquisitionFailure(repository, revision, "configure discovery repository")
		}
	}
	if _, err := run(gitFetchArgs(repoRoot, revision, acquirer.Reporter != nil)...); err != nil {
		return nil, gitAcquisitionFailure(repository, revision, "fetch immutable revision for package discovery")
	}
	resolved, err := run("-C", repoRoot, "rev-parse", "--verify", "FETCH_HEAD^{commit}")
	if err != nil || strings.TrimSpace(string(resolved)) != revision {
		return nil, gitAcquisitionFailure(repository, revision, "verify immutable discovery revision")
	}
	rootTree, err := acquirer.runner().Run(ctx, Command{Dir: tempRoot,
		Args: []string{"-C", repoRoot, "ls-tree", "-z", revision, "--", "plugin.json", ".codex-plugin/plugin.json"}, MaxOutputBytes: maxRootManifestMetadataBytes})
	if err != nil {
		return nil, gitAcquisitionFailure(repository, revision, "inspect repository root for Agent Plugins package")
	}
	rootPaths, err := packagePathsFromTree(rootTree)
	if err != nil {
		return nil, gitAcquisitionFailure(repository, revision, "parse repository root package candidate")
	}
	if len(rootPaths) == 1 && rootPaths[0] == "" {
		return rootPaths, nil
	}
	tree, err := acquirer.runner().Run(ctx, Command{Dir: tempRoot,
		Args: []string{"-C", repoRoot, "ls-tree", "-r", "-z", revision}, MaxOutputBytes: maxGitHubDiscoveryMetadataBytes})
	if err != nil {
		if err == processadapter.ErrStdoutLimitExceeded {
			return nil, fmt.Errorf("repository package metadata exceeds the safe auto-discovery limit; choose a package explicitly with //path")
		}
		return nil, gitAcquisitionFailure(repository, revision, "inspect repository for Agent Plugins packages")
	}
	paths, err := packagePathsFromTree(tree)
	if err != nil {
		return nil, gitAcquisitionFailure(repository, revision, "parse repository package candidates")
	}
	return paths, nil
}

func (acquirer Acquirer) AcquireGitHub(ctx context.Context, repository, revision, pluginSubpath string) (domain.PackageSnapshot, error) {
	if !repositoryPattern.MatchString(repository) {
		return domain.PackageSnapshot{}, fmt.Errorf("GitHub repository must be owner/repo")
	}
	if !fullSHA.MatchString(revision) {
		return domain.PackageSnapshot{}, fmt.Errorf("immutable GitHub source requires a full 40-character lowercase commit SHA")
	}
	subpath, err := normalizeSubpath(pluginSubpath)
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	url := "https://github.com/" + repository + ".git"
	if acquirer.URLForRepo != nil {
		url = acquirer.URLForRepo(repository)
	}
	return acquirer.acquireGit(ctx, url, repository, revision, subpath)
}

// AcquireGitHubVerified fails closed when signed/reviewed package identity
// disagrees with the bytes at the immutable source.
func (acquirer Acquirer) AcquireGitHubVerified(ctx context.Context, repository, revision, pluginSubpath, expectedTreeDigest string) (domain.PackageSnapshot, error) {
	snapshot, err := acquirer.AcquireGitHub(ctx, repository, revision, pluginSubpath)
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	if strings.TrimSpace(expectedTreeDigest) == "" || snapshot.TreeDigest != expectedTreeDigest {
		_ = packagedigest.Remove(snapshot)
		return domain.PackageSnapshot{}, fmt.Errorf("acquired package tree digest %s does not match expected %s", snapshot.TreeDigest, expectedTreeDigest)
	}
	return snapshot, nil
}

func (acquirer Acquirer) AcquireLocal(ctx context.Context, sourceRoot string) (domain.PackageSnapshot, error) {
	absolute, err := filepath.Abs(sourceRoot)
	if err != nil {
		return domain.PackageSnapshot{}, fmt.Errorf("resolve local package source failed")
	}
	source := domain.SourceIdentity{RequestedSource: sourceRoot, CanonicalSource: filepath.Clean(absolute), SourceBindingHint: "direct-local"}
	snapshot, err := acquirer.digester().Snapshot(ctx, absolute, source)
	if err != nil {
		return domain.PackageSnapshot{}, fmt.Errorf("acquire local package: snapshot package content failed")
	}
	snapshot.AcquiredAt = acquirer.now()
	return snapshot, nil
}

func (acquirer Acquirer) acquireGit(ctx context.Context, url, repository, revision, subpath string) (domain.PackageSnapshot, error) {
	tempRoot, err := os.MkdirTemp(acquirer.TempRoot, "agentplugins-git-*")
	if err != nil {
		return domain.PackageSnapshot{}, gitAcquisitionFailure(repository, revision, "create temporary workspace")
	}
	defer os.RemoveAll(tempRoot)
	repoRoot := filepath.Join(tempRoot, "repository")
	run := acquirer.gitCommand(ctx, tempRoot)
	if _, err := run("init", "--quiet", repoRoot); err != nil {
		return domain.PackageSnapshot{}, gitAcquisitionFailure(repository, revision, "initialize repository")
	}
	if _, err := run("-C", repoRoot, "remote", "add", "origin", url); err != nil {
		return domain.PackageSnapshot{}, gitAcquisitionFailure(repository, revision, "configure repository remote")
	}
	for _, setting := range [][]string{{"core.autocrlf", "false"}, {"core.filemode", "true"}, {"core.symlinks", "true"}, {"fetch.recurseSubmodules", "false"}} {
		if _, err := run("-C", repoRoot, "config", setting[0], setting[1]); err != nil {
			return domain.PackageSnapshot{}, gitAcquisitionFailure(repository, revision, "configure isolated repository")
		}
	}
	if _, err := run(gitFetchArgs(repoRoot, revision, acquirer.Reporter != nil)...); err != nil {
		return domain.PackageSnapshot{}, gitAcquisitionFailure(repository, revision, "fetch immutable revision")
	}
	resolved, err := run("-C", repoRoot, "rev-parse", "--verify", "FETCH_HEAD^{commit}")
	if err != nil {
		return domain.PackageSnapshot{}, gitAcquisitionFailure(repository, revision, "verify fetched revision")
	}
	if strings.TrimSpace(string(resolved)) != revision {
		return domain.PackageSnapshot{}, fmt.Errorf("acquire GitHub repository %q at revision %s: fetched revision did not match the requested immutable revision", repository, revision)
	}
	pathspec := "."
	if subpath != "" {
		pathspec = subpath
	}
	tree, err := run("-C", repoRoot, "ls-tree", "-r", "-z", revision, "--", pathspec)
	if err != nil {
		return domain.PackageSnapshot{}, gitAcquisitionFailure(repository, revision, "inspect package tree")
	}
	if len(tree) == 0 {
		return domain.PackageSnapshot{}, fmt.Errorf("acquire GitHub repository %q at revision %s: inspect package tree: plugin subpath %q does not exist", repository, revision, pathspec)
	}
	executableFiles, err := executablePathsFromTree(tree, subpath)
	if err != nil {
		return domain.PackageSnapshot{}, fmt.Errorf("acquire GitHub repository %q at revision %s: inspect package tree: %s", repository, revision, publicTreeError(err))
	}
	if subpath != "" {
		if _, err := run("-C", repoRoot, "sparse-checkout", "init", "--cone"); err != nil {
			return domain.PackageSnapshot{}, gitAcquisitionFailure(repository, revision, "initialize sparse checkout")
		}
		if _, err := run("-C", repoRoot, "sparse-checkout", "set", "--cone", "--", subpath); err != nil {
			return domain.PackageSnapshot{}, gitAcquisitionFailure(repository, revision, "configure sparse checkout")
		}
	}
	if _, err := run(gitCheckoutArgs(repoRoot, revision, acquirer.Reporter != nil)...); err != nil {
		return domain.PackageSnapshot{}, gitAcquisitionFailure(repository, revision, "checkout immutable revision")
	}
	packageRoot := repoRoot
	if subpath != "" {
		packageRoot = filepath.Join(repoRoot, filepath.FromSlash(subpath))
	}
	info, err := os.Lstat(packageRoot)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return domain.PackageSnapshot{}, fmt.Errorf("acquire GitHub repository %q at revision %s: plugin subpath %q is not a real directory", repository, revision, pathspec)
	}
	requestedSource := repository + "@" + revision
	canonicalSource := "https://github.com/" + requestedSource
	if subpath != "" {
		requestedSource += "//" + subpath
		canonicalSource += "//" + subpath
	}
	source := domain.SourceIdentity{
		RequestedSource: requestedSource,
		CanonicalSource: canonicalSource,
		Repository:      repository, PackageSubpath: subpath, ResolvedRevision: revision,
		SourceBindingHint: "direct-github-full-sha",
	}
	snapshot, err := acquirer.digester().SnapshotWithExecutables(ctx, packageRoot, source, executableFiles)
	if err != nil {
		return domain.PackageSnapshot{}, gitAcquisitionFailure(repository, revision, "snapshot package content")
	}
	snapshot.AcquiredAt = acquirer.now()
	return snapshot, nil
}

func gitAcquisitionFailure(repository, revision, operation string) error {
	return fmt.Errorf("acquire GitHub repository %q at revision %s: %s failed", repository, revision, operation)
}

func publicTreeError(err error) string {
	message := err.Error()
	switch {
	case strings.Contains(message, "submodule"):
		return "Git submodule content is unsupported in plugin subpath"
	case strings.Contains(message, "outside plugin subpath"):
		return "Git tree contained content outside the plugin subpath"
	default:
		return "Git returned an invalid package tree"
	}
}

func executablePathsFromTree(tree []byte, subpath string) ([]string, error) {
	var executable []string
	prefix := ""
	if subpath != "" {
		prefix = subpath + "/"
	}
	for _, record := range bytes.Split(tree, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		metadata, filename, ok := bytes.Cut(record, []byte{'\t'})
		fields := bytes.Fields(metadata)
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("Git tree returned a malformed entry")
		}
		mode, objectType := string(fields[0]), string(fields[1])
		if mode == "160000" || objectType == "commit" {
			// A repository-root Agent Plugin may live beside unrelated Git
			// submodules. The isolated checkout never fetches their content, so
			// they cannot contribute files or executable bits to the snapshot.
			// Keep rejecting a gitlink inside an explicitly selected package
			// subpath, where omitting it could silently produce a partial package.
			if subpath == "" {
				continue
			}
			return nil, fmt.Errorf("Git submodule content is unsupported in plugin subpath")
		}
		if mode != "100755" {
			continue
		}
		relative := string(filename)
		if prefix != "" {
			if !strings.HasPrefix(relative, prefix) {
				return nil, fmt.Errorf("Git tree entry %q is outside plugin subpath %q", relative, subpath)
			}
			relative = strings.TrimPrefix(relative, prefix)
		}
		if relative == "" {
			return nil, fmt.Errorf("Git tree returned an empty executable path")
		}
		executable = append(executable, relative)
	}
	return executable, nil
}

func packagePathsFromTree(tree []byte) ([]string, error) {
	manifestRoots := map[string]struct{}{}
	componentRoots := map[string]struct{}{}
	rootPortable, rootNative := false, false
	for _, record := range bytes.Split(tree, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		metadata, filename, ok := bytes.Cut(record, []byte{'\t'})
		fields := bytes.Fields(metadata)
		if !ok || len(fields) != 3 || len(filename) == 0 {
			return nil, fmt.Errorf("Git tree returned a malformed entry")
		}
		mode, objectType := string(fields[0]), string(fields[1])
		if objectType != "blob" || (mode != "100644" && mode != "100755") {
			continue
		}
		name := string(filename)
		if name == "plugin.json" {
			rootPortable = true
		}
		if name == ".codex-plugin/plugin.json" {
			rootNative = true
		}
		if path.Base(name) == "plugin.json" {
			manifestRoots[path.Dir(name)] = struct{}{}
		}
		if path.Base(name) == "mcp.json" {
			componentRoots[path.Dir(name)] = struct{}{}
		}
		for offset := 0; offset < len(name); {
			index := strings.Index(name[offset:], "/skills/")
			if index < 0 {
				break
			}
			root := name[:offset+index]
			if root != "" {
				componentRoots[root] = struct{}{}
			}
			offset += index + len("/skills/")
		}
	}
	if rootPortable || rootNative {
		return []string{""}, nil
	}
	candidates := make([]string, 0)
	for root := range manifestRoots {
		if root == "." {
			continue
		}
		if _, err := normalizeSubpath(root); err != nil {
			continue
		}
		if _, packageShaped := componentRoots[root]; packageShaped {
			candidates = append(candidates, root)
		}
	}
	sort.Strings(candidates)
	return candidates, nil
}

func normalizeSubpath(value string) (string, error) {
	value = strings.TrimSpace(strings.Trim(value, "/"))
	if value == "" {
		return "", nil
	}
	if strings.Contains(value, `\`) || path.IsAbs(value) || path.Clean(value) != value {
		return "", fmt.Errorf("plugin subpath must be a normalized repository-relative directory")
	}
	for _, segment := range strings.Split(value, "/") {
		if err := pathpolicy.ValidatePortablePathSegment(segment); err != nil {
			return "", fmt.Errorf("unsafe plugin subpath %q: %w", value, err)
		}
	}
	return value, nil
}

func (acquirer Acquirer) runner() Runner {
	if acquirer.Runner != nil {
		return acquirer.Runner
	}
	return OSRunner{}
}

func (acquirer Acquirer) digester() packagedigest.Builder {
	digester := acquirer.Digester
	if digester.TempRoot == "" {
		digester.TempRoot = acquirer.TempRoot
	}
	return digester
}

func (acquirer Acquirer) now() time.Time {
	if acquirer.Now != nil {
		return acquirer.Now().UTC()
	}
	return time.Now().UTC()
}
