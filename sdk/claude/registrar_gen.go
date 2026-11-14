package claude

// OnConfigChange registers a handler for the Claude ConfigChange.
func (r *Registrar) OnConfigChange(fn func(*ConfigChangeEvent) *ConfigChangeResponse) {
	r.backend.Register("claude", "ConfigChange", wrapConfigChange(fn))
}

// OnNotification registers a handler for the Claude Notification.
func (r *Registrar) OnNotification(fn func(*NotificationEvent) *NotificationResponse) {
	r.backend.Register("claude", "Notification", wrapNotification(fn))
}

// OnPermissionRequest registers a handler for the Claude PermissionRequest.
func (r *Registrar) OnPermissionRequest(fn func(*PermissionRequestEvent) *PermissionRequestResponse) {
	r.backend.Register("claude", "PermissionRequest", wrapPermissionRequest(fn))
}

// OnPostToolUse registers a handler for the Claude PostToolUse.
func (r *Registrar) OnPostToolUse(fn func(*PostToolUseEvent) *PostToolUseResponse) {
	r.backend.Register("claude", "PostToolUse", wrapPostToolUse(fn))
}

// OnPostToolUseFailure registers a handler for the Claude PostToolUseFailure.
func (r *Registrar) OnPostToolUseFailure(fn func(*PostToolUseFailureEvent) *PostToolUseFailureResponse) {
	r.backend.Register("claude", "PostToolUseFailure", wrapPostToolUseFailure(fn))
}

// OnPreCompact registers a handler for the Claude PreCompact.
func (r *Registrar) OnPreCompact(fn func(*PreCompactEvent) *PreCompactResponse) {
	r.backend.Register("claude", "PreCompact", wrapPreCompact(fn))
}

// OnPreToolUse registers a handler for the Claude PreToolUse.
func (r *Registrar) OnPreToolUse(fn func(*PreToolUseEvent) *PreToolResponse) {
	r.backend.Register("claude", "PreToolUse", wrapPreToolUse(fn))
}

// OnSessionEnd registers a handler for the Claude SessionEnd.
func (r *Registrar) OnSessionEnd(fn func(*SessionEndEvent) *SessionEndResponse) {
	r.backend.Register("claude", "SessionEnd", wrapSessionEnd(fn))
}

// OnSessionStart registers a handler for the Claude SessionStart.
func (r *Registrar) OnSessionStart(fn func(*SessionStartEvent) *SessionStartResponse) {
	r.backend.Register("claude", "SessionStart", wrapSessionStart(fn))
}

// OnSetup registers a handler for the Claude Setup.
func (r *Registrar) OnSetup(fn func(*SetupEvent) *SetupResponse) {
	r.backend.Register("claude", "Setup", wrapSetup(fn))
}

// OnStop registers a handler for the Claude Stop.
func (r *Registrar) OnStop(fn func(*StopEvent) *Response) {
	r.backend.Register("claude", "Stop", wrapStop(fn))
}

// OnSubagentStart registers a handler for the Claude SubagentStart.
func (r *Registrar) OnSubagentStart(fn func(*SubagentStartEvent) *SubagentStartResponse) {
	r.backend.Register("claude", "SubagentStart", wrapSubagentStart(fn))
}

// OnSubagentStop registers a handler for the Claude SubagentStop.
func (r *Registrar) OnSubagentStop(fn func(*SubagentStopEvent) *SubagentStopResponse) {
	r.backend.Register("claude", "SubagentStop", wrapSubagentStop(fn))
}

// OnTaskCompleted registers a handler for the Claude TaskCompleted.
func (r *Registrar) OnTaskCompleted(fn func(*TaskCompletedEvent) *TaskCompletedResponse) {
	r.backend.Register("claude", "TaskCompleted", wrapTaskCompleted(fn))
}

// OnTeammateIdle registers a handler for the Claude TeammateIdle.
func (r *Registrar) OnTeammateIdle(fn func(*TeammateIdleEvent) *TeammateIdleResponse) {
	r.backend.Register("claude", "TeammateIdle", wrapTeammateIdle(fn))
}

// OnUserPromptSubmit registers a handler for the Claude UserPromptSubmit.
func (r *Registrar) OnUserPromptSubmit(fn func(*UserPromptEvent) *UserPromptResponse) {
	r.backend.Register("claude", "UserPromptSubmit", wrapUserPromptSubmit(fn))
}

// OnWorktreeCreate registers a handler for the Claude WorktreeCreate.
func (r *Registrar) OnWorktreeCreate(fn func(*WorktreeCreateEvent) *WorktreeCreateResponse) {
	r.backend.Register("claude", "WorktreeCreate", wrapWorktreeCreate(fn))
}

// OnWorktreeRemove registers a handler for the Claude WorktreeRemove.
func (r *Registrar) OnWorktreeRemove(fn func(*WorktreeRemoveEvent) *WorktreeRemoveResponse) {
	r.backend.Register("claude", "WorktreeRemove", wrapWorktreeRemove(fn))
}
