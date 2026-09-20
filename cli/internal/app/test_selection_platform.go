package app

import (
	"fmt"
	"strings"
)

func resolveRequestedRuntimeTestPlatform(enabledTargets []string, requested string) (string, error) {
	if !isRuntimeTestPlatform(requested) {
		return "", runtimeTestUnsupportedPlatformError(enabledTargets, requested)
	}
	for _, target := range enabledTargets {
		if target == requested {
			return requested, nil
		}
	}
	return "", fmt.Errorf("plugin.yaml does not enable target %q", requested)
}

func resolveAutoRuntimeTestPlatform(enabledTargets []string, requested string) (string, error) {
	candidates := collectRuntimeTestPlatformCandidates(enabledTargets)
	switch len(candidates) {
	case 0:
		return "", runtimeTestUnsupportedPlatformError(enabledTargets, requested)
	case 1:
		return candidates[0], nil
	default:
		return "", fmt.Errorf("test requires --platform when multiple launcher-based runtime targets are enabled (%s)", strings.Join(candidates, ", "))
	}
}

func collectRuntimeTestPlatformCandidates(enabledTargets []string) []string {
	var candidates []string
	for _, target := range enabledTargets {
		if isRuntimeTestPlatform(target) {
			candidates = append(candidates, target)
		}
	}
	return candidates
}
