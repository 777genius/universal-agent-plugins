package app

import (
	"fmt"
	"strings"
)

func validateBundleFetchURLMode(opts PluginBundleFetchOptions) error {
	if strings.TrimSpace(opts.Ref) != "" {
		return fmt.Errorf("bundle fetch accepts either --url or owner/repo, not both")
	}
	if strings.TrimSpace(opts.Tag) != "" || opts.Latest {
		return fmt.Errorf("bundle fetch URL mode does not accept --tag or --latest")
	}
	if strings.TrimSpace(opts.AssetName) != "" || strings.TrimSpace(opts.Platform) != "" || strings.TrimSpace(opts.Runtime) != "" {
		return fmt.Errorf("bundle fetch URL mode does not use --asset-name, --platform, or --runtime")
	}
	return nil
}
