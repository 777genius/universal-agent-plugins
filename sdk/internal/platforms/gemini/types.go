package gemini

import "encoding/json"

type BaseInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path,omitempty"`
	CWD            string `json:"cwd,omitempty"`
	HookEventName  string `json:"hook_event_name"`
	Timestamp      string `json:"timestamp,omitempty"`
}

type SessionStartInput struct {
	BaseInput
	Source string `json:"source,omitempty"`
}

type SessionEndInput struct {
	BaseInput
	Reason string `json:"reason,omitempty"`
}

type BeforeModelInput struct {
	BaseInput
	LLMRequest json.RawMessage `json:"llm_request,omitempty"`
}

type AfterModelInput struct {
	BaseInput
	LLMRequest  json.RawMessage `json:"llm_request,omitempty"`
	LLMResponse json.RawMessage `json:"llm_response,omitempty"`
}

type BeforeToolSelectionInput struct {
	BaseInput
	LLMRequest json.RawMessage `json:"llm_request,omitempty"`
}

type BeforeAgentInput struct {
	BaseInput
	Prompt string `json:"prompt,omitempty"`
}

type AfterAgentInput struct {
	BaseInput
	Prompt         string `json:"prompt,omitempty"`
	PromptResponse string `json:"prompt_response,omitempty"`
	StopHookActive bool   `json:"stop_hook_active,omitempty"`
}

type BeforeToolInput struct {
	BaseInput
	ToolName            string          `json:"tool_name,omitempty"`
	ToolInput           json.RawMessage `json:"tool_input,omitempty"`
	MCPContext          json.RawMessage `json:"mcp_context,omitempty"`
	OriginalRequestName string          `json:"original_request_name,omitempty"`
}

type AfterToolInput struct {
	BaseInput
	ToolName            string          `json:"tool_name,omitempty"`
	ToolInput           json.RawMessage `json:"tool_input,omitempty"`
	ToolResponse        json.RawMessage `json:"tool_response,omitempty"`
	MCPContext          json.RawMessage `json:"mcp_context,omitempty"`
	OriginalRequestName string          `json:"original_request_name,omitempty"`
}

type CommonOutcome struct {
	Continue       *bool
	SuppressOutput bool
	StopReason     string
	Decision       string
	Reason         string
	SystemMessage  string
}

type SessionStartOutcome struct {
	CommonOutcome
	AdditionalContext string
}

type SessionEndOutcome struct {
	CommonOutcome
}

type BeforeModelOutcome struct {
	CommonOutcome
	LLMRequest  json.RawMessage
	LLMResponse json.RawMessage
}

type AfterModelOutcome struct {
	CommonOutcome
	LLMResponse json.RawMessage
}

type ToolConfig struct {
	Mode                 string
	AllowedFunctionNames []string
}

type BeforeToolSelectionOutcome struct {
	SuppressOutput bool
	ToolConfig     *ToolConfig
}

type BeforeAgentOutcome struct {
	CommonOutcome
	AdditionalContext string
}

type AfterAgentOutcome struct {
	CommonOutcome
	ClearContext bool
}

type BeforeToolOutcome struct {
	CommonOutcome
	ToolInput json.RawMessage
}

type TailToolCallRequest struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type AfterToolOutcome struct {
	CommonOutcome
	AdditionalContext   string
	TailToolCallRequest *TailToolCallRequest
}
