package claude

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

func wrapSessionStart(fn func(*SessionStartEvent) *SessionStartResponse) runtime.TypedHandler {
	return wrapClaudeHandler("SessionStart", fn, func(r *SessionStartResponse) any {
		return contextOutcomeFromResponse(r)
	})
}

func wrapSessionEnd(fn func(*SessionEndEvent) *SessionEndResponse) runtime.TypedHandler {
	return wrapClaudeHandler("SessionEnd", fn, func(r *SessionEndResponse) any {
		return commonOutcomeFromResponse(r)
	})
}

func wrapNotification(fn func(*NotificationEvent) *NotificationResponse) runtime.TypedHandler {
	return wrapClaudeHandler("Notification", fn, func(r *NotificationResponse) any {
		return contextOutcomeFromResponse(r)
	})
}

func wrapPostToolUse(fn func(*PostToolUseEvent) *PostToolUseResponse) runtime.TypedHandler {
	return wrapClaudeHandler("PostToolUse", fn, func(r *PostToolUseResponse) any {
		return postToolUseOutcomeFromResponse(r)
	})
}

func wrapPostToolUseFailure(fn func(*PostToolUseFailureEvent) *PostToolUseFailureResponse) runtime.TypedHandler {
	return wrapClaudeHandler("PostToolUseFailure", fn, func(r *PostToolUseFailureResponse) any {
		return contextOutcomeFromResponse(r)
	})
}

func wrapPermissionRequest(fn func(*PermissionRequestEvent) *PermissionRequestResponse) runtime.TypedHandler {
	return wrapClaudeHandler("PermissionRequest", fn, func(r *PermissionRequestResponse) any {
		return permissionOutcomeFromResponse(r)
	})
}

func wrapSubagentStart(fn func(*SubagentStartEvent) *SubagentStartResponse) runtime.TypedHandler {
	return wrapClaudeHandler("SubagentStart", fn, func(r *SubagentStartResponse) any {
		return contextOutcomeFromResponse(r)
	})
}

func wrapSubagentStop(fn func(*SubagentStopEvent) *SubagentStopResponse) runtime.TypedHandler {
	return wrapClaudeHandler("SubagentStop", fn, func(r *SubagentStopResponse) any {
		return commonOutcomeFromResponse(r)
	})
}

func wrapPreCompact(fn func(*PreCompactEvent) *PreCompactResponse) runtime.TypedHandler {
	return wrapClaudeHandler("PreCompact", fn, func(r *PreCompactResponse) any {
		return commonOutcomeFromResponse(r)
	})
}

func wrapSetup(fn func(*SetupEvent) *SetupResponse) runtime.TypedHandler {
	return wrapClaudeHandler("Setup", fn, func(r *SetupResponse) any {
		return contextOutcomeFromResponse(r)
	})
}

func wrapTeammateIdle(fn func(*TeammateIdleEvent) *TeammateIdleResponse) runtime.TypedHandler {
	return wrapClaudeHandler("TeammateIdle", fn, func(r *TeammateIdleResponse) any {
		return commonOutcomeFromResponse(r)
	})
}

func wrapTaskCompleted(fn func(*TaskCompletedEvent) *TaskCompletedResponse) runtime.TypedHandler {
	return wrapClaudeHandler("TaskCompleted", fn, func(r *TaskCompletedResponse) any {
		return commonOutcomeFromResponse(r)
	})
}

func wrapConfigChange(fn func(*ConfigChangeEvent) *ConfigChangeResponse) runtime.TypedHandler {
	return wrapClaudeHandler("ConfigChange", fn, func(r *ConfigChangeResponse) any {
		return commonOutcomeFromResponse(r)
	})
}

func wrapWorktreeCreate(fn func(*WorktreeCreateEvent) *WorktreeCreateResponse) runtime.TypedHandler {
	return wrapClaudeHandler("WorktreeCreate", fn, func(r *WorktreeCreateResponse) any {
		return commonOutcomeFromResponse(r)
	})
}

func wrapWorktreeRemove(fn func(*WorktreeRemoveEvent) *WorktreeRemoveResponse) runtime.TypedHandler {
	return wrapClaudeHandler("WorktreeRemove", fn, func(r *WorktreeRemoveResponse) any {
		return commonOutcomeFromResponse(r)
	})
}
