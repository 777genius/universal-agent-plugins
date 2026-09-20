package app

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

func resolveGitHubBundleChecksum(ctx context.Context, source bundleGitHubSource, rel *domain.Release, asset domain.Asset) ([]byte, string, error) {
	if sum, src, ok, err := resolveGitHubChecksumsTXT(ctx, source, rel, asset); ok || err != nil {
		return sum, src, err
	}
	return resolveGitHubSidecarChecksum(ctx, source, rel, asset)
}

func resolveGitHubChecksumsTXT(ctx context.Context, source bundleGitHubSource, rel *domain.Release, asset domain.Asset) ([]byte, string, bool, error) {
	checksums := findReleaseAsset(rel.Assets, "checksums.txt")
	if checksums == nil {
		return nil, "", false, nil
	}
	body, _, err := source.DownloadAsset(ctx, checksums.BrowserDownloadURL)
	if err != nil {
		return nil, "", true, err
	}
	sum, err := parseBundleChecksum(body, asset.Name)
	if err != nil {
		return nil, "", false, nil
	}
	return sum, "release asset checksums.txt", true, nil
}

func resolveGitHubSidecarChecksum(ctx context.Context, source bundleGitHubSource, rel *domain.Release, asset domain.Asset) ([]byte, string, error) {
	sidecarName := asset.Name + ".sha256"
	sidecar := findReleaseAsset(rel.Assets, sidecarName)
	if sidecar == nil {
		return nil, "", fmt.Errorf("bundle fetch requires checksums.txt or %s on the selected release", sidecarName)
	}
	body, _, err := source.DownloadAsset(ctx, sidecar.BrowserDownloadURL)
	if err != nil {
		return nil, "", err
	}
	sum, err := parseBundleChecksum(body, asset.Name)
	if err != nil {
		return nil, "", fmt.Errorf("invalid checksum asset %s: %w", sidecarName, err)
	}
	return sum, "release asset " + sidecarName, nil
}
