package app

import (
	"context"
	"fmt"
)

func resolveURLBundleChecksum(ctx context.Context, downloader bundleHTTPDownloader, rawURL, flagValue string) ([]byte, string, error) {
	if flagValue != "" {
		return resolveFlagBundleChecksum(flagValue)
	}
	sidecarURL, err := bundleSidecarURL(rawURL)
	if err != nil {
		return nil, "", err
	}
	body, _, err := downloader.Download(ctx, sidecarURL)
	if err != nil {
		return nil, "", fmt.Errorf("bundle fetch requires --sha256 or %s: %w", sidecarURL, err)
	}
	sum, err := parseBundleChecksum(body, "")
	if err != nil {
		return nil, "", fmt.Errorf("invalid checksum sidecar %s: %w", sidecarURL, err)
	}
	return sum, sidecarURL, nil
}

func resolveFlagBundleChecksum(flagValue string) ([]byte, string, error) {
	sum, err := parseBundleChecksum([]byte(flagValue), "")
	if err != nil {
		return nil, "", fmt.Errorf("invalid --sha256: %w", err)
	}
	return sum, "flag --sha256", nil
}
