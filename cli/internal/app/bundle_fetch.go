package app

import (
	"context"
	"net/http"
	"time"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

const (
	defaultBundleFetchTimeout      = 15 * time.Minute
	defaultBundleFetchConnect      = 15 * time.Second
	defaultBundleFetchMaxRedirects = 10
	defaultBundleFetchMaxBytes     = 256 << 20 // 256 MiB
	bundleFetchTestCAFileEnv       = "PLUGIN_KIT_AI_TEST_CA_FILE"
)

type PluginBundleFetchOptions struct {
	URL           string
	Ref           string
	Tag           string
	Latest        bool
	Dest          string
	SHA256        string
	AssetName     string
	Platform      string
	Runtime       string
	GitHubToken   string
	GitHubAPIBase string
	Force         bool
}

type PluginBundleFetchResult struct {
	Lines []string
}

type bundleHTTPDownloader interface {
	Download(ctx context.Context, url string) ([]byte, string, error)
}

type bundleGitHubSource interface {
	GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*domain.Release, error)
	GetLatestRelease(ctx context.Context, owner, repo string) (*domain.Release, error)
	FindReleaseByTag(ctx context.Context, owner, repo, tag string) (*domain.Release, error)
	DownloadAsset(ctx context.Context, url string) (body []byte, contentType string, err error)
}

type bundleFetchDeps struct {
	URLDownloader bundleHTTPDownloader
	GitHub        bundleGitHubSource
}

type bundleHTTPClient struct {
	Client   *http.Client
	MaxBytes int64
}

type bundleRemoteSource struct {
	ArchiveBytes   []byte
	BundleSource   string
	ChecksumSource string
}

func (PluginService) BundleFetch(ctx context.Context, opts PluginBundleFetchOptions) (PluginBundleFetchResult, error) {
	deps, err := defaultBundleFetchDeps(opts)
	if err != nil {
		return PluginBundleFetchResult{}, err
	}
	return bundleFetch(ctx, opts, deps)
}
