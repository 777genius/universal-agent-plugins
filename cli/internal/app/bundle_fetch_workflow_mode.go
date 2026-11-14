package app

import "strings"

func validateBundleFetchMode(opts PluginBundleFetchOptions) error {
	if strings.TrimSpace(opts.URL) != "" {
		return validateBundleFetchURLMode(opts)
	}
	return validateBundleFetchGitHubMode(opts)
}
