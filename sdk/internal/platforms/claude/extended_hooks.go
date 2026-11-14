package claude

import (
	"encoding/json"
)

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

type SessionEndInput struct {
	BaseInput
	Reason string `json:"reason,omitempty"`
}

type NotificationInput struct {
	BaseInput
	Message          string `json:"message,omitempty"`
	NotificationType string `json:"notification_type,omitempty"`
}

type PostToolUseInput struct {
	BaseInput
	ToolName     string          `json:"tool_name"`
	ToolUseID    string          `json:"tool_use_id,omitempty"`
	ToolInput    json.RawMessage `json:"tool_input,omitempty"`
	ToolResponse json.RawMessage `json:"tool_response,omitempty"`
}

type PostToolUseFailureInput struct {
	BaseInput
	ToolName  string          `json:"tool_name"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	ToolInput json.RawMessage `json:"tool_input,omitempty"`
	Error     string          `json:"error,omitempty"`
}

type PermissionRequestInput struct {
	BaseInput
	ToolName       string          `json:"tool_name,omitempty"`
	ToolUseID      string          `json:"tool_use_id,omitempty"`
	ToolInput      json.RawMessage `json:"tool_input,omitempty"`
	RequestedPath  string          `json:"requested_path,omitempty"`
	RequestedScope string          `json:"requested_scope,omitempty"`
}

type SubagentStartInput struct {
	BaseInput
	AgentID         string `json:"agent_id,omitempty"`
	AgentType       string `json:"agent_type,omitempty"`
	Prompt          string `json:"prompt,omitempty"`
	ParentSessionID string `json:"parent_session_id,omitempty"`
	ParentToolUseID string `json:"parent_tool_use_id,omitempty"`
}

type SubagentStopInput struct {
	BaseInput
	AgentID         string `json:"agent_id,omitempty"`
	AgentType       string `json:"agent_type,omitempty"`
	ParentSessionID string `json:"parent_session_id,omitempty"`
	ParentToolUseID string `json:"parent_tool_use_id,omitempty"`
}

type PreCompactInput struct {
	BaseInput
	Trigger string `json:"trigger,omitempty"`
}

type SetupInput struct {
	BaseInput
	SetupHookActive bool `json:"setup_hook_active,omitempty"`
}

type TeammateIdleInput struct {
	BaseInput
	TeamName     string `json:"team_name,omitempty"`
	TeammateName string `json:"teammate_name,omitempty"`
}

type TaskCompletedInput struct {
	BaseInput
	TaskID   string `json:"task_id,omitempty"`
	TaskName string `json:"task_name,omitempty"`
}

type ConfigChangeInput struct {
	BaseInput
	ChangedKeys []string `json:"changed_keys,omitempty"`
}

type WorktreeCreateInput struct {
	BaseInput
	WorktreePath string `json:"worktree_path,omitempty"`
}

type WorktreeRemoveInput struct {
	BaseInput
	WorktreePath string `json:"worktree_path,omitempty"`
}

type PermissionBehavior string

const (
	PermissionAllow PermissionBehavior = "allow"
	PermissionDeny  PermissionBehavior = "deny"
)

type PermissionRuleValue struct {
	Type   string   `json:"type,omitempty"`
	Action string   `json:"action,omitempty"`
	Paths  []string `json:"paths,omitempty"`
}

type PermissionUpdate struct {
	Type        string                `json:"type,omitempty"`
	Behavior    PermissionBehavior    `json:"behavior,omitempty"`
	Destination string                `json:"destination,omitempty"`
	Mode        string                `json:"mode,omitempty"`
	Directories []string              `json:"directories,omitempty"`
	Rules       []PermissionRuleValue `json:"rules,omitempty"`
}

type CommonOutcome struct {
	Continue       *bool
	SuppressOutput bool
	StopReason     string
	Decision       string
	Reason         string
	SystemMessage  string
}

type ContextOutcome struct {
	CommonOutcome
	AdditionalContext string
}

type PostToolUseOutcome struct {
	CommonOutcome
	AdditionalContext    string
	UpdatedMCPToolOutput json.RawMessage
}

type PermissionDecision struct {
	Behavior           PermissionBehavior
	UpdatedInput       json.RawMessage
	UpdatedPermissions []PermissionUpdate
	Message            string
	Interrupt          bool
}

type PermissionRequestOutcome struct {
	CommonOutcome
	Permission *PermissionDecision
}

type syncOutputDTO struct {
	Continue           *bool  `json:"continue,omitempty"`
	SuppressOutput     bool   `json:"suppressOutput,omitempty"`
	StopReason         string `json:"stopReason,omitempty"`
	Decision           string `json:"decision,omitempty"`
	Reason             string `json:"reason,omitempty"`
	SystemMessage      string `json:"systemMessage,omitempty"`
	HookSpecificOutput any    `json:"hookSpecificOutput,omitempty"`
}

type contextHookSpecificDTO struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext,omitempty"`
}

type postToolUseHookSpecificDTO struct {
	HookEventName        string          `json:"hookEventName"`
	AdditionalContext    string          `json:"additionalContext,omitempty"`
	UpdatedMCPToolOutput json.RawMessage `json:"updatedMcpToolOutput,omitempty"`
}

type permissionDecisionDTO struct {
	Behavior           string             `json:"behavior"`
	UpdatedInput       json.RawMessage    `json:"updatedInput,omitempty"`
	UpdatedPermissions []PermissionUpdate `json:"updatedPermissions,omitempty"`
	Message            string             `json:"message,omitempty"`
	Interrupt          bool               `json:"interrupt,omitempty"`
}

type permissionHookSpecificDTO struct {
	HookEventName string                `json:"hookEventName"`
	Decision      permissionDecisionDTO `json:"decision"`
}
