package qoder

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

func TestDecodeSessionStartLiteralWirePayload(t *testing.T) {
	t.Parallel()
	out, hookEvent, err := DecodeSessionStart(runtime.Envelope{Stdin: []byte(`{
		"session_id": "sess-42",
		"transcript_path": "/tmp/transcript.jsonl",
		"cwd": "/repo",
		"permission_mode": "acceptEdits",
		"hook_event_name": "SessionStart",
		"source": "startup"
	}`)})
	if err != nil {
		t.Fatal(err)
	}
	if hookEvent != "SessionStart" {
		t.Fatalf("hook event = %q", hookEvent)
	}
	got, ok := out.(*SessionStartInput)
	if !ok {
		t.Fatalf("decoded %T", out)
	}
	if got.SessionID != "sess-42" || got.TranscriptPath != "/tmp/transcript.jsonl" || got.CWD != "/repo" || got.PermissionMode != "acceptEdits" || got.HookEventName != "SessionStart" || got.Source != "startup" {
		t.Fatalf("decoded = %+v", got)
	}
}

func TestDecodeUserPromptSubmitLiteralWirePayload(t *testing.T) {
	t.Parallel()
	out, hookEvent, err := DecodeUserPromptSubmit(runtime.Envelope{Stdin: []byte(`{
		"session_id": "sess-42",
		"cwd": "/repo",
		"hook_event_name": "UserPromptSubmit",
		"prompt": "ship the release"
	}`)})
	if err != nil {
		t.Fatal(err)
	}
	if hookEvent != "UserPromptSubmit" {
		t.Fatalf("hook event = %q", hookEvent)
	}
	got, ok := out.(*UserPromptSubmitInput)
	if !ok {
		t.Fatalf("decoded %T", out)
	}
	if got.Prompt != "ship the release" || got.SessionID != "sess-42" {
		t.Fatalf("decoded = %+v", got)
	}
}

func TestDecodePostToolUseLiteralWirePayload(t *testing.T) {
	t.Parallel()
	out, hookEvent, err := DecodePostToolUse(runtime.Envelope{Stdin: []byte(`{
		"session_id": "sess-42",
		"hook_event_name": "PostToolUse",
		"tool_name": "Write",
		"tool_use_id": "toolu_1",
		"tool_input": {"file_path": "/repo/notes.md", "content": "hi"},
		"tool_response": {"ok": true}
	}`)})
	if err != nil {
		t.Fatal(err)
	}
	if hookEvent != "PostToolUse" {
		t.Fatalf("hook event = %q", hookEvent)
	}
	got, ok := out.(*PostToolUseInput)
	if !ok {
		t.Fatalf("decoded %T", out)
	}
	if got.ToolName != "Write" || got.ToolUseID != "toolu_1" {
		t.Fatalf("decoded = %+v", got)
	}
	assertJSONField(t, got.ToolInput, "file_path", "/repo/notes.md")
	assertJSONField(t, got.ToolResponse, "ok", true)
}

func assertJSONField(t *testing.T, raw []byte, key string, want any) {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	if doc[key] != want {
		t.Fatalf("%s[%s] = %v, want %v", raw, key, doc[key], want)
	}
}

func TestDecodeStopLiteralWirePayload(t *testing.T) {
	t.Parallel()
	out, hookEvent, err := DecodeStop(runtime.Envelope{Stdin: []byte(`{
		"session_id": "sess-42",
		"hook_event_name": "Stop",
		"stop_hook_active": true,
		"last_assistant_message": "done"
	}`)})
	if err != nil {
		t.Fatal(err)
	}
	if hookEvent != "Stop" {
		t.Fatalf("hook event = %q", hookEvent)
	}
	got, ok := out.(*StopInput)
	if !ok {
		t.Fatalf("decoded %T", out)
	}
	if !got.StopHookActive || got.LastAssistantMessage != "done" {
		t.Fatalf("decoded = %+v", got)
	}
}

func TestEncodeSyncRejectsUnknownDecision(t *testing.T) {
	t.Parallel()
	result := EncodeSessionStart(SessionStartOutcome{CommonOutcome: CommonOutcome{Decision: "maybe"}})
	if result.ExitCode == 0 {
		t.Fatalf("unknown decision must fail: %+v", result)
	}
	if !strings.Contains(result.Stderr, `unknown decision "maybe"`) {
		t.Fatalf("stderr = %q", result.Stderr)
	}
}
