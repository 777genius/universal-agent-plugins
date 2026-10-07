package pluginkitai_test

import (
	"strings"
	"testing"

	pluginkitai "github.com/777genius/plugin-kit-ai/sdk"
)

// Synthetic Local wire, bound to VS Code 1.140.0 (07f806f999227108933c2e30515b26eecc1fda74)
// chatHookService commonInput and the official Local hooks reference updated
// 2026-09-30. The supplied native TEST packet reports genuine Stop/false/session
// presence but contains facts, not this raw payload. This is injected_contract.
const localStopInput = `{"timestamp":"2026-10-01T10:41:04.061Z","hook_event_name":"Stop","cwd":"/TEST/ü project","session_id":"TEST-opaque-session","transcript_path":"/TEST/private-do-not-read","stop_hook_active":false}`

// Red: the public compiled module cannot dispatch both Local selectors through
// actual typed callbacks, loses native strings/pointer presence, or serializes
// nil/empty observer results with control fields or spurious mismatch warnings.
func TestVSCodeLocalPublicProcess(t *testing.T) {
	c := buildLocalConsumer(t)
	t.Run("common strings and neutral nil/empty", func(t *testing.T) {
		for _, mode := range []string{"nil", "empty"} {
			got := c.run(t, "VSCodeLocalStop", mode, localStopInput)
			if got.Code != 0 || got.Stdout != "{}" || got.Stderr != "" || got.Calls != 1 ||
				got.Callback != "Stop" || got.NativeName != "Stop" || !got.Eligible || !got.Known || got.Active ||
				got.Timestamp != "2026-10-01T10:41:04.061Z" || got.CWD != "/TEST/ü project" ||
				got.SessionID != "TEST-opaque-session" || got.TranscriptPath != "/TEST/private-do-not-read" {
				t.Fatalf("lost native contract: %+v", got)
			}
		}
	})
	t.Run("subagent typed separation", func(t *testing.T) {
		input := strings.Replace(localStopInput, `"hook_event_name":"Stop"`, `"hook_event_name":"SubagentStop"`, 1)
		input = strings.TrimSuffix(input, "}") + `,"agent_id":"TEST-agent","agent_type":"TEST-custom"}`
		for _, mode := range []string{"nil", "empty"} {
			got := c.run(t, "VSCodeLocalSubagentStop", mode, input)
			if got.Code != 0 || got.Stdout != "{}" || got.Stderr != "" || got.Calls != 1 || got.Eligible ||
				got.Callback != "SubagentStop" || got.NativeName != "SubagentStop" || !got.Known || got.Active ||
				got.AgentID != "TEST-agent" || got.AgentType != "TEST-custom" || got.SessionID != "TEST-opaque-session" ||
				got.Timestamp != "2026-10-01T10:41:04.061Z" {
				t.Fatalf("subagent admitted as root Stop or fields lost: %+v", got)
			}
		}
	})
	t.Run("unknown fields remain forward compatible", func(t *testing.T) {
		input := strings.TrimSuffix(localStopInput, "}") + `,"future":{"nested":[true,null,9007199254740993]}}`
		got := c.run(t, "VSCodeLocalStop", "empty", input)
		if got.Code != 0 || got.Stdout != "{}" || got.Stderr != "" || !got.Eligible {
			t.Fatalf("future field changed admission: %+v", got)
		}
	})
	t.Run("case folded argv keeps exact native spelling", func(t *testing.T) {
		got := c.run(t, "vScOdElOcAlStOp", "nil", localStopInput)
		if got.Callback != "Stop" || got.NativeName != "Stop" || got.Code != 0 || got.Stdout != "{}" || got.Stderr != "" {
			t.Fatalf("case-folded dispatch changed: %+v", got)
		}
	})
}

// Red: unknown/recursive stop becomes explicit false, optional session becomes
// required, or a native name mismatch is rewritten by selector dispatch and
// wrongly admitted by a consumer checking the actual event and bool presence.
func TestVSCodeLocalAdmissionFacts(t *testing.T) {
	c := buildLocalConsumer(t)
	cases := []struct {
		name, input, native     string
		known, active, eligible bool
	}{
		{"false", localStopInput, "Stop", true, false, true},
		{"true", strings.Replace(localStopInput, `:false`, `:true`, 1), "Stop", true, true, false},
		{"missing", strings.Replace(localStopInput, `,"stop_hook_active":false`, "", 1), "Stop", false, false, false},
		{"null", strings.Replace(localStopInput, `:false`, `:null`, 1), "Stop", false, false, false},
		{"subagent on Stop selector", strings.Replace(localStopInput, `"Stop"`, `"SubagentStop"`, 1), "SubagentStop", true, false, false},
		{"different harness", strings.Replace(localStopInput, `"Stop"`, `"agentStop"`, 1), "agentStop", true, false, false},
		{"lowercase native", strings.Replace(localStopInput, `"Stop"`, `"stop"`, 1), "stop", true, false, false},
		{"missing native name", strings.Replace(localStopInput, `"hook_event_name":"Stop",`, "", 1), "", true, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := c.run(t, "VSCodeLocalStop", "empty", tc.input)
			if got.Code != 0 || got.Stdout != "{}" || got.Calls != 1 || got.Callback != "Stop" ||
				got.NativeName != tc.native || got.Known != tc.known || got.Active != tc.active || got.Eligible != tc.eligible {
				t.Fatalf("native admission facts changed: %+v", got)
			}
			if tc.native == "SubagentStop" || tc.native == "agentStop" {
				if !strings.Contains(got.Stderr, "does not match argv hook") {
					t.Fatal("mismatch lost existing SDK diagnostic")
				}
			}
		})
	}
	for _, member := range []string{`"session_id":"TEST-opaque-session",`, `"cwd":"/TEST/ü project",`, `"transcript_path":"/TEST/private-do-not-read",`} {
		// The transcript member is last among common fields; deletion remains JSON.
		got := c.run(t, "VSCodeLocalStop", "empty", strings.Replace(localStopInput, member, "", 1))
		if got.Code != 0 || !got.Eligible || got.Stdout != "{}" {
			t.Fatalf("optional common field became required: %+v", got)
		}
	}
}

