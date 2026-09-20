package app

import (
	"context"
	"fmt"
	"strings"
)

func resolveBundleRemoteSource(ctx context.Context, opts PluginBundleFetchOptions, deps bundleFetchDeps) (bundleRemoteSource, error) {
	if bundleFetchUsesURLSource(opts) {
		return resolveBundleURLSource(ctx, opts, deps.URLDownloader)
	}
	return resolveBundleGitHubSource(ctx, opts, deps.GitHub)
}

func validateFetchedBundleMatchesRequest(metadata exportMetadata, opts PluginBundleFetchOptions) error {
	if err := validateFetchedBundlePlatform(metadata, opts.Platform); err != nil {
		return err
	}
	return validateFetchedBundleRuntime(metadata, opts.Runtime)
}

func bundleFetchUsesURLSource(opts PluginBundleFetchOptions) bool {
	return strings.TrimSpace(opts.URL) != ""
}

func validateFetchedBundlePlatform(metadata exportMetadata, platform string) error {
	platform = strings.TrimSpace(platform)
	if platform != "" && metadata.Platform != platform {
		return fmt.Errorf("bundle fetch selected asset does not match requested platform %q", platform)
	}
	return nil
}

func validateFetchedBundleRuntime(metadata exportMetadata, runtime string) error {
	runtime = strings.TrimSpace(runtime)
	if runtime != "" && metadata.Runtime != runtime {
		return fmt.Errorf("bundle fetch selected asset does not match requested runtime %q", runtime)
	}
	return nil
}
