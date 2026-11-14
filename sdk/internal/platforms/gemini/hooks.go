package gemini

import (
	"encoding/json"
)

type syncOutputDTO struct {
	SystemMessage      string `json:"systemMessage,omitempty"`
	SuppressOutput     bool   `json:"suppressOutput,omitempty"`
	Continue           *bool  `json:"continue,omitempty"`
	StopReason         string `json:"stopReason,omitempty"`
	Decision           string `json:"decision,omitempty"`
	Reason             string `json:"reason,omitempty"`
	HookSpecificOutput any    `json:"hookSpecificOutput,omitempty"`
}

type contextHookSpecificDTO struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext,omitempty"`
}

type afterAgentHookSpecificDTO struct {
	HookEventName string `json:"hookEventName"`
	ClearContext  bool   `json:"clearContext,omitempty"`
}

type toolHookSpecificDTO struct {
	HookEventName string          `json:"hookEventName"`
	ToolInput     json.RawMessage `json:"tool_input,omitempty"`
}

type tailToolCallRequestDTO struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type afterToolHookSpecificDTO struct {
	HookEventName       string                  `json:"hookEventName"`
	AdditionalContext   string                  `json:"additionalContext,omitempty"`
	TailToolCallRequest *tailToolCallRequestDTO `json:"tailToolCallRequest,omitempty"`
}

type modelHookSpecificDTO struct {
	HookEventName string          `json:"hookEventName"`
	LLMRequest    json.RawMessage `json:"llm_request,omitempty"`
	LLMResponse   json.RawMessage `json:"llm_response,omitempty"`
}

type toolConfigDTO struct {
	Mode                 string   `json:"mode,omitempty"`
	AllowedFunctionNames []string `json:"allowedFunctionNames,omitempty"`
}

type beforeToolSelectionHookSpecificDTO struct {
	HookEventName string         `json:"hookEventName"`
	ToolConfig    *toolConfigDTO `json:"toolConfig,omitempty"`
}
