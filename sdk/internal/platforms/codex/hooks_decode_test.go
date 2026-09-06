package codex

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

// realStopPayload is the verbatim payload captured from a live Codex CLI
// v0.152.0 Stop hook run (transcript path shortened).
const realStopPayload = `{"session_id":"01a05cd8-b495-7f80-a36b-cc0aa98efc05","turn_id":"01a05cd8-b51b-7343-8b75-b2d4ad9e276e","transcript_path":"/tmp/rollout-2026.jsonl","cwd":"/tmp/proj","hook_event_name":"Stop","model":"gpt-5.6-sol","permission_mode":"bypassPermissions","stop_hook_active":false,"last_assistant_message":"OK"}`

func TestDecodeStopRealPayload(t *testing.T) {
	t.Parallel()

	v, name, err := DecodeStop(runtime.Envelope{Stdin: []byte(realStopPayload)})
	if err != nil {
		t.Fatalf("DecodeStop() error = %v", err)
	}
	if name != "Stop" {
		t.Fatalf("hook name = %q, want Stop", name)
	}
	in, ok := v.(*StopInput)
	if !ok {
		t.Fatalf("DecodeStop() type = %T", v)
	}
	want := StopInput{
		SessionID:            "01a05cd8-b495-7f80-a36b-cc0aa98efc05",
		TurnID:               "01a05cd8-b51b-7343-8b75-b2d4ad9e276e",
		TranscriptPath:       "/tmp/rollout-2026.jsonl",
		CWD:                  "/tmp/proj",
		HookEventName:        "Stop",
		Model:                "gpt-5.6-sol",
		PermissionMode:       "bypassPermissions",
		StopHookActive:       false,
		LastAssistantMessage: "OK",
	}
	if *in != want {
		t.Fatalf("DecodeStop() = %+v, want %+v", *in, want)
	}
}

func TestDecodeStopNullAndMissingFields(t *testing.T) {
	t.Parallel()

	payload := `{"session_id":"s","turn_id":"t","transcript_path":null,"cwd":"/x","hook_event_name":"Stop","last_assistant_message":null}`
	v, _, err := DecodeStop(runtime.Envelope{Stdin: []byte(payload)})
	if err != nil {
		t.Fatalf("DecodeStop() error = %v", err)
	}
	in := v.(*StopInput)
	if in.TranscriptPath != "" || in.LastAssistantMessage != "" || in.Model != "" {
		t.Fatalf("nullable/missing fields not zero: %+v", *in)
	}
}

func TestDecodeStopMultibyteUTF8(t *testing.T) {
	t.Parallel()

	payload := `{"session_id":"s","turn_id":"t","hook_event_name":"Stop","last_assistant_message":"готово ✅ 完了"}`
	v, _, err := DecodeStop(runtime.Envelope{Stdin: []byte(payload)})
	if err != nil {
		t.Fatalf("DecodeStop() error = %v", err)
	}
	if got := v.(*StopInput).LastAssistantMessage; got != "готово ✅ 完了" {
		t.Fatalf("LastAssistantMessage = %q", got)
	}
}

func TestDecodeStopRejectsEmptyAndMalformed(t *testing.T) {
	t.Parallel()

	for name, body := range map[string][]byte{
		"empty":     nil,
		"blank":     []byte("   "),
		"malformed": []byte("{oops"),
	} {
		if _, _, err := DecodeStop(runtime.Envelope{Stdin: body}); err == nil {
			t.Fatalf("%s: DecodeStop() expected error", name)
		}
	}
}

func TestDecodeStopSizeGuard(t *testing.T) {
	t.Parallel()

	prefix := `{"session_id":"s","turn_id":"t","hook_event_name":"Stop","last_assistant_message":"`
	suffix := `"}`
	pad := func(total int) []byte {
		fill := total - len(prefix) - len(suffix)
		if fill < 0 {
			t.Fatalf("bad size %d", total)
		}
		return []byte(prefix + strings.Repeat("a", fill) + suffix)
	}

	if _, _, err := DecodeStop(runtime.Envelope{Stdin: pad(runtime.MaxPayloadBytes - 1)}); err != nil {
		t.Fatalf("limit-1 payload rejected: %v", err)
	}
	if _, _, err := DecodeStop(runtime.Envelope{Stdin: pad(runtime.MaxPayloadBytes)}); err != nil {
		t.Fatalf("limit payload rejected: %v", err)
	}
	if _, _, err := DecodeStop(runtime.Envelope{Stdin: pad(runtime.MaxPayloadBytes + 1)}); err == nil {
		t.Fatal("limit+1 payload accepted")
	}
}

