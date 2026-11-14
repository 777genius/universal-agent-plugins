package app

import (
	"context"
	"fmt"
	"strings"
)

func bundleFetch(ctx context.Context, opts PluginBundleFetchOptions, deps bundleFetchDeps) (PluginBundleFetchResult, error) {
	if err := validateBundleFetchWorkflowInput(opts); err != nil {
		return PluginBundleFetchResult{}, err
	}
	return executeBundleFetchWorkflow(ctx, opts, deps)
}

func requireBundleFetchDest(opts PluginBundleFetchOptions) error {
	if strings.TrimSpace(opts.Dest) == "" {
		return fmt.Errorf("bundle fetch requires --dest")
	}
	return nil
}
