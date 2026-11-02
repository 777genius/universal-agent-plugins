package qoder

import "encoding/json"

// BaseInput mirrors the Claude-derived hook payload fields shared by every
// Qoder hook event.
type BaseInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path,omitempty"`
	CWD            string `json:"cwd,omitempty"`
	PermissionMode string `json:"permission_mode,omitempty"`
	HookEventName  string `json:"hook_event_name"`
}

type SessionStartInput struct {
	BaseInput
	Source string `json:"source,omitempty"`
}

type UserPromptSubmitInput struct {
	BaseInput
	Prompt string `json:"prompt"`
}

type PostToolUseInput struct {
	BaseInput
	ToolName     string          `json:"tool_name"`
	ToolUseID    string          `json:"tool_use_id,omitempty"`
	ToolInput    json.RawMessage `json:"tool_input,omitempty"`
	ToolResponse json.RawMessage `json:"tool_response,omitempty"`
}

type StopInput struct {
	BaseInput
	StopHookActive       bool   `json:"stop_hook_active,omitempty"`
	LastAssistantMessage string `json:"last_assistant_message,omitempty"`
}

// StopOutcome mirrors the Claude Stop decision model.
type StopOutcome struct {
	AllowStop     bool
	BlockReason   string
	BlockExitCode int
}

// UserPromptSubmitOutcome mirrors the Claude UserPromptSubmit decision model.
type UserPromptSubmitOutcome struct {
	Allow             bool
	BlockReason       string
	BlockExitCode     int
	AdditionalContext string
}

// CommonOutcome mirrors the Claude common synchronous response envelope fields.
type CommonOutcome struct {
	Continue       *bool
	SuppressOutput bool
	StopReason     string
	Decision       string
	Reason         string
	SystemMessage  string
}

// SessionStartOutcome mirrors the Claude SessionStart context response model.
type SessionStartOutcome struct {
	CommonOutcome
	AdditionalContext string
}

// PostToolUseOutcome mirrors the Claude PostToolUse response model.
type PostToolUseOutcome struct {
	CommonOutcome
	AdditionalContext    string
	UpdatedMCPToolOutput json.RawMessage
}
