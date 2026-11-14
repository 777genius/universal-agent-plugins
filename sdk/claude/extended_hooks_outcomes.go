package claude

import internalclaude "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/claude"

func commonOutcomeFromResponse(r *CommonResponse) internalclaude.CommonOutcome {
	if r == nil {
		return internalclaude.CommonOutcome{}
	}
	return internalclaude.CommonOutcome{
		Continue:       r.Continue,
		SuppressOutput: r.SuppressOutput,
		StopReason:     r.StopReason,
		Decision:       r.Decision,
		Reason:         r.Reason,
		SystemMessage:  r.SystemMessage,
	}
}

func contextOutcomeFromResponse(r *ContextResponse) internalclaude.ContextOutcome {
	if r == nil {
		return internalclaude.ContextOutcome{}
	}
	return internalclaude.ContextOutcome{
		CommonOutcome:     commonOutcomeFromResponse(&r.CommonResponse),
		AdditionalContext: r.AdditionalContext,
	}
}

func postToolUseOutcomeFromResponse(r *PostToolUseResponse) internalclaude.PostToolUseOutcome {
	if r == nil {
		return internalclaude.PostToolUseOutcome{}
	}
	return internalclaude.PostToolUseOutcome{
		CommonOutcome:        commonOutcomeFromResponse(&r.CommonResponse),
		AdditionalContext:    r.AdditionalContext,
		UpdatedMCPToolOutput: r.UpdatedMCPToolOutput,
	}
}

func permissionOutcomeFromResponse(r *PermissionRequestResponse) internalclaude.PermissionRequestOutcome {
	if r == nil {
		return internalclaude.PermissionRequestOutcome{}
	}
	out := internalclaude.PermissionRequestOutcome{
		CommonOutcome: commonOutcomeFromResponse(&r.CommonResponse),
	}
	if r.Permission != nil {
		out.Permission = &internalclaude.PermissionDecision{
			Behavior:           internalclaude.PermissionBehavior(r.Permission.Behavior),
			UpdatedInput:       r.Permission.UpdatedInput,
			UpdatedPermissions: r.Permission.UpdatedPermissions,
			Message:            r.Permission.Message,
			Interrupt:          r.Permission.Interrupt,
		}
	}
	return out
}
