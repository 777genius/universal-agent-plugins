package app

import (
	"fmt"
	"strings"
)

func validateBundleFetchGitHubMode(opts PluginBundleFetchOptions) error {
	if strings.TrimSpace(opts.Ref) == "" {
		return fmt.Errorf("bundle fetch requires --url or owner/repo")
	}
	if opts.Latest && strings.TrimSpace(opts.Tag) != "" {
		return fmt.Errorf("bundle fetch does not use --tag together with --latest")
	}
	if !opts.Latest && strings.TrimSpace(opts.Tag) == "" {
		return fmt.Errorf("bundle fetch GitHub mode requires --tag or --latest")
	}
	if (strings.TrimSpace(opts.Platform) == "") != (strings.TrimSpace(opts.Runtime) == "") {
		return fmt.Errorf("bundle fetch GitHub mode requires --platform and --runtime together")
	}
	return nil
}
