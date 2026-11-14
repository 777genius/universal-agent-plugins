package github

import (
	"context"
)

// DownloadAsset downloads the full body from browser_download_url (follows redirects; see httpconfig.DownloadClient).
func (c *Client) DownloadAsset(ctx context.Context, url string) ([]byte, string, error) {
	return c.downloadAsset(ctx, url)
}
