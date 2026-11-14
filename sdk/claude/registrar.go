package claude

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

// Registrar registers public Claude hook handlers on a root SDK app.
type Registrar struct {
	backend runtime.RegistrarBackend
}

// NewRegistrar builds a Claude registrar on top of the shared runtime backend.
func NewRegistrar(backend runtime.RegistrarBackend) *Registrar {
	return &Registrar{backend: backend}
}
