package app

import (
	"net/http"
	"strings"

	gh "github.com/777genius/plugin-kit-ai/plugininstall/adapters/github"
)

func newBundleFetchGitHubClient(opts PluginBundleFetchOptions) *gh.Client {
	client := gh.NewClient(strings.TrimSpace(opts.GitHubToken))
	if base := strings.TrimSpace(opts.GitHubAPIBase); base != "" {
		client.BaseURL = base
	}
	return client
}

func attachBundleFetchHTTPClient(client *gh.Client, httpClient *http.Client) {
	if httpClient == nil {
		return
	}
	client.APIClient = httpClient
	client.DLClient = httpClient
}
