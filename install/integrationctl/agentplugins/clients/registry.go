package clients

import (
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// ErrRegistryRequired is what a dispatcher returns when it was constructed
// without a registry. A nil registry is never resolved to a default containing
// every client: that would compile all client adapters into any binary that
// imports a generic package, and would make the registry a global the
// composition root no longer controls.
var ErrRegistryRequired = errors.New("client registry is required")

// Registry resolves a client id to its adapter. It is built once in the
// composition root (and explicitly in tests) and injected into every generic
// dispatcher.
type Registry struct {
	adapters map[domain.ClientID]Adapter
}

// NewRegistry rejects a duplicate id and an id that domain does not define, so
// a typo cannot silently produce a registry that is missing a client.
func NewRegistry(adapters ...Adapter) (*Registry, error) {
	registry := &Registry{adapters: make(map[domain.ClientID]Adapter, len(adapters))}
	for _, adapter := range adapters {
		if adapter == nil {
			return nil, fmt.Errorf("client registry received a nil adapter")
		}
		id := adapter.ID()
		if !domain.IsSupportedClient(id) {
			return nil, fmt.Errorf("client registry received an unknown client %q", id)
		}
		if _, duplicate := registry.adapters[id]; duplicate {
			return nil, fmt.Errorf("client registry received a duplicate client %q", id)
		}
		registry.adapters[id] = adapter
	}
	return registry, nil
}

// Lookup reports the adapter registered for id.
func (r *Registry) Lookup(id domain.ClientID) (Adapter, bool) {
	if r == nil {
		return nil, false
	}
	adapter, ok := r.adapters[id]
	return adapter, ok
}

// All returns the registered adapters in domain.ClientDefinitions order, so any
// listing built on the registry keeps the stable user-facing order.
func (r *Registry) All() []Adapter {
	if r == nil {
		return nil
	}
	result := make([]Adapter, 0, len(r.adapters))
	for _, definition := range domain.ClientDefinitions() {
		if adapter, ok := r.adapters[definition.ID]; ok {
			result = append(result, adapter)
		}
	}
	return result
}

// As is the capability lookup: it reports the adapter for id when that adapter
// implements T. A client that does not implement the capability is the normal
// case, not an error - the dispatcher decides what the absence means.
func As[T any](r *Registry, id domain.ClientID) (T, bool) {
	var zero T
	adapter, ok := r.Lookup(id)
	if !ok {
		return zero, false
	}
	capability, ok := adapter.(T)
	if !ok {
		return zero, false
	}
	return capability, true
}