// Red: malformed Local scalars reach callbacks, or the SDK payload bound is
// bypassed by injected IO. Independent expected codes/call counts are checked;
// trailing JSON and a frame exactly at the bound also exercise the shared codec.
func TestVSCodeLocalDecodeErrors(t *testing.T) {
	c := buildLocalConsumer(t)
	wrongScalars := []string{
		`"stop_hook_active":"false"`, `"stop_hook_active":0`, `"stop_hook_active":[]`,
		`"timestamp":123`, `"timestamp":{}`, `"hook_event_name":false`,
		`"cwd":[]`, `"session_id":123`, `"transcript_path":{}`,
	}
	for _, member := range wrongScalars {
		key := strings.SplitN(member, ":", 2)[0]
		input := `{"hook_event_name":"Stop",` + member + `}`
		if key == `"hook_event_name"` {
			input = "{" + member + "}"
		}
		t.Run(member, func(t *testing.T) {
			got := c.run(t, "VSCodeLocalStop", "empty", input)
			if got.Code != 1 || got.Calls != 0 || got.Stdout != "" || got.Stderr == "" {
				t.Fatalf("wrong scalar accepted: %+v", got)
			}
		})
	}
	for _, input := range []string{`{`, localStopInput + `{}`, `[]`} {
		got := c.run(t, "VSCodeLocalStop", "empty", input)
		if got.Code != 1 || got.Calls != 0 || got.Stdout != "" || got.Stderr == "" {
			t.Fatalf("invalid frame accepted: %+v", got)
		}
	}
	for _, extra := range []int{0, 1} {
		input := localStopInput + strings.Repeat(" ", pluginkitai.MaxPayloadBytes-len(localStopInput)+extra)
		got := c.run(t, "VSCodeLocalStop", "empty", input)
		if extra == 0 && (got.Code != 0 || got.Calls != 1 || got.Stdout != "{}") {
			t.Fatalf("exact size limit rejected: %+v", got)
		}
		if extra == 1 && (got.Code != 1 || got.Calls != 0 || got.Stdout != "" || !strings.Contains(got.Stderr, "exceeds max payload size")) {
			t.Fatalf("oversize injected IO bypassed bound: %+v", got)
		}
	}
}

// Red: Local registration steals another platform's alias, unsupported Local
// hooks acquire accidental support, or ordinary RunContext starts hiding errors
// behind a fail-open supervisor. Existing permission/legacy process probes supply
// the positive cross-platform contracts; this consumer must never call Local.
func TestVSCodeLocalDispatchIsolationAndErrors(t *testing.T) {
	c := buildLocalConsumer(t)
	for _, selector := range []string{"Stop", "SubagentStop", "CursorStop", "CodexStop", "GeminiAfterAgent", "GeminiNotification", "Notification", "VSCodeLocalPreToolUse", "vscode", "copilot"} {
		got := c.run(t, selector, "empty", localStopInput)
		if got.Code != 1 || got.Calls != 0 || got.Stdout != "" || got.Stderr == "" {
			t.Fatalf("alias %s was redirected to Local: %+v", selector, got)
		}
	}
	// Red: the Cursor-only runner starts dispatching new registered Local hooks.
	for _, selector := range []string{"VSCodeLocalStop", "VSCodeLocalSubagentStop"} {
		got := c.run(t, selector, "cursor-runner", localStopInput)
		if got.Code != 0 || got.Calls != 0 || got.Stdout != "{}\n" || got.Stderr != "" {
			t.Fatalf("Cursor runner dispatched Local: %+v", got)
		}
	}
	for _, mode := range []string{"read-error", "write-error", "handler-error", "canceled", "panic"} {
		t.Run(mode, func(t *testing.T) {
			got := c.run(t, "VSCodeLocalStop", mode, localStopInput)
			if got.Code != 1 || got.Stdout != "" || got.Stderr == "" {
				t.Fatalf("ordinary SDK error hidden by neutral runner: %+v", got)
			}
		})
	}
}

// Red: beta observer metadata disappears/promotes or implies scaffold, native
// live, install or stable qualification for this runtime-only library slice.
func TestVSCodeLocalSupportContract(t *testing.T) {
	want := map[string]string{"Stop": "vscode_local_stop", "SubagentStop": "vscode_local_subagent_stop"}
	for _, entry := range pluginkitai.Supported() {
		if entry.Platform != "vscode-local" {
			continue
		}
		capability, ok := want[string(entry.Event)]
		if !ok || entry.Maturity != "beta" || entry.Status != "runtime_supported" || entry.V1Target ||
			entry.ScaffoldSupport || entry.ValidateSupport || entry.LiveTestProfile != "" ||
			entry.Carrier != "stdin_json" || entry.InvocationKind != "argv_command_casefold" ||
			len(entry.TransportModes) != 1 || entry.TransportModes[0] != "process" ||
			len(entry.Capabilities) != 1 || string(entry.Capabilities[0]) != capability {
			t.Fatalf("unqualified public metadata: %+v", entry)
		}
		delete(want, string(entry.Event))
	}
	if len(want) != 0 {
		t.Fatalf("missing Local support entries: %v", want)
	}
}
