package cursor

import (
	"context"

	internalcursor "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/cursor"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

// StopStatus is a native Cursor stop status (public-beta). Unknown strings are
// retained. Completed means an agent loop ended, not that a task succeeded.
type StopStatus = internalcursor.StopStatus

const (
	StopStatusCompleted StopStatus = internalcursor.StopStatusCompleted
	StopStatusAborted   StopStatus = internalcursor.StopStatusAborted
	StopStatusError     StopStatus = internalcursor.StopStatusError
)

// ModelParam is one documented string-valued native model parameter pair.
type ModelParam = internalcursor.ModelParam

// StopEvent is the documented native stop input (public-beta). Optional
// LoopCount, UserEmail and TranscriptPath retain nil versus explicit values.
// Fields may contain private content; consumers must filter before delivery.
// The codec admits only native stop, bounded nonempty IDs and a nonnegative
// 32-bit loop count. These admission bounds do not authenticate the sender.
// Missing/null/unknown Status remains unknown; consumers must suppress effects
// for statuses they do not understand.
type StopEvent = internalcursor.StopInput

// StopResponse is the neutral observer subset (public-beta). Nil and empty
// responses encode as {}. It cannot request continuation or decide permission.
type StopResponse struct{}

func wrapStop(fn func(*StopEvent) *StopResponse) runtime.TypedHandler {
	return wrapStopContext(func(_ context.Context, e *StopEvent) (*StopResponse, error) {
		return fn(e), nil
	})
}

func wrapStopContext(fn func(context.Context, *StopEvent) (*StopResponse, error)) runtime.TypedHandler {
	return func(ic runtime.InvocationContext, v any) runtime.Handled {
		e, ok := v.(*StopEvent)
		if !ok {
			return runtime.Handled{Err: runtime.InternalHookTypeMismatch("Cursor stop")}
		}
		if err := ic.Context.Err(); err != nil {
			return runtime.Handled{Err: err}
		}
		_, err := fn(ic.Context, e)
		return runtime.Handled{Value: internalcursor.StopOutcome{}, Err: err}
	}
}
