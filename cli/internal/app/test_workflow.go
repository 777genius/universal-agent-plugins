package app

import "context"

func runPluginTests(ctx context.Context, opts PluginTestOptions) (PluginTestResult, error) {
	run, err := preparePluginTestRun(opts)
	if err != nil {
		return PluginTestResult{}, err
	}
	return executePluginTestRun(ctx, run, opts), nil
}
