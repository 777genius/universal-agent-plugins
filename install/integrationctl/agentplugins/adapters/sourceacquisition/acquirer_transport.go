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

// AcquisitionTempRoot reports the directory acquisitions stage their files in;
// an empty value means the system temporary directory.
func (acquirer Acquirer) AcquisitionTempRoot() string { return acquirer.TempRoot }

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
	// Name a mistyped source in the user's own words; later snapshot failures
	// stay redacted because they can involve private staging paths.
	if info, err := os.Lstat(absolute); os.IsNotExist(err) {
		return domain.PackageSnapshot{}, fmt.Errorf("local package source %q was not found", sourceRoot)
	} else if err == nil && !info.IsDir() {
		return domain.PackageSnapshot{}, fmt.Errorf("local package source %q must be a directory, not a file or symbolic link", sourceRoot)
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

func gitAcquisitionFailure(repository, revision, operation string) error {
	return fmt.Errorf("acquire GitHub repository %q at revision %s: %s failed", repository, revision, operation)
}

// gitReadFailure is gitAcquisitionFailure for a remote read; it keeps the same
// message and stays connect-class when cause is.
func gitReadFailure(cause error, repository, revision, operation string) error {
	return gitFailure(cause, gitAcquisitionFailure(repository, revision, operation).Error())
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
