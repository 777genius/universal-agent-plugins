package pluginkitai

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/sdk/codex"
	"github.com/777genius/plugin-kit-ai/sdk/internal/descriptors/gen"
)

const codexHooksStopPayload = `{"session_id":"01a05cd8-b495-7f80-a36b-cc0aa98efc05","turn_id":"01a05cd8-b51b-7343-8b75-b2d4ad9e276e","transcript_path":"/tmp/rollout.jsonl","cwd":"/tmp/proj","hook_event_name":"Stop","model":"gpt-5.6-sol","permission_mode":"bypassPermissions","stop_hook_active":false,"last_assistant_message":"OK"}`

func TestApp_CodexStopHook(t *testing.T) {
	iox := &testIO{in: []byte(codexHooksStopPayload)}
	app := New(Config{
		Name: "t",
		Args: []string{"plugin-kit-ai", "CodexStop"},
		IO:   iox,
		Env:  testEnv{},
	})
	calls := 0
	app.Codex().OnStop(func(e *codex.StopEvent) *codex.Response {
		calls++
		if e.SessionID != "01a05cd8-b495-7f80-a36b-cc0aa98efc05" {
			t.Fatalf("session = %q", e.SessionID)
		}
		if e.TurnID == "" || e.LastAssistantMessage != "OK" {
			t.Fatalf("event = %+v", *e)
		}
		return codex.Continue()
	})
	if c := app.Run(); c != 0 {
		t.Fatalf("exit %d stderr=%q", c, iox.err.String())
	}
	if calls != 1 {
		t.Fatalf("handler calls = %d", calls)
	}
	if iox.out.Len() != 0 || iox.err.Len() != 0 {
		t.Fatalf("observation hook wrote output: stdout=%q stderr=%q", iox.out.String(), iox.err.String())
	}
}

func TestApp_CodexSubagentStopHook(t *testing.T) {
	iox := &testIO{in: []byte(`{"session_id":"s","turn_id":"t","hook_event_name":"SubagentStop","agent_id":"a1","agent_type":"worker","agent_transcript_path":"/at","last_assistant_message":"done"}`)}
	app := New(Config{
		Name: "t",
		Args: []string{"plugin-kit-ai", "CodexSubagentStop"},
		IO:   iox,
		Env:  testEnv{},
	})
	app.Codex().OnSubagentStop(func(e *codex.SubagentStopEvent) *codex.Response {
		if e.AgentID != "a1" || e.AgentType != "worker" || e.AgentTranscriptPath != "/at" {
			t.Fatalf("event = %+v", *e)
		}
		return codex.Continue()
	})
	if c := app.Run(); c != 0 {
		t.Fatalf("exit %d stderr=%q", c, iox.err.String())
	}
	if iox.out.Len() != 0 {
		t.Fatalf("stdout = %q", iox.out.String())
	}
}

func TestApp_CodexPreToolUseHook(t *testing.T) {
	iox := &testIO{in: []byte(`{"session_id":"s","turn_id":"t","hook_event_name":"PreToolUse","tool_name":"request_user_input","tool_input":{"questions":[{"question":"Which one?"}]},"tool_use_id":"call-1"}`)}
	app := New(Config{
		Name: "t",
		Args: []string{"plugin-kit-ai", "CodexPreToolUse"},
		IO:   iox,
		Env:  testEnv{},
	})
	app.Codex().OnPreToolUse(func(e *codex.PreToolUseEvent) *codex.Response {
		if e.ToolName != "request_user_input" || e.ToolUseID != "call-1" {
			t.Fatalf("event = %+v", *e)
		}
		return codex.Continue()
	})
	if c := app.Run(); c != 0 {
		t.Fatalf("exit %d stderr=%q", c, iox.err.String())
	}
	if iox.out.Len() != 0 || iox.err.Len() != 0 {
		t.Fatalf("observation hook wrote output: stdout=%q stderr=%q", iox.out.String(), iox.err.String())
	}
}

func TestApp_CodexPermissionRequestHook(t *testing.T) {
	iox := &testIO{in: []byte(`{"session_id":"s","turn_id":"t","hook_event_name":"PermissionRequest","tool_name":"shell","tool_input":{"command":["ls"]}}`)}
	app := New(Config{
		Name: "t",
		Args: []string{"plugin-kit-ai", "CodexPermissionRequest"},
		IO:   iox,
		Env:  testEnv{},
	})
	app.Codex().OnPermissionRequest(func(e *codex.PermissionRequestEvent) *codex.Response {
		if e.ToolName != "shell" {
			t.Fatalf("tool = %q", e.ToolName)
		}
		if string(e.ToolInput) != `{"command":["ls"]}` {
			t.Fatalf("tool input = %s", e.ToolInput)
		}
		return codex.Continue()
	})
	if c := app.Run(); c != 0 {
		t.Fatalf("exit %d stderr=%q", c, iox.err.String())
	}
	if iox.out.Len() != 0 {
		t.Fatalf("stdout = %q", iox.out.String())
	}
}

// TestResolverCodexPrefixNoCollision proves the flat resolver keeps bare event
// names owned by Claude while the prefixed names route to Codex.
func TestResolverCodexPrefixNoCollision(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw      string
		platform string
		event    string
	}{
		{raw: "Stop", platform: "claude", event: "Stop"},
		{raw: "SubagentStop", platform: "claude", event: "SubagentStop"},
		{raw: "PreToolUse", platform: "claude", event: "PreToolUse"},
		{raw: "PermissionRequest", platform: "claude", event: "PermissionRequest"},
		{raw: "CodexStop", platform: "codex", event: "Stop"},
		{raw: "CodexSubagentStop", platform: "codex", event: "SubagentStop"},
		{raw: "CodexPreToolUse", platform: "codex", event: "PreToolUse"},
		{raw: "CodexPermissionRequest", platform: "codex", event: "PermissionRequest"},
		{raw: "codexstop", platform: "codex", event: "Stop"},
		{raw: "notify", platform: "codex", event: "Notify"},
	}
	for _, tc := range cases {
		inv, err := gen.ResolveInvocation([]string{"plugin-kit-ai", tc.raw}, nil)
		if err != nil {
			t.Fatalf("%s: resolve error = %v", tc.raw, err)
		}
		if string(inv.Platform) != tc.platform || string(inv.Event) != tc.event {
			t.Fatalf("%s: resolved to %s/%s, want %s/%s", tc.raw, inv.Platform, inv.Event, tc.platform, tc.event)
		}
	}
}