func TestDecodeSubagentStopAllFields(t *testing.T) {
	t.Parallel()

	payload := `{"session_id":"s","turn_id":"t","transcript_path":"/tp","cwd":"/x","hook_event_name":"SubagentStop","model":"m","permission_mode":"default","stop_hook_active":true,"last_assistant_message":"done","agent_id":"a1","agent_type":"explorer","agent_transcript_path":"/at"}`
	v, name, err := DecodeSubagentStop(runtime.Envelope{Stdin: []byte(payload)})
	if err != nil {
		t.Fatalf("DecodeSubagentStop() error = %v", err)
	}
	if name != "SubagentStop" {
		t.Fatalf("hook name = %q", name)
	}
	in := v.(*SubagentStopInput)
	want := SubagentStopInput{
		SessionID:            "s",
		TurnID:               "t",
		TranscriptPath:       "/tp",
		CWD:                  "/x",
		HookEventName:        "SubagentStop",
		Model:                "m",
		PermissionMode:       "default",
		StopHookActive:       true,
		LastAssistantMessage: "done",
		AgentID:              "a1",
		AgentType:            "explorer",
		AgentTranscriptPath:  "/at",
	}
	if *in != want {
		t.Fatalf("DecodeSubagentStop() = %+v, want %+v", *in, want)
	}
}

func TestDecodePermissionRequestAllFields(t *testing.T) {
	t.Parallel()

	payload := `{"session_id":"s","turn_id":"t","transcript_path":"/tp","cwd":"/x","hook_event_name":"PermissionRequest","model":"m","permission_mode":"default","tool_name":"shell","tool_input":{"command":["rm","-rf","/tmp/x"]},"agent_id":"a1","agent_type":"worker"}`
	v, name, err := DecodePermissionRequest(runtime.Envelope{Stdin: []byte(payload)})
	if err != nil {
		t.Fatalf("DecodePermissionRequest() error = %v", err)
	}
	if name != "PermissionRequest" {
		t.Fatalf("hook name = %q", name)
	}
	in := v.(*PermissionRequestInput)
	if in.ToolName != "shell" || in.AgentID != "a1" || in.AgentType != "worker" {
		t.Fatalf("DecodePermissionRequest() = %+v", *in)
	}
	if string(in.ToolInput) != `{"command":["rm","-rf","/tmp/x"]}` {
		t.Fatalf("ToolInput = %s", in.ToolInput)
	}
}

func TestDecodePermissionRequestOptionalAgentAbsent(t *testing.T) {
	t.Parallel()

	payload := `{"session_id":"s","turn_id":"t","hook_event_name":"PermissionRequest","tool_name":"shell","tool_input":"echo"}`
	v, _, err := DecodePermissionRequest(runtime.Envelope{Stdin: []byte(payload)})
	if err != nil {
		t.Fatalf("DecodePermissionRequest() error = %v", err)
	}
	in := v.(*PermissionRequestInput)
	if in.AgentID != "" || in.AgentType != "" {
		t.Fatalf("agent identity not zero: %+v", *in)
	}
	if string(in.ToolInput) != `"echo"` {
		t.Fatalf("arbitrary ToolInput = %s", in.ToolInput)
	}
}

func TestEncodeObservationOutcomes(t *testing.T) {
	t.Parallel()

	for name, res := range map[string]runtime.Result{
		"stop":               EncodeStop(StopOutcome{}),
		"subagent stop":      EncodeSubagentStop(SubagentStopOutcome{}),
		"permission request": EncodePermissionRequest(PermissionRequestOutcome{}),
	} {
		if res.ExitCode != 0 || len(res.Stdout) != 0 || res.Stderr != "" {
			t.Fatalf("%s: result = %+v, want empty success", name, res)
		}
	}
}

func TestEncodeObservationTypeMismatch(t *testing.T) {
	t.Parallel()

	for name, res := range map[string]runtime.Result{
		"stop":               EncodeStop(42),
		"subagent stop":      EncodeSubagentStop("x"),
		"permission request": EncodePermissionRequest(nil),
	} {
		if res.ExitCode == 0 || res.Stderr == "" {
			t.Fatalf("%s: mismatch result = %+v, want failure", name, res)
		}
	}
}
