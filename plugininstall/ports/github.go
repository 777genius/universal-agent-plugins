package ports

import (
	"context"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

// ReleaseSource fetches GitHub release metadata and bytes.
type ReleaseSource interface {
	GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*domain.Release, error)
	// GetLatestRelease is GET /repos/{owner}/{repo}/releases/latest (non-draft, non-prerelease).
	GetLatestRelease(ctx context.Context, owner, repo string) (*domain.Release, error)
	DownloadAsset(ctx context.Context, url string) (body []byte, contentType string, err error)
}
