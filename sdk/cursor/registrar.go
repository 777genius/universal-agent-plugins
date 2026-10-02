package cursor

import (
	"context"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

// Registrar registers beta Cursor stop handlers on the shared SDK engine.
type Registrar struct{ backend runtime.RegistrarBackend }

// NewRegistrar builds a Cursor registrar using the shared runtime backend.
func NewRegistrar(backend runtime.RegistrarBackend) *Registrar { return &Registrar{backend: backend} }

// OnStopContext registers a cancellable beta stop observer. It replaces a prior
// OnStop registration and vice versa. Callbacks must honor ctx, including any
// IO they perform, and must not write process stdout. Returning an error leaves
// the public RunCursorObserver response neutral; RunContext retains errors.
func (r *Registrar) OnStopContext(fn func(context.Context, *StopEvent) (*StopResponse, error)) {
	r.backend.Register("cursor", "stop", wrapStopContext(fn))
}
