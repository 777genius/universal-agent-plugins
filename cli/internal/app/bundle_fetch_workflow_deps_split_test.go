package app

import "testing"

func TestNewBundleFetchGitHubClientUsesAPIBaseWhenProvided(t *testing.T) {
	t.Parallel()

	client := newBundleFetchGitHubClient(PluginBundleFetchOptions{
		GitHubAPIBase: " https://example.test/api/ ",
	})
	if got := client.BaseURL; got != "https://example.test/api/" {
		t.Fatalf("base URL = %q", got)
	}
}

func TestAttachBundleFetchHTTPClientNoopsOnNilClient(t *testing.T) {
	t.Parallel()

	client := newBundleFetchGitHubClient(PluginBundleFetchOptions{})
	apiClient := client.APIClient
	dlClient := client.DLClient
	attachBundleFetchHTTPClient(client, nil)
	if client.APIClient != apiClient || client.DLClient != dlClient {
		t.Fatalf("client = %#v", client)
	}
}
