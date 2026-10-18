package qoder

import (
	internalqoder "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/qoder"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

// CommonResponse contains fields shared by Qoder's common hook response
// envelope (Claude-style wire: continue / suppressOutput / stopReason /
// decision / reason / systemMessage).
type CommonResponse struct {
	// Continue overrides the continue flag for compatible hooks when non-nil.
	Continue *bool
	// SuppressOutput omits the usual hook output message when true.
	SuppressOutput bool
	// StopReason explains why processing should stop.
	StopReason string
	// Decision carries the decision token such as approve or block.
	Decision string
	// Reason carries a human-readable reason for the decision.
	Reason string
	// SystemMessage injects a system message when supported.
	SystemMessage string
}

// ContextResponse extends CommonResponse with additional context text.
type ContextResponse struct {
	CommonResponse
	// AdditionalContext appends hook-specific context.
	AdditionalContext string
}

func commonOutcomeFromResponse(r *CommonResponse) internalqoder.CommonOutcome {
	if r == nil {
		return internalqoder.CommonOutcome{}
	}
	return internalqoder.CommonOutcome{
		Continue:       r.Continue,
		SuppressOutput: r.SuppressOutput,
		StopReason:     r.StopReason,
		Decision:       r.Decision,
		Reason:         r.Reason,
		SystemMessage:  r.SystemMessage,
	}
}

func contextOutcomeFromResponse(r *ContextResponse) internalqoder.SessionStartOutcome {
	if r == nil {
		return internalqoder.SessionStartOutcome{}
	}
	return internalqoder.SessionStartOutcome{
		CommonOutcome:     commonOutcomeFromResponse(&r.CommonResponse),
		AdditionalContext: r.AdditionalContext,
	}
}

func wrapQoderHandler[T any, R any](name string, fn func(*T) R, mapResponse func(R) any) runtime.TypedHandler {
	return func(_ runtime.InvocationContext, v any) runtime.Handled {
		ev, ok := v.(*T)
		if !ok {
			return runtime.Handled{Err: runtime.InternalHookTypeMismatch("qoder " + name)}
		}
		return runtime.Handled{Value: mapResponse(fn(ev))}
	}
}
