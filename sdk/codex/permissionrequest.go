package codex

import (
	internalcodex "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/codex"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

// PermissionRequestEvent is the Codex PermissionRequest hook input (decoded
// from stdin JSON). ToolInput stays raw JSON for typed consumers.
type PermissionRequestEvent = internalcodex.PermissionRequestInput

func wrapPermissionRequest(fn func(*PermissionRequestEvent) *Response) runtime.TypedHandler {
	return func(_ runtime.InvocationContext, v any) runtime.Handled {
		ev, ok := v.(*internalcodex.PermissionRequestInput)
		if !ok {
			return runtime.Handled{Err: runtime.InternalHookTypeMismatch("codex PermissionRequest")}
		}
		_ = fn(ev)
		return runtime.Handled{Value: internalcodex.PermissionRequestOutcome{}}
	}
}
