package gemini

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestSessionStartHelpers(t *testing.T) {
	t.Parallel()

	if got := SessionStartContinue(); got == nil {
		t.Fatal("SessionStartContinue() = nil")
	} else if got.AdditionalContext != "" {
		t.Fatalf("SessionStartContinue().AdditionalContext = %q", got.AdditionalContext)
	}

	if got := SessionStartAddContext("repo memory"); got == nil {
		t.Fatal("SessionStartAddContext() = nil")
	} else if got.AdditionalContext != "repo memory" {
		t.Fatalf("SessionStartAddContext().AdditionalContext = %q", got.AdditionalContext)
	}

	if got := SessionStartMessage("hello"); got == nil {
		t.Fatal("SessionStartMessage() = nil")
	} else if got.SystemMessage != "hello" || got.AdditionalContext != "" {
		t.Fatalf("SessionStartMessage() = %#v", got)
	}
}

func TestSessionEndContinue(t *testing.T) {
	t.Parallel()

	if got := SessionEndContinue(); got == nil {
		t.Fatal("SessionEndContinue() = nil")
	} else if got.Decision != "" || got.Reason != "" {
		t.Fatalf("SessionEndContinue() = %#v", got)
	}

	if got := SessionEndMessage("bye"); got == nil {
		t.Fatal("SessionEndMessage() = nil")
	} else if got.SystemMessage != "bye" || got.Decision != "" || got.Reason != "" {
		t.Fatalf("SessionEndMessage() = %#v", got)
	}
}

func TestBeforeModelHelpers(t *testing.T) {
	t.Parallel()

	if got := BeforeModelContinue(); got == nil {
		t.Fatal("BeforeModelContinue() = nil")
	} else if got.Decision != "" || got.Reason != "" || len(got.LLMRequest) != 0 || len(got.LLMResponse) != 0 {
		t.Fatalf("BeforeModelContinue() = %#v", got)
	}

	if got := BeforeModelDeny("blocked"); got == nil {
		t.Fatal("BeforeModelDeny() = nil")
	} else if got.Decision != "deny" || got.Reason != "blocked" {
		t.Fatalf("BeforeModelDeny() = %#v", got)
	}

	req := json.RawMessage(`{"model":"gemini-2.5-pro"}`)
	if got := BeforeModelOverrideRequest(req); got == nil {
		t.Fatal("BeforeModelOverrideRequest() = nil")
	} else if !bytes.Equal(got.LLMRequest, req) {
		t.Fatalf("BeforeModelOverrideRequest().LLMRequest = %s", string(got.LLMRequest))
	}

	resp := json.RawMessage(`{"candidates":[{"content":{"role":"model"}}]}`)
	if got := BeforeModelSyntheticResponse(resp); got == nil {
		t.Fatal("BeforeModelSyntheticResponse() = nil")
	} else if !bytes.Equal(got.LLMResponse, resp) {
		t.Fatalf("BeforeModelSyntheticResponse().LLMResponse = %s", string(got.LLMResponse))
	}

	gotReq, err := BeforeModelOverrideRequestValue(map[string]any{"model": "gemini-2.5-pro"})
	if err != nil {
		t.Fatalf("BeforeModelOverrideRequestValue() error = %v", err)
	}
	if gotReq == nil || string(gotReq.LLMRequest) != `{"model":"gemini-2.5-pro"}` {
		t.Fatalf("BeforeModelOverrideRequestValue() = %#v", gotReq)
	}

	gotResp, err := BeforeModelSyntheticResponseValue(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"role": "model"}}}})
	if err != nil {
		t.Fatalf("BeforeModelSyntheticResponseValue() error = %v", err)
	}
	if gotResp == nil || string(gotResp.LLMResponse) == "" {
		t.Fatalf("BeforeModelSyntheticResponseValue() = %#v", gotResp)
	}
}

func TestBeforeModelValueHelpersRejectNonObject(t *testing.T) {
	t.Parallel()

	if _, err := BeforeModelOverrideRequestValue([]string{"bad"}); err == nil {
		t.Fatal("BeforeModelOverrideRequestValue() error = nil, want error")
	}
	if _, err := BeforeModelSyntheticResponseValue([]string{"bad"}); err == nil {
		t.Fatal("BeforeModelSyntheticResponseValue() error = nil, want error")
	}
}

