package gemini

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

func TestDecodeSessionStart(t *testing.T) {
	v, name, err := DecodeSessionStart(runtime.Envelope{
		Stdin: []byte(`{"session_id":"s","cwd":"/","hook_event_name":"SessionStart","source":"startup"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if name != "SessionStart" {
		t.Fatalf("name = %q", name)
	}
	ev, ok := v.(*SessionStartInput)
	if !ok {
		t.Fatalf("type = %T", v)
	}
	if ev.Source != "startup" {
		t.Fatalf("source = %q", ev.Source)
	}
}

func TestDecodeSessionEnd(t *testing.T) {
	v, name, err := DecodeSessionEnd(runtime.Envelope{
		Stdin: []byte(`{"session_id":"s","cwd":"/","hook_event_name":"SessionEnd","reason":"prompt_input_exit"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if name != "SessionEnd" {
		t.Fatalf("name = %q", name)
	}
	ev, ok := v.(*SessionEndInput)
	if !ok {
		t.Fatalf("type = %T", v)
	}
	if ev.Reason != "prompt_input_exit" {
		t.Fatalf("reason = %q", ev.Reason)
	}
}

func TestDecodeBeforeModel(t *testing.T) {
	v, name, err := DecodeBeforeModel(runtime.Envelope{
		Stdin: []byte(`{"session_id":"s","cwd":"/","hook_event_name":"BeforeModel","llm_request":{"model":"gemini-2.5-pro","messages":[{"role":"user","content":"hi"}]}}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if name != "BeforeModel" {
		t.Fatalf("name = %q", name)
	}
	ev, ok := v.(*BeforeModelInput)
	if !ok {
		t.Fatalf("type = %T", v)
	}
	if string(ev.LLMRequest) == "" {
		t.Fatal("llm_request missing")
	}
}

func TestDecodeAfterModel(t *testing.T) {
	v, name, err := DecodeAfterModel(runtime.Envelope{
		Stdin: []byte(`{"session_id":"s","cwd":"/","hook_event_name":"AfterModel","llm_request":{"model":"gemini-2.5-pro"},"llm_response":{"candidates":[{"content":{"role":"model","parts":["ok"]}}]}}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if name != "AfterModel" {
		t.Fatalf("name = %q", name)
	}
	ev, ok := v.(*AfterModelInput)
	if !ok {
		t.Fatalf("type = %T", v)
	}
	if string(ev.LLMRequest) == "" || string(ev.LLMResponse) == "" {
		t.Fatalf("event = %#v", ev)
	}
}

func TestDecodeBeforeToolSelection(t *testing.T) {
	v, name, err := DecodeBeforeToolSelection(runtime.Envelope{
		Stdin: []byte(`{"session_id":"s","cwd":"/","hook_event_name":"BeforeToolSelection","llm_request":{"model":"gemini-2.5-pro","messages":[{"role":"user","content":"hi"}]}}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if name != "BeforeToolSelection" {
		t.Fatalf("name = %q", name)
	}
	ev, ok := v.(*BeforeToolSelectionInput)
	if !ok {
		t.Fatalf("type = %T", v)
	}
	if string(ev.LLMRequest) == "" {
		t.Fatal("llm_request missing")
	}
}

func TestDecodeBeforeAgent(t *testing.T) {
	v, name, err := DecodeBeforeAgent(runtime.Envelope{
		Stdin: []byte(`{"session_id":"s","cwd":"/","hook_event_name":"BeforeAgent","prompt":"hello"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if name != "BeforeAgent" {
		t.Fatalf("name = %q", name)
	}
	ev, ok := v.(*BeforeAgentInput)
	if !ok {
		t.Fatalf("type = %T", v)
	}
	if ev.Prompt != "hello" {
		t.Fatalf("prompt = %q", ev.Prompt)
	}
}

func TestDecodeAfterAgent(t *testing.T) {
	v, name, err := DecodeAfterAgent(runtime.Envelope{
		Stdin: []byte(`{"session_id":"s","cwd":"/","hook_event_name":"AfterAgent","prompt":"hello","prompt_response":"ok","stop_hook_active":true}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if name != "AfterAgent" {
		t.Fatalf("name = %q", name)
	}
	ev, ok := v.(*AfterAgentInput)
	if !ok {
		t.Fatalf("type = %T", v)
	}
	if ev.Prompt != "hello" || ev.PromptResponse != "ok" || !ev.StopHookActive {
		t.Fatalf("event = %#v", ev)
	}
}

func TestDecodeBeforeToolMalformedJSON(t *testing.T) {
	_, _, err := DecodeBeforeTool(runtime.Envelope{Stdin: []byte("{")})
	if err == nil || !strings.Contains(err.Error(), "decode Gemini before tool input") {
		t.Fatalf("err = %v", err)
	}
}

func TestDecodeBeforeToolRejectsOversizedPayload(t *testing.T) {
	body := `{"session_id":"s","cwd":"/","hook_event_name":"BeforeTool","tool_name":"read_file","tool_input":{"value":"` + strings.Repeat("a", runtime.MaxPayloadBytes) + `"}}`
	_, _, err := DecodeBeforeTool(runtime.Envelope{Stdin: []byte(body)})
	if err == nil || !strings.Contains(err.Error(), "exceeds max payload size") {
		t.Fatalf("err = %v", err)
	}
}

func TestDecodeAfterTool(t *testing.T) {
	v, name, err := DecodeAfterTool(runtime.Envelope{
		Stdin: []byte(`{"session_id":"s","cwd":"/","hook_event_name":"AfterTool","tool_name":"read_file","tool_input":{"file_path":"README.md"},"tool_response":{"llmContent":"ok","returnDisplay":"ok"},"mcp_context":{"server":"filesystem"},"original_request_name":"tail.read_file"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if name != "AfterTool" {
		t.Fatalf("name = %q", name)
	}
	ev, ok := v.(*AfterToolInput)
	if !ok {
		t.Fatalf("type = %T", v)
	}
	if ev.ToolName != "read_file" {
		t.Fatalf("tool = %q", ev.ToolName)
	}
	if string(ev.ToolResponse) == "" {
		t.Fatal("tool_response missing")
	}
	if string(ev.MCPContext) == "" {
		t.Fatal("mcp_context missing")
	}
	if ev.OriginalRequestName != "tail.read_file" {
		t.Fatalf("original_request_name = %q", ev.OriginalRequestName)
	}
}

func TestDecodeBeforeTool(t *testing.T) {
	v, name, err := DecodeBeforeTool(runtime.Envelope{
		Stdin: []byte(`{"session_id":"s","cwd":"/","hook_event_name":"BeforeTool","tool_name":"read_file","tool_input":{"file_path":"README.md"},"mcp_context":{"server":"filesystem"},"original_request_name":"tail.read_file"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if name != "BeforeTool" {
		t.Fatalf("name = %q", name)
	}
	ev, ok := v.(*BeforeToolInput)
	if !ok {
		t.Fatalf("type = %T", v)
	}
	if ev.ToolName != "read_file" {
		t.Fatalf("tool = %q", ev.ToolName)
	}
	if string(ev.ToolInput) == "" {
		t.Fatal("tool_input missing")
	}
	if string(ev.MCPContext) == "" {
		t.Fatal("mcp_context missing")
	}
	if ev.OriginalRequestName != "tail.read_file" {
		t.Fatalf("original_request_name = %q", ev.OriginalRequestName)
	}
}

func TestEncodeBeforeToolOutcome(t *testing.T) {
	res := EncodeBeforeTool(BeforeToolOutcome{
		CommonOutcome: CommonOutcome{
			Decision: "deny",
			Reason:   "blocked",
		},
	})
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	if got := string(res.Stdout); !strings.Contains(got, `"decision":"deny"`) || !strings.Contains(got, `"reason":"blocked"`) {
		t.Fatalf("stdout = %q", got)
	}
}

func TestEncodeBeforeToolOutcomeRejectsNonObjectToolInput(t *testing.T) {
	res := EncodeBeforeTool(BeforeToolOutcome{
		ToolInput: json.RawMessage(`["bad"]`),
	})
	if res.ExitCode != 1 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	if !strings.Contains(res.Stderr, "hookSpecificOutput.tool_input must be a JSON object") {
		t.Fatalf("stderr = %q", res.Stderr)
	}
}

func TestEncodeBeforeToolOutcomeRejectsMalformedToolInputObject(t *testing.T) {
	res := EncodeBeforeTool(BeforeToolOutcome{
		ToolInput: json.RawMessage(`{"bad":`),
	})
	if res.ExitCode != 1 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	if !strings.Contains(res.Stderr, "hookSpecificOutput.tool_input must be valid JSON object") {
		t.Fatalf("stderr = %q", res.Stderr)
	}
}

func TestEncodeSessionStartOutcome(t *testing.T) {
	res := EncodeSessionStart(SessionStartOutcome{
		AdditionalContext: "repo memory",
	})
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	if got := string(res.Stdout); !strings.Contains(got, `"hookEventName":"SessionStart"`) || !strings.Contains(got, `"additionalContext":"repo memory"`) {
		t.Fatalf("stdout = %q", got)
	}
}

func TestEncodeSessionStartOutcomeEmptyIsMinimal(t *testing.T) {
	res := EncodeSessionStart(SessionStartOutcome{})
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	if got := string(res.Stdout); got != "{}" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestEncodeBeforeAgentOutcomeAdditionalContext(t *testing.T) {
	res := EncodeBeforeAgent(BeforeAgentOutcome{AdditionalContext: "repo memory"})
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	if got := string(res.Stdout); !strings.Contains(got, `"hookEventName":"BeforeAgent"`) || !strings.Contains(got, `"additionalContext":"repo memory"`) {
		t.Fatalf("stdout = %q", got)
	}
}

func TestEncodeAfterAgentOutcomeClearContext(t *testing.T) {
	res := EncodeAfterAgent(AfterAgentOutcome{ClearContext: true})
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	if got := string(res.Stdout); !strings.Contains(got, `"hookEventName":"AfterAgent"`) || !strings.Contains(got, `"clearContext":true`) {
		t.Fatalf("stdout = %q", got)
	}
}

func TestEncodeSessionStartOutcomeIgnoresFlowControlFields(t *testing.T) {
	continueFalse := false
	res := EncodeSessionStart(SessionStartOutcome{
		CommonOutcome: CommonOutcome{
			SystemMessage: "hello",
			Continue:      &continueFalse,
			Decision:      "deny",
			Reason:        "ignored",
			StopReason:    "ignored",
		},
		AdditionalContext: "repo memory",
	})
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	got := string(res.Stdout)
	if !strings.Contains(got, `"systemMessage":"hello"`) || !strings.Contains(got, `"additionalContext":"repo memory"`) {
		t.Fatalf("stdout = %q", got)
	}
	for _, unwanted := range []string{`"continue":`, `"decision":`, `"reason":`, `"stopReason":`} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("stdout unexpectedly contains %q: %s", unwanted, got)
		}
	}
}

func TestEncodeBeforeModelOutcomeRequestOverride(t *testing.T) {
	res := EncodeBeforeModel(BeforeModelOutcome{
		LLMRequest: json.RawMessage(`{"model":"gemini-2.5-pro"}`),
	})
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	if got := string(res.Stdout); !strings.Contains(got, `"hookEventName":"BeforeModel"`) || !strings.Contains(got, `"llm_request":{"model":"gemini-2.5-pro"}`) {
		t.Fatalf("stdout = %q", got)
	}
}

func TestEncodeBeforeModelOutcomeSyntheticResponse(t *testing.T) {
	res := EncodeBeforeModel(BeforeModelOutcome{
		LLMResponse: json.RawMessage(`{"candidates":[{"content":{"role":"model"}}]}`),
	})
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	if got := string(res.Stdout); !strings.Contains(got, `"hookEventName":"BeforeModel"`) || !strings.Contains(got, `"llm_response":{"candidates":[{"content":{"role":"model"}}]}`) {
		t.Fatalf("stdout = %q", got)
	}
}

func TestEncodeBeforeModelOutcomeRejectsNonObjectRequest(t *testing.T) {
	res := EncodeBeforeModel(BeforeModelOutcome{
		LLMRequest: json.RawMessage(`["bad"]`),
	})
	if res.ExitCode != 1 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	if !strings.Contains(res.Stderr, "hookSpecificOutput.llm_request must be a JSON object") {
		t.Fatalf("stderr = %q", res.Stderr)
	}
}

func TestEncodeAfterModelOutcomeReplacement(t *testing.T) {
	res := EncodeAfterModel(AfterModelOutcome{
		LLMResponse: json.RawMessage(`{"candidates":[{"content":{"role":"model"}}]}`),
	})
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	if got := string(res.Stdout); !strings.Contains(got, `"hookEventName":"AfterModel"`) || !strings.Contains(got, `"llm_response":{"candidates":[{"content":{"role":"model"}}]}`) {
		t.Fatalf("stdout = %q", got)
	}
}

func TestEncodeAfterModelOutcomeRejectsNonObjectResponse(t *testing.T) {
	res := EncodeAfterModel(AfterModelOutcome{
		LLMResponse: json.RawMessage(`["bad"]`),
	})
	if res.ExitCode != 1 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	if !strings.Contains(res.Stderr, "hookSpecificOutput.llm_response must be a JSON object") {
		t.Fatalf("stderr = %q", res.Stderr)
	}
}

func TestEncodeBeforeToolSelectionOutcomeConfig(t *testing.T) {
	res := EncodeBeforeToolSelection(BeforeToolSelectionOutcome{
		ToolConfig: &ToolConfig{
			Mode:                 "any",
			AllowedFunctionNames: []string{"read_file", "read_file", "list_directory"},
		},
	})
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	got := string(res.Stdout)
	if !strings.Contains(got, `"hookEventName":"BeforeToolSelection"`) || !strings.Contains(got, `"mode":"ANY"`) {
		t.Fatalf("stdout = %q", got)
	}
	if !strings.Contains(got, `"allowedFunctionNames":["read_file","list_directory"]`) {
		t.Fatalf("stdout = %q", got)
	}
}

func TestEncodeBeforeToolSelectionOutcomeDisableAll(t *testing.T) {
	res := EncodeBeforeToolSelection(BeforeToolSelectionOutcome{
		ToolConfig: &ToolConfig{Mode: "NONE"},
	})
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	if got := string(res.Stdout); !strings.Contains(got, `"mode":"NONE"`) {
		t.Fatalf("stdout = %q", got)
	}
}

func TestEncodeBeforeToolSelectionOutcomeSuppressOutputOnly(t *testing.T) {
	res := EncodeBeforeToolSelection(BeforeToolSelectionOutcome{
		SuppressOutput: true,
	})
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	if got := string(res.Stdout); got != `{"suppressOutput":true}` {
		t.Fatalf("stdout = %q", got)
	}
}

func TestEncodeBeforeToolSelectionOutcomeRejectsInvalidMode(t *testing.T) {
	res := EncodeBeforeToolSelection(BeforeToolSelectionOutcome{
		ToolConfig: &ToolConfig{Mode: "ALL"},
	})
	if res.ExitCode != 1 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	if !strings.Contains(res.Stderr, "hookSpecificOutput.toolConfig.mode must be one of AUTO, ANY, or NONE") {
		t.Fatalf("stderr = %q", res.Stderr)
	}
}

func TestEncodeBeforeToolSelectionOutcomeRejectsEmptyNames(t *testing.T) {
	res := EncodeBeforeToolSelection(BeforeToolSelectionOutcome{
		ToolConfig: &ToolConfig{
			Mode:                 "AUTO",
			AllowedFunctionNames: []string{"read_file", " "},
		},
	})
	if res.ExitCode != 1 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	if !strings.Contains(res.Stderr, "hookSpecificOutput.toolConfig.allowedFunctionNames must not contain empty names") {
		t.Fatalf("stderr = %q", res.Stderr)
	}
}

func TestEncodeBeforeToolSelectionOutcomeRejectsAllowedNamesOutsideAnyMode(t *testing.T) {
	res := EncodeBeforeToolSelection(BeforeToolSelectionOutcome{
		ToolConfig: &ToolConfig{
			Mode:                 "AUTO",
			AllowedFunctionNames: []string{"read_file"},
		},
	})
	if res.ExitCode != 1 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	if !strings.Contains(res.Stderr, "hookSpecificOutput.toolConfig.allowedFunctionNames currently requires ANY mode") {
		t.Fatalf("stderr = %q", res.Stderr)
	}
}

func TestEncodeSessionEndOutcomeIgnoresFlowControlFields(t *testing.T) {
	continueFalse := false
	res := EncodeSessionEnd(SessionEndOutcome{
		CommonOutcome: CommonOutcome{
			SystemMessage: "bye",
			Continue:      &continueFalse,
			Decision:      "deny",
			Reason:        "ignored",
			StopReason:    "ignored",
		},
	})
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	got := string(res.Stdout)
	if !strings.Contains(got, `"systemMessage":"bye"`) {
		t.Fatalf("stdout = %q", got)
	}
	for _, unwanted := range []string{`"continue":`, `"decision":`, `"reason":`, `"stopReason":`} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("stdout unexpectedly contains %q: %s", unwanted, got)
		}
	}
}

func TestEncodeAfterToolOutcomeAdditionalContext(t *testing.T) {
	res := EncodeAfterTool(AfterToolOutcome{AdditionalContext: "redacted"})
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	if got := string(res.Stdout); !strings.Contains(got, `"hookEventName":"AfterTool"`) || !strings.Contains(got, `"additionalContext":"redacted"`) {
		t.Fatalf("stdout = %q", got)
	}
}

func TestEncodeAfterToolOutcomeTailToolCall(t *testing.T) {
	res := EncodeAfterTool(AfterToolOutcome{
		TailToolCallRequest: &TailToolCallRequest{
			Name: "read_file",
			Args: json.RawMessage(`{"file_path":"README.md"}`),
		},
	})
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	if got := string(res.Stdout); !strings.Contains(got, `"tailToolCallRequest":{"name":"read_file","args":{"file_path":"README.md"}}`) {
		t.Fatalf("stdout = %q", got)
	}
}

func TestEncodeAfterToolOutcomeRejectsTailToolCallWithoutName(t *testing.T) {
	res := EncodeAfterTool(AfterToolOutcome{
		TailToolCallRequest: &TailToolCallRequest{
			Args: json.RawMessage(`{"file_path":"README.md"}`),
		},
	})
	if res.ExitCode != 1 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	if !strings.Contains(res.Stderr, "hookSpecificOutput.tailToolCallRequest.name is required") {
		t.Fatalf("stderr = %q", res.Stderr)
	}
}

func TestEncodeAfterToolOutcomeRejectsNonObjectTailToolCallArgs(t *testing.T) {
	res := EncodeAfterTool(AfterToolOutcome{
		TailToolCallRequest: &TailToolCallRequest{
			Name: "read_file",
			Args: json.RawMessage(`["bad"]`),
		},
	})
	if res.ExitCode != 1 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	if !strings.Contains(res.Stderr, "hookSpecificOutput.tailToolCallRequest.args must be a JSON object") {
		t.Fatalf("stderr = %q", res.Stderr)
	}
}

func TestEncodeAfterToolOutcomeRejectsMalformedTailToolCallArgs(t *testing.T) {
	res := EncodeAfterTool(AfterToolOutcome{
		TailToolCallRequest: &TailToolCallRequest{
			Name: "read_file",
			Args: json.RawMessage(`{"path":`),
		},
	})
	if res.ExitCode != 1 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	if !strings.Contains(res.Stderr, "hookSpecificOutput.tailToolCallRequest.args must be valid JSON object") {
		t.Fatalf("stderr = %q", res.Stderr)
	}
}
