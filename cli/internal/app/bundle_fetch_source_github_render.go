package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

func buildBundleGitHubRemoteSource(ctx context.Context, source bundleGitHubSource, rel *domain.Release, asset domain.Asset, opts PluginBundleFetchOptions, owner, repo string) (bundleRemoteSource, error) {
	body, _, err := source.DownloadAsset(ctx, asset.BrowserDownloadURL)
	if err != nil {
		return bundleRemoteSource{}, err
	}
	sum, checksumSource, err := resolveGitHubBundleChecksum(ctx, source, rel, asset)
	if err != nil {
		return bundleRemoteSource{}, err
	}
	if err := verifyBundleChecksum(body, sum); err != nil {
		return bundleRemoteSource{}, fmt.Errorf("bundle fetch checksum verification failed: %w", err)
	}
	return bundleRemoteSource{
		ArchiveBytes:   body,
		BundleSource:   buildBundleGitHubSourceLabel(owner, repo, rel, opts, asset.Name),
		ChecksumSource: checksumSource,
	}, nil
}

func buildBundleGitHubSourceLabel(owner, repo string, rel *domain.Release, opts PluginBundleFetchOptions, assetName string) string {
	releaseRef := strings.TrimSpace(rel.TagName)
	if releaseRef == "" {
		releaseRef = strings.TrimSpace(opts.Tag)
	}
	refLabel := owner + "/" + repo
	if releaseRef != "" {
		refLabel += "@" + releaseRef
	}
	if opts.Latest {
		refLabel += " (latest)"
	} else {
		refLabel += " (tag)"
	}
	return fmt.Sprintf("github release %s asset=%s", refLabel, assetName)
}
