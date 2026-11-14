package claude

import (
	internalclaude "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/claude"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

// UserPromptEvent is the Claude Code UserPromptSubmit hook input.
type UserPromptEvent = internalclaude.UserPromptSubmitInput

// UserPromptResponse is the hook outcome for UserPromptSubmit.
type UserPromptResponse struct {
	allow             bool
	blockReason       string
	blockExitCode     int
	additionalContext string
}

// UserPromptAllow lets the prompt proceed.
func UserPromptAllow() *UserPromptResponse {
	return &UserPromptResponse{allow: true}
}

// UserPromptAllowWithContext adds additionalContext on exit 0 (JSON).
func UserPromptAllowWithContext(context string) *UserPromptResponse {
	return &UserPromptResponse{allow: true, additionalContext: context}
}

// UserPromptBlock blocks the prompt with decision/reason (exit 0 + JSON).
func UserPromptBlock(reason string) *UserPromptResponse {
	return &UserPromptResponse{allow: false, blockReason: reason, blockExitCode: 0}
}

// UserPromptBlockExit2 rejects the prompt via exit 2; stderr to Claude.
func UserPromptBlockExit2(reason string) *UserPromptResponse {
	return &UserPromptResponse{allow: false, blockReason: reason, blockExitCode: 2}
}

// UserPromptOutcomeFromResponse maps handler return to platform outcome (nil → allow).
func UserPromptOutcomeFromResponse(r *UserPromptResponse) internalclaude.UserPromptSubmitOutcome {
	if r == nil {
		return internalclaude.UserPromptSubmitOutcome{Allow: true}
	}
	return internalclaude.UserPromptSubmitOutcome{
		Allow:             r.allow,
		BlockReason:       r.blockReason,
		BlockExitCode:     r.blockExitCode,
		AdditionalContext: r.additionalContext,
	}
}

func wrapUserPromptSubmit(fn func(*UserPromptEvent) *UserPromptResponse) runtime.TypedHandler {
	return wrapClaudeHandler("UserPromptSubmit", fn, func(r *UserPromptResponse) any {
		return UserPromptOutcomeFromResponse(r)
	})
}
