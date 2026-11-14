package app

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

func findExistingReleaseAsset(release *domain.Release, name string) (*domain.Asset, error) {
	if release == nil {
		return nil, fmt.Errorf("bundle publish release is required")
	}
	return findReleaseAsset(release.Assets, name), nil
}

func requirePublishAssetReplacement(asset *domain.Asset, name string, force bool) error {
	if asset == nil {
		return nil
	}
	if !force {
		return fmt.Errorf("bundle publish release already has asset %q; use --force to replace", name)
	}
	if asset.ID == 0 {
		return fmt.Errorf("bundle publish cannot replace existing asset %q because GitHub asset id is missing", name)
	}
	return nil
}
