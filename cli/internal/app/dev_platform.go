package app

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

func resolveDevPlatform(root, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested != "" {
		if !isRuntimeTestPlatform(requested) {
			return "", runtimeDevUnsupportedPlatformError(nil, requested)
		}
		return strings.ToLower(requested), nil
	}
	graph, _, err := pluginmanifest.Discover(root)
	if err != nil {
		return "", err
	}
	enabledTargets := graph.Manifest.EnabledTargets()
	platform, err := resolveRuntimeTestPlatform(enabledTargets, "")
	if err != nil {
		return "", err
	}
	return platform, nil
}
