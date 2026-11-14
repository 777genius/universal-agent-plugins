package app

import (
	"context"
	"strings"

	gh "github.com/777genius/plugin-kit-ai/plugininstall/adapters/github"
	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

type PluginBundlePublishOptions struct {
	Root          string
	Platform      string
	Repo          string
	Tag           string
	Draft         bool
	GitHubToken   string
	GitHubAPIBase string
	Force         bool
}

type PluginBundlePublishResult struct {
	Lines []string
}

type bundleGitHubPublisher interface {
	FindReleaseByTag(ctx context.Context, owner, repo, tag string) (*domain.Release, error)
	CreateRelease(ctx context.Context, owner, repo, tag string, draft bool) (*domain.Release, error)
	UpdateReleaseDraftState(ctx context.Context, owner, repo string, releaseID int64, draft bool) (*domain.Release, error)
	UploadReleaseAsset(ctx context.Context, uploadURL, name string, body []byte, contentType string) (*domain.Asset, error)
	DeleteReleaseAsset(ctx context.Context, owner, repo string, assetID int64) error
}

type bundlePublishDeps struct {
	GitHub bundleGitHubPublisher
	Export func(PluginExportOptions) (PluginExportResult, error)
}

func (PluginService) BundlePublish(ctx context.Context, opts PluginBundlePublishOptions) (PluginBundlePublishResult, error) {
	deps := defaultBundlePublishDeps(opts)
	return bundlePublish(ctx, opts, deps)
}

func defaultBundlePublishDeps(opts PluginBundlePublishOptions) bundlePublishDeps {
	client := gh.NewClient(strings.TrimSpace(opts.GitHubToken))
	if base := strings.TrimSpace(opts.GitHubAPIBase); base != "" {
		client.BaseURL = base
	}
	return bundlePublishDeps{
		GitHub: client,
		Export: func(exportOpts PluginExportOptions) (PluginExportResult, error) {
			return PluginService{}.Export(exportOpts)
		},
	}
}

func bundlePublish(ctx context.Context, opts PluginBundlePublishOptions, deps bundlePublishDeps) (PluginBundlePublishResult, error) {
	input, err := resolveBundlePublishInput(opts, deps)
	if err != nil {
		return PluginBundlePublishResult{}, err
	}
	artifact, release, state, err := executeBundlePublish(ctx, input, deps)
	if err != nil {
		return PluginBundlePublishResult{}, err
	}
	return buildBundlePublishResult(input, artifact, release, state), nil
}
