package qoder

import (
	internalqoder "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/qoder"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

// UserPromptSubmitEvent is the Qoder UserPromptSubmit hook input.
type UserPromptSubmitEvent = internalqoder.UserPromptSubmitInput

// UserPromptSubmitResponse is the hook outcome for UserPromptSubmit.
type UserPromptSubmitResponse struct {
	allow             bool
	blockReason       string
	blockExitCode     int
	additionalContext string
}

// UserPromptSubmitAllow lets the prompt proceed.
func UserPromptSubmitAllow() *UserPromptSubmitResponse {
	return &UserPromptSubmitResponse{allow: true}
}

// UserPromptSubmitAllowWithContext adds additionalContext on exit 0 (JSON).
func UserPromptSubmitAllowWithContext(context string) *UserPromptSubmitResponse {
	return &UserPromptSubmitResponse{allow: true, additionalContext: context}
}

// UserPromptSubmitBlock blocks the prompt with decision/reason (exit 0 + JSON).
func UserPromptSubmitBlock(reason string) *UserPromptSubmitResponse {
	return &UserPromptSubmitResponse{allow: false, blockReason: reason, blockExitCode: 0}
}

// UserPromptSubmitBlockExit2 rejects the prompt via exit 2; stderr is surfaced.
func UserPromptSubmitBlockExit2(reason string) *UserPromptSubmitResponse {
	return &UserPromptSubmitResponse{allow: false, blockReason: reason, blockExitCode: 2}
}

func userPromptSubmitOutcomeFromResponse(r *UserPromptSubmitResponse) internalqoder.UserPromptSubmitOutcome {
	if r == nil {
		return internalqoder.UserPromptSubmitOutcome{Allow: true}
	}
	return internalqoder.UserPromptSubmitOutcome{
		Allow:             r.allow,
		BlockReason:       r.blockReason,
		BlockExitCode:     r.blockExitCode,
		AdditionalContext: r.additionalContext,
	}
}

func wrapUserPromptSubmit(fn func(*UserPromptSubmitEvent) *UserPromptSubmitResponse) runtime.TypedHandler {
	return wrapQoderHandler("UserPromptSubmit", fn, func(r *UserPromptSubmitResponse) any {
		return userPromptSubmitOutcomeFromResponse(r)
	})
}
