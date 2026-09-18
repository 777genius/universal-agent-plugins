package all

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/contracttest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// TestDefaultRegistryCoversEveryDefinedClient is the parity check between the
// declarative registry in domain and the adapters compiled into this package.
// Adding a client to domain without an adapter would silently drop it from
// detection; registering an adapter twice is caught by NewRegistry, which
// Default turns into a panic.
func TestDefaultRegistryCoversEveryDefinedClient(t *testing.T) {
	t.Parallel()
	registered := map[domain.ClientID]int{}
	for _, adapter := range Default().All() {
		registered[adapter.ID()]++
	}
	for _, definition := range domain.ClientDefinitions() {
		switch registered[definition.ID] {
		case 1:
		case 0:
			t.Errorf("client %q is defined in domain but has no adapter in clients/all", definition.ID)
		default:
			t.Errorf("client %q has %d adapters in clients/all, want exactly one", definition.ID, registered[definition.ID])
		}
		delete(registered, definition.ID)
	}
	for id := range registered {
		t.Errorf("client %q has an adapter but domain.ClientDefinitions does not define it", id)
	}
}

// TestDefaultRegistryAdaptersSatisfyTheContract runs the shared harness against
// every adapter, which is what keeps clients.As[T] honest: a capability is
// found by type assertion, so a renamed method turns it off without a
// compilation error.
func TestDefaultRegistryAdaptersSatisfyTheContract(t *testing.T) {
	t.Parallel()
	for _, adapter := range Default().All() {
		t.Run(string(adapter.ID()), func(t *testing.T) {
			t.Parallel()
			contracttest.RunHostDetector(t, adapter)
			// Unconditional: every client has something to say about a plan, so
			// an adapter that stops implementing PlanRefiner - a renamed method,
			// a changed signature - is a defect, not a client with nothing to
			// add. Asking "if it implements it" here would make the harness
			// silently skip exactly that case.
			contracttest.RunPlanRefiner(t, adapter)
		})
	}
	contracttest.RunTraitParity(t, Default(), everyClientDetectsSurfaces())
}

// everyClientDetectsSurfaces is the only capability every client declares so
// far: detection is not optional, because a client nobody can find cannot be
// installed into. The trait-driven requirements arrive with domain.ClientTraits.
func everyClientDetectsSurfaces() []contracttest.CapabilityRequirement {
	return []contracttest.CapabilityRequirement{{
		Name:  "HostDetector",
		Holds: func(domain.ClientDefinition) bool { return true },
		Implements: func(adapter clients.Adapter) bool {
			_, ok := adapter.(clients.HostDetector)
			return ok
		},
	}}
}
