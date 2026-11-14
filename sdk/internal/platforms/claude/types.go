package claude

import "encoding/json"

type StopInput struct {
	SessionID            string
	TranscriptPath       string
	CWD                  string
	PermissionMode       string
	HookEventName        string
	StopHookActive       bool
	LastAssistantMessage string
}

type PreToolUseInput struct {
	SessionID      string
	TranscriptPath string
	CWD            string
	PermissionMode string
	HookEventName  string
	ToolName       string
	ToolUseID      string
	ToolInput      json.RawMessage
}

type UserPromptSubmitInput struct {
	SessionID      string
	TranscriptPath string
	CWD            string
	PermissionMode string
	HookEventName  string
	Prompt         string
}

type StopOutcome struct {
	AllowStop     bool
	BlockReason   string
	BlockExitCode int
}

type PreToolUseOutcome struct {
	Permission    string
	Reason        string
	BlockExitCode int
	BlockReason   string
}

type UserPromptSubmitOutcome struct {
	Allow             bool
	BlockReason       string
	BlockExitCode     int
	AdditionalContext string
}
