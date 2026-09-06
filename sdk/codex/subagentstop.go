package codex

import (
	internalcodex "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/codex"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

// SubagentStopEvent is the Codex SubagentStop hook input (decoded from
// stdin JSON). It carries the Stop fields plus subagent identity.
type SubagentStopEvent = internalcodex.SubagentStopInput

func wrapSubagentStop(fn func(*SubagentStopEvent) *Response) runtime.TypedHandler {
	return func(_ runtime.InvocationContext, v any) runtime.Handled {
		ev, ok := v.(*internalcodex.SubagentStopInput)
		if !ok {
			return runtime.Handled{Err: runtime.InternalHookTypeMismatch("codex SubagentStop")}
		}
		_ = fn(ev)
		return runtime.Handled{Value: internalcodex.SubagentStopOutcome{}}
	}
}