func TestAfterModelHelpers(t *testing.T) {
	t.Parallel()

	if got := AfterModelContinue(); got == nil {
		t.Fatal("AfterModelContinue() = nil")
	} else if got.Decision != "" || got.Reason != "" || len(got.LLMResponse) != 0 {
		t.Fatalf("AfterModelContinue() = %#v", got)
	}

	if got := AfterModelDeny("retry"); got == nil {
		t.Fatal("AfterModelDeny() = nil")
	} else if got.Decision != "deny" || got.Reason != "retry" {
		t.Fatalf("AfterModelDeny() = %#v", got)
	}

	if got := AfterModelStop("halt"); got == nil {
		t.Fatal("AfterModelStop() = nil")
	} else if got.Continue == nil || *got.Continue || got.StopReason != "halt" {
		t.Fatalf("AfterModelStop() = %#v", got)
	}

	resp := json.RawMessage(`{"candidates":[{"content":{"role":"model"}}]}`)
	if got := AfterModelReplaceResponse(resp); got == nil {
		t.Fatal("AfterModelReplaceResponse() = nil")
	} else if !bytes.Equal(got.LLMResponse, resp) {
		t.Fatalf("AfterModelReplaceResponse().LLMResponse = %s", string(got.LLMResponse))
	}

	got, err := AfterModelReplaceResponseValue(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"role": "model"}}}})
	if err != nil {
		t.Fatalf("AfterModelReplaceResponseValue() error = %v", err)
	}
	if got == nil || string(got.LLMResponse) == "" {
		t.Fatalf("AfterModelReplaceResponseValue() = %#v", got)
	}
}

func TestAfterModelReplaceResponseValueRejectsNonObject(t *testing.T) {
	t.Parallel()

	if _, err := AfterModelReplaceResponseValue([]string{"bad"}); err == nil {
		t.Fatal("AfterModelReplaceResponseValue() error = nil, want error")
	}
}

func TestBeforeToolSelectionHelpers(t *testing.T) {
	t.Parallel()

	if got := BeforeToolSelectionContinue(); got == nil {
		t.Fatal("BeforeToolSelectionContinue() = nil")
	} else if got.Mode != "" || len(got.AllowedFunctionNames) != 0 {
		t.Fatalf("BeforeToolSelectionContinue() = %#v", got)
	}

	got := BeforeToolSelectionConfig(ToolModeAny, "read_file", "list_directory")
	if got == nil {
		t.Fatal("BeforeToolSelectionConfig() = nil")
	}
	if got.Mode != ToolModeAny {
		t.Fatalf("BeforeToolSelectionConfig().Mode = %q", got.Mode)
	}
	if len(got.AllowedFunctionNames) != 2 {
		t.Fatalf("BeforeToolSelectionConfig().AllowedFunctionNames = %#v", got.AllowedFunctionNames)
	}

	if got := BeforeToolSelectionDisableAll(); got == nil {
		t.Fatal("BeforeToolSelectionDisableAll() = nil")
	} else if got.Mode != ToolModeNone {
		t.Fatalf("BeforeToolSelectionDisableAll() = %#v", got)
	}

	if got := BeforeToolSelectionAllowOnly("read_file", "list_directory"); got == nil {
		t.Fatal("BeforeToolSelectionAllowOnly() = nil")
	} else if got.Mode != ToolModeAny || len(got.AllowedFunctionNames) != 2 {
		t.Fatalf("BeforeToolSelectionAllowOnly() = %#v", got)
	}

	if got := BeforeToolSelectionForceAny("read_file"); got == nil {
		t.Fatal("BeforeToolSelectionForceAny() = nil")
	} else if got.Mode != ToolModeAny || len(got.AllowedFunctionNames) != 1 || got.AllowedFunctionNames[0] != "read_file" {
		t.Fatalf("BeforeToolSelectionForceAny() = %#v", got)
	}

	if got := BeforeToolSelectionForceAuto("read_file"); got == nil {
		t.Fatal("BeforeToolSelectionForceAuto() = nil")
	} else if got.Mode != ToolModeAuto || len(got.AllowedFunctionNames) != 0 {
		t.Fatalf("BeforeToolSelectionForceAuto() = %#v", got)
	}

	if got := BeforeToolSelectionQuiet(); got == nil {
		t.Fatal("BeforeToolSelectionQuiet() = nil")
	} else if !got.SuppressOutput || got.Mode != "" || len(got.AllowedFunctionNames) != 0 {
		t.Fatalf("BeforeToolSelectionQuiet() = %#v", got)
	}
}

