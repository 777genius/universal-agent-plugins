package app

import "context"

func validateBundleFetchWorkflowInput(opts PluginBundleFetchOptions) error {
	if err := requireBundleFetchDest(opts); err != nil {
		return err
	}
	return validateBundleFetchMode(opts)
}

func executeBundleFetchWorkflow(ctx context.Context, opts PluginBundleFetchOptions, deps bundleFetchDeps) (PluginBundleFetchResult, error) {
	source, err := resolveBundleRemoteSource(ctx, opts, deps)
	if err != nil {
		return PluginBundleFetchResult{}, err
	}
	metadata, installedPath, err := installFetchedBundleSource(source, opts)
	if err != nil {
		return PluginBundleFetchResult{}, err
	}
	return buildBundleFetchResult(metadata, source, installedPath), nil
}
