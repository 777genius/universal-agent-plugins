package codex

import (
	internalcodex "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/codex"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

// StopEvent is the Codex Stop lifecycle hook input (decoded from stdin JSON).
// Field names follow the Codex hooks schema; wire uses snake_case via the
// platform adapter.
type StopEvent = internalcodex.StopInput

func wrapStop(fn func(*StopEvent) *Response) runtime.TypedHandler {
	return func(_ runtime.InvocationContext, v any) runtime.Handled {
		ev, ok := v.(*internalcodex.StopInput)
		if !ok {
			return runtime.Handled{Err: runtime.InternalHookTypeMismatch("codex Stop")}
		}
		_ = fn(ev)
		return runtime.Handled{Value: internalcodex.StopOutcome{}}
	}
}