func TestBeforeAgentHelpers(t *testing.T) {
	t.Parallel()

	if got := BeforeAgentContinue(); got == nil {
		t.Fatal("BeforeAgentContinue() = nil")
	} else if got.Decision != "" || got.Reason != "" || got.AdditionalContext != "" {
		t.Fatalf("BeforeAgentContinue() = %#v", got)
	}

	if got := BeforeAgentAddContext("repo memory"); got == nil {
		t.Fatal("BeforeAgentAddContext() = nil")
	} else if got.AdditionalContext != "repo memory" {
		t.Fatalf("BeforeAgentAddContext() = %#v", got)
	}

	if got := BeforeAgentAllow(); got == nil {
		t.Fatal("BeforeAgentAllow() = nil")
	} else if got.Decision != "allow" || got.Reason != "" {
		t.Fatalf("BeforeAgentAllow() = %#v", got)
	}

	if got := BeforeAgentDeny("blocked"); got == nil {
		t.Fatal("BeforeAgentDeny() = nil")
	} else if got.Decision != "deny" || got.Reason != "blocked" {
		t.Fatalf("BeforeAgentDeny() = %#v", got)
	}

	if got := BeforeAgentStop("pause"); got == nil {
		t.Fatal("BeforeAgentStop() = nil")
	} else if got.Continue == nil || *got.Continue || got.StopReason != "pause" || got.Reason != "pause" {
		t.Fatalf("BeforeAgentStop() = %#v", got)
	}
}

func TestAfterAgentHelpers(t *testing.T) {
	t.Parallel()

	if got := AfterAgentContinue(); got == nil {
		t.Fatal("AfterAgentContinue() = nil")
	} else if got.Decision != "" || got.Reason != "" || got.ClearContext {
		t.Fatalf("AfterAgentContinue() = %#v", got)
	}

	if got := AfterAgentAllow(); got == nil {
		t.Fatal("AfterAgentAllow() = nil")
	} else if got.Decision != "allow" || got.Reason != "" {
		t.Fatalf("AfterAgentAllow() = %#v", got)
	}

	if got := AfterAgentDeny("retry"); got == nil {
		t.Fatal("AfterAgentDeny() = nil")
	} else if got.Decision != "deny" || got.Reason != "retry" {
		t.Fatalf("AfterAgentDeny() = %#v", got)
	}

	if got := AfterAgentStop("stop"); got == nil {
		t.Fatal("AfterAgentStop() = nil")
	} else if got.Continue == nil || *got.Continue || got.StopReason != "stop" {
		t.Fatalf("AfterAgentStop() = %#v", got)
	}

	if got := AfterAgentClearContext(); got == nil {
		t.Fatal("AfterAgentClearContext() = nil")
	} else if !got.ClearContext {
		t.Fatalf("AfterAgentClearContext() = %#v", got)
	}
}

