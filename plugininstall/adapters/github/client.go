package github

import (
	"net/http"

	"github.com/777genius/plugin-kit-ai/plugininstall/internal/httpconfig"
)

const defaultReleaseJSONMaxBytes = 32 << 20

// Client implements ports.ReleaseSource for api.github.com (or Enterprise).
type Client struct {
	BaseURL    string // e.g. https://api.github.com
	Token      string
	APIClient  *http.Client
	DLClient   *http.Client
	MaxBytes   int64
	APIVersion string
	// ReleaseJSONMaxBytes caps the GitHub release JSON payload; 0 = default (32 MiB). Set in tests.
	ReleaseJSONMaxBytes int64
}

// NewClient returns a Client with sensible defaults.
func NewClient(token string) *Client {
	return &Client{
		BaseURL:    "https://api.github.com",
		Token:      token,
		APIClient:  httpconfig.APIClient(),
		DLClient:   httpconfig.DownloadClient(),
		MaxBytes:   httpconfig.DefaultMaxDownloadBytes,
		APIVersion: httpconfig.DefaultGitHubAPIVersion,
	}
}
