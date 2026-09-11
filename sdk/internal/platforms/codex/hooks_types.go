package codex

import "encoding/json"

// StopInput is the decoded Codex Stop lifecycle hook payload (stdin JSON).
type StopInput struct {
	SessionID            string
	TurnID               string
	TranscriptPath       string
	CWD                  string
	HookEventName        string
	Model                string
	PermissionMode       string
	StopHookActive       bool
	LastAssistantMessage string
}

// SubagentStopInput is the decoded Codex SubagentStop payload (stdin JSON).
type SubagentStopInput struct {
	SessionID            string
	TurnID               string
	TranscriptPath       string
	CWD                  string
	HookEventName        string
	Model                string
	PermissionMode       string
	StopHookActive       bool
	LastAssistantMessage string
	AgentID              string
	AgentType            string
	AgentTranscriptPath  string
}

// PreToolUseInput is the decoded Codex PreToolUse payload (stdin JSON).
// ToolInput stays raw for typed consumers.
type PreToolUseInput struct {
	SessionID      string
	TurnID         string
	TranscriptPath string
	CWD            string
	HookEventName  string
	Model          string
	PermissionMode string
	ToolName       string
	ToolInput      json.RawMessage
	ToolUseID      string
	AgentID        string
	AgentType      string
}

// PermissionRequestInput is the decoded Codex PermissionRequest payload
// (stdin JSON). ToolInput stays raw for typed consumers.
type PermissionRequestInput struct {
	SessionID      string
	TurnID         string
	TranscriptPath string
	CWD            string
	HookEventName  string
	Model          string
	PermissionMode string
	ToolName       string
	ToolInput      json.RawMessage
	AgentID        string
	AgentType      string
}

// StopOutcome acknowledges an observation-only Stop hook.
type StopOutcome struct{}

// PreToolUseOutcome acknowledges an observation-only PreToolUse hook.
type PreToolUseOutcome struct{}

// SubagentStopOutcome acknowledges an observation-only SubagentStop hook.
type SubagentStopOutcome struct{}

// PermissionRequestOutcome acknowledges an observation-only
// PermissionRequest hook.
type PermissionRequestOutcome struct{}
