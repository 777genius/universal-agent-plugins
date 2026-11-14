package vscodelocal

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

// Registrar registers beta Local observers on the existing shared engine.
type Registrar struct{ backend runtime.RegistrarBackend }

// NewRegistrar builds a registrar using the shared runtime backend.
func NewRegistrar(backend runtime.RegistrarBackend) *Registrar {
	return &Registrar{backend: backend}
}