func TestBeforeToolHelpers(t *testing.T) {
	t.Parallel()

	if got := BeforeToolContinue(); got == nil {
		t.Fatal("BeforeToolContinue() = nil")
	} else if got.Decision != "" || got.Reason != "" || len(got.ToolInput) != 0 {
		t.Fatalf("BeforeToolContinue() = %#v", got)
	}

	if got := BeforeToolAllow(); got == nil {
		t.Fatal("BeforeToolAllow() = nil")
	} else if got.Decision != "allow" || got.Reason != "" {
		t.Fatalf("BeforeToolAllow() = %#v", got)
	}

	if got := BeforeToolDeny("blocked"); got == nil {
		t.Fatal("BeforeToolDeny() = nil")
	} else if got.Decision != "deny" || got.Reason != "blocked" {
		t.Fatalf("BeforeToolDeny() = %#v", got)
	}

	if got := BeforeToolStop("halt"); got == nil {
		t.Fatal("BeforeToolStop() = nil")
	} else if got.Continue == nil || *got.Continue || got.StopReason != "halt" {
		t.Fatalf("BeforeToolStop() = %#v", got)
	}

	input := json.RawMessage(`{"content":"hello"}`)
	if got := BeforeToolRewriteInput(input); got == nil {
		t.Fatal("BeforeToolRewriteInput() = nil")
	} else if !bytes.Equal(got.ToolInput, input) {
		t.Fatalf("BeforeToolRewriteInput().ToolInput = %s", string(got.ToolInput))
	}

	got, err := BeforeToolRewriteInputValue(map[string]any{"content": "rewritten"})
	if err != nil {
		t.Fatalf("BeforeToolRewriteInputValue() error = %v", err)
	}
	if got == nil {
		t.Fatal("BeforeToolRewriteInputValue() = nil")
	}
	if string(got.ToolInput) != `{"content":"rewritten"}` {
		t.Fatalf("BeforeToolRewriteInputValue().ToolInput = %s", string(got.ToolInput))
	}
}

func TestBeforeToolRewriteInputValueRejectsNonObject(t *testing.T) {
	t.Parallel()

	if _, err := BeforeToolRewriteInputValue([]string{"not", "an", "object"}); err == nil {
		t.Fatal("BeforeToolRewriteInputValue() error = nil, want error")
	}
}

func TestAfterToolHelpers(t *testing.T) {
	t.Parallel()

	if got := AfterToolContinue(); got == nil {
		t.Fatal("AfterToolContinue() = nil")
	} else if got.Decision != "" || got.Reason != "" {
		t.Fatalf("AfterToolContinue() = %#v", got)
	}

	if got := AfterToolAllow(); got == nil {
		t.Fatal("AfterToolAllow() = nil")
	} else if got.Decision != "allow" || got.Reason != "" {
		t.Fatalf("AfterToolAllow() = %#v", got)
	}

	if got := AfterToolDeny("blocked"); got == nil {
		t.Fatal("AfterToolDeny() = nil")
	} else if got.Decision != "deny" || got.Reason != "blocked" {
		t.Fatalf("AfterToolDeny() = %#v", got)
	}

	if got := AfterToolStop("halt"); got == nil {
		t.Fatal("AfterToolStop() = nil")
	} else if got.Continue == nil || *got.Continue || got.StopReason != "halt" {
		t.Fatalf("AfterToolStop() = %#v", got)
	}

	if got := AfterToolAddContext("redacted"); got == nil {
		t.Fatal("AfterToolAddContext() = nil")
	} else if got.AdditionalContext != "redacted" {
		t.Fatalf("AfterToolAddContext() = %#v", got)
	}

	got, err := AfterToolTailCallValue("read_file", map[string]any{"file_path": "README.md"})
	if err != nil {
		t.Fatalf("AfterToolTailCallValue() error = %v", err)
	}
	if got == nil || got.TailToolCallRequest == nil {
		t.Fatal("AfterToolTailCallValue() missing tail call")
	}
	if got.TailToolCallRequest.Name != "read_file" {
		t.Fatalf("AfterToolTailCallValue().Name = %q", got.TailToolCallRequest.Name)
	}
	if string(got.TailToolCallRequest.Args) != `{"file_path":"README.md"}` {
		t.Fatalf("AfterToolTailCallValue().Args = %s", string(got.TailToolCallRequest.Args))
	}
}

func TestAfterToolTailCallValueRejectsNonObject(t *testing.T) {
	t.Parallel()

	if _, err := AfterToolTailCallValue("read_file", []string{"README.md"}); err == nil {
		t.Fatal("AfterToolTailCallValue() error = nil, want error")
	}
}
