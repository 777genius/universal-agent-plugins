package app

import (
	"fmt"
	"strings"

	pluginkitai "github.com/777genius/plugin-kit-ai/sdk"
)

func collectStableRuntimeSupport(target string) []runtimeTestSupport {
	out := make([]runtimeTestSupport, 0, 4)
	for _, entry := range pluginkitai.Supported() {
		if mapSupportPlatformToTarget(string(entry.Platform)) != target {
			continue
		}
		if string(entry.Status) != "runtime_supported" || string(entry.Maturity) != "stable" {
			continue
		}
		out = append(out, runtimeTestSupport{
			Platform: target,
			Event:    string(entry.Event),
			Carrier:  string(entry.Carrier),
		})
	}
	return out
}

func selectAllRuntimeTestCases(supported []runtimeTestSupport, requestedEvent string) ([]runtimeTestSupport, error) {
	if strings.TrimSpace(requestedEvent) != "" {
		return nil, fmt.Errorf("--event cannot be used with --all")
	}
	return append([]runtimeTestSupport(nil), supported...), nil
}

func selectNamedRuntimeTestCases(supported []runtimeTestSupport, requestedEvent string) ([]runtimeTestSupport, error) {
	requestedEvent = strings.TrimSpace(requestedEvent)
	if requestedEvent == "" {
		if len(supported) == 1 {
			return []runtimeTestSupport{supported[0]}, nil
		}
		return nil, fmt.Errorf("test requires --event or --all; supported stable events: %s", strings.Join(runtimeTestCaseNames(supported), ", "))
	}
	for _, item := range supported {
		if strings.EqualFold(item.Event, requestedEvent) {
			return []runtimeTestSupport{item}, nil
		}
	}
	return nil, fmt.Errorf("unsupported stable event %q for %s; supported: %s", requestedEvent, supported[0].Platform, strings.Join(runtimeTestCaseNames(supported), ", "))
}

func runtimeTestCaseNames(supported []runtimeTestSupport) []string {
	names := make([]string, 0, len(supported))
	for _, item := range supported {
		names = append(names, item.Event)
	}
	return names
}
