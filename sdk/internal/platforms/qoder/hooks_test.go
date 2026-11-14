package qoder

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

func decodeRoundTrip[T any](t *testing.T, in *T, label string) *T {
	t.Helper()
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var (
		out  any
		err2 error
	)
	switch label {
	case "sessionstart":
		out, _, err2 = DecodeSessionStart(runtime.Envelope{Stdin: raw})
	case "userpromptsubmit":
		out, _, err2 = DecodeUserPromptSubmit(runtime.Envelope{Stdin: raw})
	case "posttooluse":
		out, _, err2 = DecodePostToolUse(runtime.Envelope{Stdin: raw})
	case "stop":
		out, _, err2 = DecodeStop(runtime.Envelope{Stdin: raw})
	default:
		t.Fatalf("unknown label %q", label)
	}
	if err2 != nil {
		t.Fatalf("decode %s: %v", label, err2)
	}
	got, ok := out.(*T)
	if !ok {
		t.Fatalf("decode %s returned %T", label, out)
	}
	return got
}

func TestSessionStartCodecRoundTrip(t *testing.T) {
	t.Parallel()

	want := &SessionStartInput{
		BaseInput: BaseInput{
			SessionID:      "sess-1",
			TranscriptPath: "/tmp/transcript.jsonl",
			CWD:            "/repo",
			PermissionMode: "default",
			HookEventName:  "SessionStart",
		},
		Source: "startup",
	}
	got := decodeRoundTrip(t, want, "sessionstart")
	if *got != *want {
		t.Fatalf("decoded = %+v, want %+v", *got, *want)
	}

	res := EncodeSessionStart(SessionStartOutcome{AdditionalContext: "hello"})
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d stderr = %q", res.ExitCode, res.Stderr)
	}
	var body map[string]any
	if err := json.Unmarshal(res.Stdout, &body); err != nil {
		t.Fatal(err)
	}
	specific := body["hookSpecificOutput"].(map[string]any)
	if specific["hookEventName"] != "SessionStart" || specific["additionalContext"] != "hello" {
		t.Fatalf("hookSpecificOutput = %#v", specific)
	}
}

func TestUserPromptSubmitCodecRoundTrip(t *testing.T) {
	t.Parallel()

	want := &UserPromptSubmitInput{
		BaseInput: BaseInput{
			SessionID:      "sess-2",
			TranscriptPath: "/tmp/transcript.jsonl",
			HookEventName:  "UserPromptSubmit",
		},
		Prompt: "do the thing",
	}
	got := decodeRoundTrip(t, want, "userpromptsubmit")
	if *got != *want {
		t.Fatalf("decoded = %+v, want %+v", *got, *want)
	}

	res := EncodeUserPromptSubmit(UserPromptSubmitOutcome{Allow: true, AdditionalContext: "ctx"})
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d stderr = %q", res.ExitCode, res.Stderr)
	}
	if string(res.Stdout) != `{"additionalContext":"ctx"}` {
		t.Fatalf("stdout = %s", res.Stdout)
	}

	res = EncodeUserPromptSubmit(UserPromptSubmitOutcome{BlockReason: "nope"})
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d stderr = %q", res.ExitCode, res.Stderr)
	}
	if string(res.Stdout) != `{"decision":"block","reason":"nope"}` {
		t.Fatalf("stdout = %s", res.Stdout)
	}
}

func TestPostToolUseCodecRoundTrip(t *testing.T) {
	t.Parallel()

	want := &PostToolUseInput{
		BaseInput: BaseInput{
			SessionID:     "sess-3",
			HookEventName: "PostToolUse",
		},
		ToolName:     "Edit",
		ToolUseID:    "toolu-1",
		ToolInput:    json.RawMessage(`{"file_path":"a.go"}`),
		ToolResponse: json.RawMessage(`{"ok":true}`),
	}
	got := decodeRoundTrip(t, want, "posttooluse")
	if got.ToolName != want.ToolName || got.ToolUseID != want.ToolUseID ||
		string(got.ToolInput) != string(want.ToolInput) || string(got.ToolResponse) != string(want.ToolResponse) {
		t.Fatalf("decoded = %+v, want %+v", *got, *want)
	}

	res := EncodePostToolUse(PostToolUseOutcome{
		AdditionalContext:    "note",
		UpdatedMCPToolOutput: json.RawMessage(`{"fixed":true}`),
	})
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d stderr = %q", res.ExitCode, res.Stderr)
	}
	var body map[string]any
	if err := json.Unmarshal(res.Stdout, &body); err != nil {
		t.Fatal(err)
	}
	specific := body["hookSpecificOutput"].(map[string]any)
	if specific["hookEventName"] != "PostToolUse" {
		t.Fatalf("hookSpecificOutput = %#v", specific)
	}
}

func TestStopCodecRoundTrip(t *testing.T) {
	t.Parallel()

	want := &StopInput{
		BaseInput: BaseInput{
			SessionID:      "sess-4",
			TranscriptPath: "/tmp/transcript.jsonl",
			HookEventName:  "Stop",
		},
		StopHookActive:       true,
		LastAssistantMessage: "done",
	}
	got := decodeRoundTrip(t, want, "stop")
	if *got != *want {
		t.Fatalf("decoded = %+v, want %+v", *got, *want)
	}

	res := EncodeStop(StopOutcome{AllowStop: true})
	if res.ExitCode != 0 || string(res.Stdout) != "{}" {
		t.Fatalf("allow: exit = %d stdout = %s stderr = %q", res.ExitCode, res.Stdout, res.Stderr)
	}

	res = EncodeStop(StopOutcome{BlockReason: "keep going"})
	if res.ExitCode != 0 {
		t.Fatalf("block: exit = %d stderr = %q", res.ExitCode, res.Stderr)
	}
	if string(res.Stdout) != `{"decision":"block","reason":"keep going"}` {
		t.Fatalf("block stdout = %s", res.Stdout)
	}

	res = EncodeStop(StopOutcome{BlockExitCode: 2, BlockReason: "halt"})
	if res.ExitCode != 2 || !strings.Contains(res.Stderr, "halt") {
		t.Fatalf("exit2: exit = %d stderr = %q", res.ExitCode, res.Stderr)
	}
}

func TestDecodePostToolUseRequiresToolName(t *testing.T) {
	t.Parallel()

	if _, _, err := DecodePostToolUse(runtime.Envelope{Stdin: []byte(`{"session_id":"s","hook_event_name":"PostToolUse"}`)}); err == nil {
		t.Fatal("expected tool_name validation error")
	}
}
