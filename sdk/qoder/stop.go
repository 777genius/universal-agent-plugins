package qoder

import (
	internalqoder "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/qoder"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

// StopEvent is the Qoder Stop hook input.
type StopEvent = internalqoder.StopInput

// StopResponse is the hook outcome for Stop.
type StopResponse struct {
	allowStop     bool
	blockReason   string
	blockExitCode int
}

// StopAllow lets the session stop (omit decision / empty JSON object on wire).
func StopAllow() *StopResponse {
	return &StopResponse{allowStop: true}
}

// StopBlock prevents the session from stopping; maps to
// {"decision":"block","reason":...} with exit 0.
func StopBlock(reason string) *StopResponse {
	return &StopResponse{allowStop: false, blockReason: reason, blockExitCode: 0}
}

// StopBlockExit2 requests a non-zero exit without JSON stdout (alternate
// block path in the Claude-style protocol).
func StopBlockExit2(reason string) *StopResponse {
	return &StopResponse{allowStop: false, blockReason: reason, blockExitCode: 2}
}

func stopOutcomeFromResponse(r *StopResponse) internalqoder.StopOutcome {
	if r == nil {
		return internalqoder.StopOutcome{AllowStop: true}
	}
	return internalqoder.StopOutcome{
		AllowStop:     r.allowStop,
		BlockReason:   r.blockReason,
		BlockExitCode: r.blockExitCode,
	}
}

func wrapStop(fn func(*StopEvent) *StopResponse) runtime.TypedHandler {
	return wrapQoderHandler("Stop", fn, func(r *StopResponse) any {
		return stopOutcomeFromResponse(r)
	})
}
