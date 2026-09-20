package app

import (
	"fmt"
	"strings"
)

func runtimeTestInvocation(entrypoint string, event, carrier string, payload []byte, platform string) ([]string, []byte, error) {
	invocation := runtimeTestInvocationName(platform, event)
	switch carrier {
	case "stdin_json":
		return []string{entrypoint, invocation}, append([]byte(nil), payload...), nil
	case "argv_json":
		return []string{entrypoint, invocation, string(payload)}, nil, nil
	default:
		return nil, nil, fmt.Errorf("unsupported carrier %q for %s/%s", carrier, platform, event)
	}
}

func runtimeTestInvocationName(platform, event string) string {
	if platform == "codex-runtime" {
		return strings.ToLower(strings.TrimSpace(event))
	}
	return strings.TrimSpace(event)
}
