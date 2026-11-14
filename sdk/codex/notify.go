package codex

import (
	"encoding/json"

	internalcodex "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/codex"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

// NotifyEvent is the decoded Codex notify payload and its raw JSON form.
type NotifyEvent struct {
	// Raw keeps the original notify payload as it was received from argv JSON.
	Raw json.RawMessage
	// Client identifies the Codex client variant that emitted the event.
	Client string
}

// Response represents a successful acknowledgement of a Codex event
// (Notify and the observation-style lifecycle hooks Stop, SubagentStop,
// PreToolUse, and PermissionRequest).
type Response struct{}

// Continue acknowledges the event and exits successfully with empty output.
func Continue() *Response {
	return &Response{}
}

// RawJSON returns the original JSON payload for pass-through or custom decoding.
func (e *NotifyEvent) RawJSON() json.RawMessage {
	if e == nil {
		return nil
	}
	return e.Raw
}

func wrapNotify(fn func(*NotifyEvent) *Response) runtime.TypedHandler {
	return func(_ runtime.InvocationContext, v any) runtime.Handled {
		ev, ok := v.(*internalcodex.NotifyInput)
		if !ok {
			return runtime.Handled{Err: runtime.InternalHookTypeMismatch("codex Notify")}
		}
		_ = fn(&NotifyEvent{Raw: ev.Raw, Client: ev.Client})
		return runtime.Handled{Value: internalcodex.NotifyOutcome{}}
	}
}
