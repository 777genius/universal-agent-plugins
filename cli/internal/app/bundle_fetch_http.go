package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
)

type bundleHTTPClientConfig struct {
	AdditionalRootsFile string
}

func newDefaultBundleFetchHTTPClient() (bundleHTTPClient, *http.Client, error) {
	client, err := newBundleHTTPClient(bundleHTTPClientConfig{
		AdditionalRootsFile: strings.TrimSpace(os.Getenv(bundleFetchTestCAFileEnv)),
	})
	if err != nil {
		return bundleHTTPClient{}, nil, err
	}
	if strings.TrimSpace(os.Getenv(bundleFetchTestCAFileEnv)) == "" {
		return client, nil, nil
	}
	return client, client.Client, nil
}

func newBundleHTTPClient(cfg bundleHTTPClientConfig) (bundleHTTPClient, error) {
	t, err := newBundleHTTPTransport(cfg)
	if err != nil {
		return bundleHTTPClient{}, err
	}
	return bundleHTTPClient{
		Client: &http.Client{
			Timeout:       defaultBundleFetchTimeout,
			Transport:     t,
			CheckRedirect: bundleFetchRedirectPolicy,
		},
		MaxBytes: defaultBundleFetchMaxBytes,
	}, nil
}

func (c bundleHTTPClient) Download(ctx context.Context, url string) ([]byte, string, error) {
	if c.Client == nil {
		defaultClient, _, err := newDefaultBundleFetchHTTPClient()
		if err != nil {
			return nil, "", err
		}
		c = defaultClient
	}
	max := c.MaxBytes
	if max <= 0 {
		max = defaultBundleFetchMaxBytes
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("bundle fetch request: %w", err)
	}
	return downloadBundleHTTPResponse(c.Client, req, max)
}
