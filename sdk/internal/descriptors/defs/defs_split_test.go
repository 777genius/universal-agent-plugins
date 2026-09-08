package defs

import "testing"

func TestEventsPreserveStableBucketOrder(t *testing.T) {
	t.Parallel()

	events := Events()
	want := []struct {
		platform string
		event    string
	}{
		{platform: "claude", event: "Stop"},
		{platform: "claude", event: "PreToolUse"},
		{platform: "claude", event: "UserPromptSubmit"},
		{platform: "claude", event: "SessionStart"},
		{platform: "claude", event: "SessionEnd"},
		{platform: "claude", event: "Notification"},
		{platform: "claude", event: "PostToolUse"},
		{platform: "claude", event: "PostToolUseFailure"},
		{platform: "claude", event: "PermissionRequest"},
		{platform: "claude", event: "SubagentStart"},
		{platform: "claude", event: "SubagentStop"},
		{platform: "claude", event: "PreCompact"},
		{platform: "claude", event: "Setup"},
		{platform: "claude", event: "TeammateIdle"},
		{platform: "claude", event: "TaskCompleted"},
		{platform: "claude", event: "ConfigChange"},
		{platform: "claude", event: "WorktreeCreate"},
		{platform: "claude", event: "WorktreeRemove"},
		{platform: "gemini", event: "SessionStart"},
		{platform: "gemini", event: "SessionEnd"},
		{platform: "gemini", event: "BeforeModel"},
		{platform: "gemini", event: "AfterModel"},
		{platform: "gemini", event: "BeforeToolSelection"},
		{platform: "gemini", event: "BeforeAgent"},
		{platform: "gemini", event: "AfterAgent"},
		{platform: "gemini", event: "BeforeTool"},
		{platform: "gemini", event: "AfterTool"},
		{platform: "codex", event: "Notify"},
		{platform: "codex", event: "Stop"},
		{platform: "codex", event: "SubagentStop"},
		{platform: "codex", event: "PreToolUse"},
		{platform: "codex", event: "PermissionRequest"},
	}
	if len(events) != len(want) {
		t.Fatalf("events count = %d want %d", len(events), len(want))
	}
	for i, expected := range want {
		got := events[i]
		if string(got.Platform) != expected.platform || string(got.Event) != expected.event {
			t.Fatalf("events[%d] = %s/%s want %s/%s", i, got.Platform, got.Event, expected.platform, expected.event)
		}
	}
}
