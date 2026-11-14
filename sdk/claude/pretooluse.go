package claude

import (
	internalclaude "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/claude"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

// PreToolUseEvent is the Claude Code PreToolUse hook input (stdin JSON).
type PreToolUseEvent = internalclaude.PreToolUseInput

// PreToolResponse is the hook outcome for PreToolUse (hookSpecificOutput on wire).
type PreToolResponse struct {
	permission    string
	reason        string
	blockExitCode int
	blockReason   string
}

// PreToolAllow lets the tool run (exit 0 + "{}" or optional reason for allow path).
func PreToolAllow() *PreToolResponse {
	return &PreToolResponse{permission: "allow"}
}

// PreToolAllowWithReason returns allow with permissionDecisionReason (shown to user).
func PreToolAllowWithReason(reason string) *PreToolResponse {
	return &PreToolResponse{permission: "allow", reason: reason}
}

// PreToolDeny blocks the tool with hookSpecificOutput deny.
func PreToolDeny(reason string) *PreToolResponse {
	return &PreToolResponse{permission: "deny", reason: reason}
}

// PreToolAsk prompts the user to confirm.
func PreToolAsk(reason string) *PreToolResponse {
	return &PreToolResponse{permission: "ask", reason: reason}
}

// PreToolBlockExit2 blocks the tool via exit 2; stderr carries reason to Claude.
func PreToolBlockExit2(reason string) *PreToolResponse {
	return &PreToolResponse{blockExitCode: 2, blockReason: reason}
}

// PreToolOutcomeFromResponse maps handler return to platform outcome (nil → allow).
func PreToolOutcomeFromResponse(r *PreToolResponse) internalclaude.PreToolUseOutcome {
	if r == nil {
		return internalclaude.PreToolUseOutcome{}
	}
	if r.blockExitCode != 0 {
		return internalclaude.PreToolUseOutcome{
			BlockExitCode: r.blockExitCode,
			BlockReason:   r.blockReason,
		}
	}
	return internalclaude.PreToolUseOutcome{
		Permission: r.permission,
		Reason:     r.reason,
	}
}

func wrapPreToolUse(fn func(*PreToolUseEvent) *PreToolResponse) runtime.TypedHandler {
	return wrapClaudeHandler("PreToolUse", fn, func(r *PreToolResponse) any {
		return PreToolOutcomeFromResponse(r)
	})
}
