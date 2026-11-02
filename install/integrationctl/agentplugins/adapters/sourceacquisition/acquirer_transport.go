// Package sourceacquisition resolves direct package sources into one sealed,
// digest-verified local snapshot. It never executes package content.
package sourceacquisition

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (acquirer Acquirer) AcquireGitHub(ctx context.Context, repository, revision, pluginSubpath string) (domain.PackageSnapshot, error) {
	return acquirer.acquireGitHub(ctx, repository, revision, pluginSubpath, true)
}

// AcquireGitHubSkill snapshots one complete raw Skill subtree. Unlike ordinary
// root packages, a root Skill cannot omit an unrelated-looking Git submodule.
func (acquirer Acquirer) AcquireGitHubSkill(ctx context.Context, repository, revision, skillSubpath string) (domain.PackageSnapshot, error) {
	return acquirer.acquireGitHub(ctx, repository, revision, skillSubpath, false)
}
func (acquirer Acquirer) acquireGitHub(ctx context.Context, repository, revision, pluginSubpath string, allowRootGitlinks bool) (domain.PackageSnapshot, error) {
	if !repositoryPattern.MatchString(repository) {
		return domain.PackageSnapshot{}, fmt.Errorf("github repository must be owner/repo")
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
	return acquirer.acquireGit(ctx, url, repository, revision, subpath, allowRootGitlinks)
}

// AcquireGitHubVerified fails closed when signed/reviewed package identity
// disagrees with the bytes at the immutable source.
func (acquirer Acquirer) AcquireGitHubVerified(ctx context.Context, repository, revision, pluginSubpath, expectedTreeDigest string) (domain.PackageSnapshot, error) {
	snapshot, err := acquirer.AcquireGitHub(ctx, repository, revision, pluginSubpath)
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	return verifyAcquiredTree(snapshot, expectedTreeDigest)
}

// AcquireGitHubSkillVerified replays a raw Skill with its own source digest and
// strict tree completeness before normalization into the standard artifact.
func (acquirer Acquirer) AcquireGitHubSkillVerified(ctx context.Context, repository, revision, skillSubpath, expectedTreeDigest string) (domain.PackageSnapshot, error) {
	snapshot, err := acquirer.AcquireGitHubSkill(ctx, repository, revision, skillSubpath)
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	return verifyAcquiredTree(snapshot, expectedTreeDigest)
}
func verifyAcquiredTree(snapshot domain.PackageSnapshot, expectedTreeDigest string) (domain.PackageSnapshot, error) {
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

func (acquirer Acquirer) acquireGit(ctx context.Context, url, repository, revision, subpath string, allowRootGitlinks bool) (domain.PackageSnapshot, error) {
	return acquirer.acquireGitTransport(ctx, url, repository, revision, subpath, false, allowRootGitlinks)
}

func (acquirer Acquirer) acquireGitTransport(ctx context.Context, url, repository, revision, subpath string, native, allowRootGitlinks bool) (domain.PackageSnapshot, error) {
	tempRoot, err := os.MkdirTemp(acquirer.TempRoot, "agentplugins-git-*")
	if err != nil {
		return domain.PackageSnapshot{}, gitAcquisitionFailure(repository, revision, "create temporary workspace")
	}
	defer func() { _ = os.RemoveAll(tempRoot) }()
	repoRoot := filepath.Join(tempRoot, "repository")
	run := func(args ...string) ([]byte, error) {
		transport := native && len(args) > 2 && args[2] == "fetch"
		return acquirer.runner().Run(ctx, Command{Dir: tempRoot, Args: args, NativeCredentials: transport, mappedLocal: transport && acquirer.mappedLocalEndpoint(url)})
	}
	if _, err := run("init", "--quiet", repoRoot); err != nil {
		return domain.PackageSnapshot{}, gitAcquisitionFailure(repository, revision, "initialize repository")
	}
	if _, err := run("-C", repoRoot, "remote", "add", "origin", url); err != nil {
		return domain.PackageSnapshot{}, gitAcquisitionFailure(repository, revision, "configure repository remote")
	}
	// Pin the default worktree line ending independently of the runner OS.
	// core.autocrlf=false alone still permits text=auto attributes to use
	// core.eol=native (CRLF on Windows, LF elsewhere). Explicit per-path
	// .gitattributes eol=crlf remains authoritative.
	for _, setting := range [][]string{{"core.autocrlf", "false"}, {"core.eol", "lf"}, {"core.filemode", "true"}, {"core.symlinks", "true"}, {"fetch.recurseSubmodules", "false"}} {
		if _, err := run("-C", repoRoot, "config", setting[0], setting[1]); err != nil {
			return domain.PackageSnapshot{}, gitAcquisitionFailure(repository, revision, "configure isolated repository")
		}
	}
	fetchArgs := []string{"-C", repoRoot, "fetch", "--quiet", "--depth=1", "--no-tags", "--no-recurse-submodules", "origin", revision}
	if !native {
		fetchArgs = append(fetchArgs[:5], append([]string{"--filter=blob:none"}, fetchArgs[5:]...)...)
	}
	if _, err := run(fetchArgs...); err != nil {
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
		public := fmt.Errorf("acquire GitHub repository %q at revision %s: inspect package tree: plugin subpath %q does not exist", repository, revision, pathspec)
		return domain.PackageSnapshot{}, gitPackageFailure(public, "selected package path does not exist")
	}
	if reason := selectedGitLeaf(tree, subpath); reason != "" {
		return domain.PackageSnapshot{}, gitNonDirectoryFailure(repository, revision, pathspec, reason)
	}
	executableFiles, err := executablePathsFromTreePolicy(tree, subpath, allowRootGitlinks)
	if err != nil {
		reason := publicTreeError(err)
		public := fmt.Errorf("acquire GitHub repository %q at revision %s: inspect package tree: %s", repository, revision, reason)
		return domain.PackageSnapshot{}, gitPackageFailure(public, reason)
	}
	if subpath != "" {
		if _, err := run("-C", repoRoot, "sparse-checkout", "init", "--cone"); err != nil {
			return domain.PackageSnapshot{}, gitAcquisitionFailure(repository, revision, "initialize sparse checkout")
		}
		if _, err := run("-C", repoRoot, "sparse-checkout", "set", "--cone", "--", subpath); err != nil {
			return domain.PackageSnapshot{}, gitAcquisitionFailure(repository, revision, "configure sparse checkout")
		}
	}
	if _, err := run("-C", repoRoot, "checkout", "--quiet", "--detach", revision); err != nil {
		return domain.PackageSnapshot{}, gitAcquisitionFailure(repository, revision, "checkout immutable revision")
	}
	packageRoot := repoRoot
	if subpath != "" {
		packageRoot = filepath.Join(repoRoot, filepath.FromSlash(subpath))
	}
	info, err := os.Lstat(packageRoot)
	if err != nil {
		public := fmt.Errorf("acquire GitHub repository %q at revision %s: plugin subpath %q is not a real directory", repository, revision, pathspec)
		if os.IsNotExist(err) {
			return domain.PackageSnapshot{}, gitPackageFailure(public, "selected package path does not exist")
		}
		return domain.PackageSnapshot{}, public
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return domain.PackageSnapshot{}, gitNonDirectoryFailure(repository, revision, pathspec, "selected package path is not a real directory")
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
