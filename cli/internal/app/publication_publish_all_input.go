package app

import (
	"fmt"
	"strings"
)

func validatePublishAllOptions(opts PluginPublishOptions) (string, error) {
	if !opts.DryRun {
		return "", fmt.Errorf("publish --all currently supports only --dry-run planning")
	}
	root := strings.TrimSpace(opts.Root)
	if root == "" {
		root = "."
	}
	return root, nil
}
