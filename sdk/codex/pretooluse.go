package codex

import (
	internalcodex "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/codex"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

// PreToolUseEvent is the Codex PreToolUse hook input (decoded from stdin
// JSON). ToolInput stays raw JSON for typed consumers.
type PreToolUseEvent = internalcodex.PreToolUseInput

func wrapPreToolUse(fn func(*PreToolUseEvent) *Response) runtime.TypedHandler {
	return func(_ runtime.InvocationContext, v any) runtime.Handled {
		ev, ok := v.(*internalcodex.PreToolUseInput)
		if !ok {
			return runtime.Handled{Err: runtime.InternalHookTypeMismatch("codex PreToolUse")}
		}
		_ = fn(ev)
		return runtime.Handled{Value: internalcodex.PreToolUseOutcome{}}
	}
}
