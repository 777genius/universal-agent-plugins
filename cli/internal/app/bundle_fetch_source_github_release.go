package app

import (
	"context"
	"strings"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

func loadBundleGitHubRelease(ctx context.Context, source bundleGitHubSource, owner, repo string, opts PluginBundleFetchOptions) (*domain.Release, error) {
	if opts.Latest {
		return source.GetLatestRelease(ctx, owner, repo)
	}
	return source.FindReleaseByTag(ctx, owner, repo, strings.TrimSpace(opts.Tag))
}
