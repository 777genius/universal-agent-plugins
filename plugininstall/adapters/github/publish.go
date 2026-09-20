package github

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

// FindReleaseByTag returns a release for publishing and allows draft releases.
func (c *Client) FindReleaseByTag(ctx context.Context, owner, repo, tag string) (*domain.Release, error) {
	path := fmt.Sprintf("repos/%s/%s/releases/tags/%s", owner, repo, tag)
	return c.fetchRelease(ctx, path, fmt.Sprintf("release tag %q not found", tag), true)
}

// CreateRelease creates a new release for the provided tag.
func (c *Client) CreateRelease(ctx context.Context, owner, repo, tag string, draft bool) (*domain.Release, error) {
	return c.createRelease(ctx, owner, repo, tag, draft)
}

// UpdateReleaseDraftState updates the draft visibility of an existing release.
func (c *Client) UpdateReleaseDraftState(ctx context.Context, owner, repo string, releaseID int64, draft bool) (*domain.Release, error) {
	return c.updateReleaseDraftState(ctx, owner, repo, releaseID, draft)
}

// UploadReleaseAsset uploads an asset to the release upload URL.
func (c *Client) UploadReleaseAsset(ctx context.Context, uploadURL, name string, body []byte, contentType string) (*domain.Asset, error) {
	return c.uploadReleaseAsset(ctx, uploadURL, name, body, contentType)
}

// DeleteReleaseAsset removes an existing asset by id.
func (c *Client) DeleteReleaseAsset(ctx context.Context, owner, repo string, assetID int64) error {
	return c.deleteReleaseAsset(ctx, owner, repo, assetID)
}
