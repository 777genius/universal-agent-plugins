package app

import (
	"context"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

func ensurePublishRelease(ctx context.Context, client bundleGitHubPublisher, owner, repo, tag string, wantDraft bool) (*domain.Release, string, error) {
	release, err := client.FindReleaseByTag(ctx, owner, repo, tag)
	if err == nil {
		return resolveExistingPublishRelease(ctx, client, owner, repo, tag, wantDraft, release)
	}
	if shouldCreatePublishRelease(err) {
		return createPublishRelease(ctx, client, owner, repo, tag, wantDraft)
	}
	return nil, "", err
}

func maybeDeleteReleaseAsset(ctx context.Context, client bundleGitHubPublisher, owner, repo string, release *domain.Release, name string, force bool) error {
	asset, err := findExistingReleaseAsset(release, name)
	if err != nil {
		return err
	}
	if asset == nil {
		return nil
	}
	if err := requirePublishAssetReplacement(asset, name, force); err != nil {
		return err
	}
	return client.DeleteReleaseAsset(ctx, owner, repo, asset.ID)
}
