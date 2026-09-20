package app

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

func selectBundleReleaseAsset(rel *domain.Release, assetName, platform, runtime string) (*domain.Asset, error) {
	assetName = strings.TrimSpace(assetName)
	if assetName != "" {
		asset := findReleaseAsset(rel.Assets, assetName)
		if asset == nil {
			return nil, fmt.Errorf("bundle fetch release has no asset named %q", assetName)
		}
		return asset, nil
	}

	platform = strings.TrimSpace(platform)
	runtime = strings.TrimSpace(runtime)
	candidates := bundleReleaseCandidates(rel.Assets)
	if platform != "" && runtime != "" {
		suffix := fmt.Sprintf("_%s_%s_bundle.tar.gz", platform, runtime)
		matches := make([]domain.Asset, 0, len(candidates))
		for _, asset := range candidates {
			if strings.HasSuffix(asset.Name, suffix) {
				matches = append(matches, asset)
			}
		}
		if len(matches) == 1 {
			return &matches[0], nil
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("bundle fetch release has no bundle asset matching %s", suffix)
		}
		return nil, fmt.Errorf("bundle fetch release has multiple bundle assets matching %s: %s", suffix, joinAssetNames(matches))
	}

	if len(candidates) == 1 {
		return &candidates[0], nil
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("bundle fetch release has no *_bundle.tar.gz assets")
	}
	return nil, fmt.Errorf("bundle fetch release bundle assets are ambiguous; use --asset-name or --platform with --runtime: %s", joinAssetNames(candidates))
}

func bundleReleaseCandidates(assets []domain.Asset) []domain.Asset {
	out := make([]domain.Asset, 0, len(assets))
	for _, asset := range assets {
		name := strings.ToLower(asset.Name)
		if strings.HasSuffix(name, "_bundle.tar.gz") {
			out = append(out, asset)
		}
	}
	return out
}

func joinAssetNames(assets []domain.Asset) string {
	names := make([]string, 0, len(assets))
	for _, asset := range assets {
		names = append(names, asset.Name)
	}
	return strings.Join(names, ", ")
}

func findReleaseAsset(assets []domain.Asset, name string) *domain.Asset {
	name = strings.TrimSpace(name)
	for i := range assets {
		if assets[i].Name == name {
			return &assets[i]
		}
	}
	return nil
}
