package app

import (
	"strings"
	"testing"
)

func TestResolveAutoRuntimeTestPlatformRejectsMultipleCandidates(t *testing.T) {
	t.Parallel()

	_, err := resolveAutoRuntimeTestPlatform([]string{"claude", "codex-runtime"}, "")
	if err == nil || !strings.Contains(err.Error(), "requires --platform") {
		t.Fatalf("err = %v", err)
	}
}

func TestSelectAllRuntimeTestCasesRejectsEvent(t *testing.T) {
	t.Parallel()

	_, err := selectAllRuntimeTestCases([]runtimeTestSupport{{Platform: "claude", Event: "Stop"}}, "Stop")
	if err == nil || err.Error() != "--event cannot be used with --all" {
		t.Fatalf("err = %v", err)
	}
}

func TestSelectNamedRuntimeTestCasesUsesSupportedEventNames(t *testing.T) {
	t.Parallel()

	_, err := selectNamedRuntimeTestCases([]runtimeTestSupport{
		{Platform: "claude", Event: "Stop"},
		{Platform: "claude", Event: "PreToolUse"},
	}, "")
	if err == nil || !strings.Contains(err.Error(), "Stop, PreToolUse") {
		t.Fatalf("err = %v", err)
	}
}
