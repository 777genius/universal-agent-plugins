package vscodelocal

import (
	internalvscodelocal "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/vscodelocal"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

// CommonEvent contains Local's native common fields (public-beta). Timestamp is
// the native ISO 8601 string, without parsing or normalization. SessionID, CWD
// and TranscriptPath are optional; absence/null decodes to an empty string.
// Fields can contain private data. Consumers must validate and filter them
// before any effect; paths never authorize reads or identify a trusted profile.
type CommonEvent = internalvscodelocal.CommonInput

// StopEvent observes the current execution about to stop (public-beta).
// StopHookActive retains nil for absent/null and a pointer for explicit false
// or true. Missing data is not an explicit false. HookEventName retains the
// native selector: argv selects the callback, so consumers must check it.
// The event does not establish task success, completion, idle state or root
// session identity, and cannot see later hook decisions to continue execution.
type StopEvent = internalvscodelocal.StopInput

// SubagentStopEvent observes a subagent about to stop (public-beta). AgentID and
// AgentType are native strings, not a root/session inference. A custom-agent
// Stop hook can run as native SubagentStop; consumers must inspect the name.
type SubagentStopEvent = internalvscodelocal.SubagentStopInput

// StopResponse is the public-beta neutral observer subset. Nil and empty values
// encode exactly {}. It has no decision, reason, continuation or context fields.
type StopResponse struct{}

// SubagentStopResponse is the public-beta neutral subagent observer subset.
// Nil and empty values encode exactly {} and cannot influence model execution.
type SubagentStopResponse struct{}

func wrapStop(fn func(*StopEvent) *StopResponse) runtime.TypedHandler {
	return wrapObserver("Stop", fn)
}

func wrapSubagentStop(fn func(*SubagentStopEvent) *SubagentStopResponse) runtime.TypedHandler {
	return wrapObserver("SubagentStop", fn)
}

func wrapObserver[T any, R any](name string, fn func(*T) R) runtime.TypedHandler {
	return func(_ runtime.InvocationContext, v any) runtime.Handled {
		e, ok := v.(*T)
		if !ok {
			return runtime.Handled{Err: runtime.InternalHookTypeMismatch("VS Code Local " + name)}
		}
		fn(e)
		return runtime.Handled{Value: internalvscodelocal.ObserverOutcome{}}
	}
}
