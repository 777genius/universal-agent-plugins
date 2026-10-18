package pluginkitai

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/sdk/internal/descriptors/gen"
	"github.com/777genius/plugin-kit-ai/sdk/qoder"
)

const qoderHooksStopPayload = `{"session_id":"sess-1","transcript_path":"/tmp/transcript.jsonl","cwd":"/tmp/proj","hook_event_name":"Stop","stop_hook_active":false,"last_assistant_message":"OK"}`

// TestResolverQoderKebabCommandsOwnExactSpellings proves the exact kebab argv
// commands resolve to Qoder before Claude's casefold "Stop" alias can claim the
// lowercase spelling, while canonical Claude names stay Claude-owned.
func TestResolverQoderKebabCommandsOwnExactSpellings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw      string
		platform string
		event    string
	}{
		{raw: "session-start", platform: "qoder", event: "session_start"},
		{raw: "user-prompt-submit", platform: "qoder", event: "user_prompt_submit"},
		{raw: "post-tool-use", platform: "qoder", event: "post_tool_use"},
		{raw: "stop", platform: "qoder", event: "stop"},
		{raw: "Stop", platform: "claude", event: "Stop"},
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

func TestApp_QoderStopHook(t *testing.T) {
	iox := &testIO{in: []byte(qoderHooksStopPayload)}
	app := New(Config{
		Name: "t",
		Args: []string{"plugin-kit-ai", "stop"},
		IO:   iox,
		Env:  testEnv{},
	})
	calls := 0
	app.Qoder().OnStop(func(e *qoder.StopEvent) *qoder.StopResponse {
		calls++
		if e.SessionID != "sess-1" || e.HookEventName != "Stop" {
			t.Fatalf("event = %+v", *e)
		}
		return qoder.StopBlock("keep going")
	})
	if c := app.Run(); c != 0 {
		t.Fatalf("exit %d stderr=%q", c, iox.err.String())
	}
	if calls != 1 {
		t.Fatalf("handler calls = %d", calls)
	}
	if got := iox.out.String(); got != `{"decision":"block","reason":"keep going"}` {
		t.Fatalf("stdout = %q", got)
	}
}
