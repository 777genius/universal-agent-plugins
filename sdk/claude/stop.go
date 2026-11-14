package claude

import (
	internalclaude "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/claude"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

// StopEvent is the Claude Code Stop hook input (decoded from stdin JSON).
// Field names follow Claude docs; wire uses snake_case via the platform adapter.
type StopEvent = internalclaude.StopInput

// Response is the hook outcome for Stop (see Stop decision control in Claude Code hooks reference).
type Response struct {
	allowStop     bool
	blockReason   string
	blockExitCode int
}

// Allow lets Claude stop (omit decision / empty JSON object on wire).
func Allow() *Response {
	return &Response{allowStop: true}
}

// Block prevents Claude from stopping; maps to {"decision":"block","reason":...} with exit 0.
func Block(reason string) *Response {
	return &Response{allowStop: false, blockReason: reason, blockExitCode: 0}
}

// Continue is the same as Block for Stop: keep the session running (decision block on wire).
func Continue(reason string) *Response {
	return Block(reason)
}

// BlockExit2 requests a non-zero exit without JSON stdout (alternate path per Claude Stop docs).
func BlockExit2(reason string) *Response {
	return &Response{allowStop: false, blockReason: reason, blockExitCode: 2}
}

// AllowStop reports whether the handler allows the session to stop.
func (r *Response) AllowStop() bool {
	if r == nil {
		return true
	}
	return r.allowStop
}

// BlockReason is set when AllowStop is false (JSON block path or exit-2 path).
func (r *Response) BlockReason() string {
	if r == nil {
		return ""
	}
	return r.blockReason
}

// OutcomeFromResponse maps a handler return value to a platform outcome (nil → allow).
func OutcomeFromResponse(r *Response) internalclaude.StopOutcome {
	if r == nil {
		return internalclaude.StopOutcome{AllowStop: true}
	}
	if r.allowStop {
		return internalclaude.StopOutcome{AllowStop: true}
	}
	return internalclaude.StopOutcome{
		AllowStop:     false,
		BlockReason:   r.blockReason,
		BlockExitCode: r.blockExitCode,
	}
}

func wrapStop(fn func(*StopEvent) *Response) runtime.TypedHandler {
	return wrapClaudeHandler("Stop", fn, func(r *Response) any {
		return OutcomeFromResponse(r)
	})
}

func internalclaudeTypeMismatch(name string) error {
	return runtime.InternalHookTypeMismatch(name)
}
