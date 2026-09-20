package app

import "strings"

func resolveRuntimeTestPlatform(enabledTargets []string, requested string) (string, error) {
	requested = strings.ToLower(strings.TrimSpace(requested))
	if requested != "" {
		return resolveRequestedRuntimeTestPlatform(enabledTargets, requested)
	}
	return resolveAutoRuntimeTestPlatform(enabledTargets, requested)
}

func isRuntimeTestPlatform(platform string) bool {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "claude", "codex-runtime":
		return true
	default:
		return false
	}
}

func stableRuntimeSupport(target string) []runtimeTestSupport {
	target = strings.ToLower(strings.TrimSpace(target))
	return collectStableRuntimeSupport(target)
}

func mapSupportPlatformToTarget(platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "codex":
		return "codex-runtime"
	default:
		return strings.ToLower(strings.TrimSpace(platform))
	}
}

func selectRuntimeTestCases(supported []runtimeTestSupport, requestedEvent string, all bool) ([]runtimeTestSupport, error) {
	if all {
		return selectAllRuntimeTestCases(supported, requestedEvent)
	}
	return selectNamedRuntimeTestCases(supported, requestedEvent)
}
