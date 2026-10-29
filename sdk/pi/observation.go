// Package pi provides typed, neutral Pi notification observations. Registration
// does not qualify installation, authenticate a frame, or authorize delivery.
package pi

import (
	"context"

	internalpi "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/pi"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

type ObservationEvent = internalpi.ObservationInput
type LoadedRevision = internalpi.LoadedRevision
type Settled = internalpi.Settled
type Prompt = internalpi.Prompt
type ObservationResponse struct{}

// DecodeObservation validates a standalone bounded frame without dispatch.
func DecodeObservation(body []byte) (*ObservationEvent, error) { return internalpi.Decode(body) }

type Registrar struct{ backend runtime.RegistrarBackend }

func NewRegistrar(backend runtime.RegistrarBackend) *Registrar { return &Registrar{backend: backend} }

// OnObservationContext registers a cancellable neutral observer. It replaces
// OnObservation and vice versa. Neither response can influence Pi execution.
func (r *Registrar) OnObservationContext(fn func(context.Context, *ObservationEvent) (*ObservationResponse, error)) {
	r.backend.Register("pi", "observation", wrapObservationContext(fn))
}
func wrapObservation(fn func(*ObservationEvent) *ObservationResponse) runtime.TypedHandler {
	return wrapObservationContext(func(_ context.Context, value *ObservationEvent) (*ObservationResponse, error) { return fn(value), nil })
}
func wrapObservationContext(fn func(context.Context, *ObservationEvent) (*ObservationResponse, error)) runtime.TypedHandler {
	return func(invocation runtime.InvocationContext, value any) runtime.Handled {
		observation, ok := value.(*ObservationEvent)
		if !ok {
			return runtime.Handled{Err: runtime.InternalHookTypeMismatch("Pi observation")}
		}
		if err := invocation.Context.Err(); err != nil {
			return runtime.Handled{Err: err}
		}
		_, err := fn(invocation.Context, observation)
		return runtime.Handled{Value: internalpi.ObservationOutcome{}, Err: err}
	}
}
