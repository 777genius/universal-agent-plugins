package app

import (
	"context"
	"fmt"
	"strings"
)

func resolveBundleGitHubSource(ctx context.Context, opts PluginBundleFetchOptions, source bundleGitHubSource) (bundleRemoteSource, error) {
	if source == nil {
		return bundleRemoteSource{}, fmt.Errorf("bundle fetch GitHub source is required")
	}
	owner, repo, err := splitOwnerRepo(opts.Ref)
	if err != nil {
		return bundleRemoteSource{}, err
	}
	rel, err := loadBundleGitHubRelease(ctx, source, owner, repo, opts)
	if err != nil {
		return bundleRemoteSource{}, err
	}
	asset, err := selectBundleReleaseAsset(rel, opts.AssetName, opts.Platform, opts.Runtime)
	if err != nil {
		return bundleRemoteSource{}, err
	}
	return buildBundleGitHubRemoteSource(ctx, source, rel, *asset, opts, owner, repo)
}

func splitOwnerRepo(ref string) (string, string, error) {
	parts := strings.SplitN(strings.TrimSpace(ref), "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("bundle fetch GitHub mode requires owner/repo")
	}
	return parts[0], parts[1], nil
}
