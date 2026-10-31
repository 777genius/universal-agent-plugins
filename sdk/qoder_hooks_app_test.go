package pluginkitai

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/sdk/claude"
	"github.com/777genius/plugin-kit-ai/sdk/internal/descriptors/gen"
	"github.com/777genius/plugin-kit-ai/sdk/qoder"
)

func TestApp_StopRoutingWithRegisteredPlatforms(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, raw, qoderEvent, want string
		qoderFirst                  bool
	}{
		{name: "Claude-only lowercase compatibility", raw: "stop", want: "claude"},
		{name: "mixed lowercase Claude first", raw: "stop", qoderEvent: "stop", want: "qoder"},
		{name: "mixed lowercase Qoder first", raw: "stop", qoderEvent: "stop", qoderFirst: true, want: "qoder"},
		{name: "mixed canonical Claude first", raw: "Stop", qoderEvent: "stop", want: "claude"},
		{name: "mixed canonical Qoder first", raw: "Stop", qoderEvent: "stop", qoderFirst: true, want: "claude"},
		{name: "missing Qoder Stop Claude first", raw: "stop", qoderEvent: "session-start"},
		{name: "missing Qoder Stop Qoder first", raw: "stop", qoderEvent: "session-start", qoderFirst: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			iox := &testIO{in: []byte(qoderHooksStopPayload)}
			app := New(Config{Name: "t", Args: []string{"plugin-kit-ai", tc.raw}, IO: iox, Env: testEnv{}})
			called := ""
			registerClaude := func() {
				app.Claude().OnStop(func(*claude.StopEvent) *claude.Response {
					called += "claude"
					return claude.Allow()
				})
			}
			registerQoder := func() {
				switch tc.qoderEvent {
				case "stop":
					app.Qoder().OnStop(func(*qoder.StopEvent) *qoder.StopResponse {
						called += "qoder"
						return qoder.StopAllow()
					})
				case "session-start":
					app.Qoder().OnSessionStart(func(*qoder.SessionStartEvent) *qoder.SessionStartResponse {
						called += "qoder-session-start"
						return nil
					})
				}
			}
			if tc.qoderFirst {
				registerQoder()
				registerClaude()
			} else {
				registerClaude()
				registerQoder()
			}
			code := app.Run()
			if called != tc.want {
				t.Fatalf("handler calls = %q, want %q; exit=%d stderr=%q", called, tc.want, code, iox.err.String())
			}
			if tc.want == "" {
				if code != 1 || !strings.Contains(iox.err.String(), "no handler registered for qoder/stop") || iox.out.Len() != 0 {
					t.Fatalf("expected missing Qoder Stop failure; exit=%d stdout=%q stderr=%q", code, iox.out.String(), iox.err.String())
				}
			} else if code != 0 || iox.out.String() != "{}" || iox.err.Len() != 0 {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, iox.out.String(), iox.err.String())
			}
		})
	}
}

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
