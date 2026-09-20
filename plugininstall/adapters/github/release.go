package github

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

type releaseDTO struct {
	ID         int64      `json:"id"`
	TagName    string     `json:"tag_name"`
	Draft      bool       `json:"draft"`
	Prerelease bool       `json:"prerelease"`
	UploadURL  string     `json:"upload_url"`
	Assets     []assetDTO `json:"assets"`
}

type assetDTO struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// GetReleaseByTag implements ports.ReleaseSource.
func (c *Client) GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*domain.Release, error) {
	path := fmt.Sprintf("repos/%s/%s/releases/tags/%s", owner, repo, tag)
	return c.fetchRelease(ctx, path, fmt.Sprintf("release tag %q not found", tag), false)
}

// GetLatestRelease implements ports.ReleaseSource (GitHub non-prerelease latest).
func (c *Client) GetLatestRelease(ctx context.Context, owner, repo string) (*domain.Release, error) {
	path := fmt.Sprintf("repos/%s/%s/releases/latest", owner, repo)
	return c.fetchRelease(ctx, path, "no latest release found (GitHub has no published non-prerelease release)", false)
}

func (c *Client) releaseJSONMaxBytes() int64 {
	if c.ReleaseJSONMaxBytes > 0 {
		return c.ReleaseJSONMaxBytes
	}
	return defaultReleaseJSONMaxBytes
}
