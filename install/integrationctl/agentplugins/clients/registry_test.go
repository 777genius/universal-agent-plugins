package clients

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type stubAdapter struct{ id domain.ClientID }

func (a stubAdapter) ID() domain.ClientID { return a.id }

type lifecycleAdapter struct{ stubAdapter }

func (lifecycleAdapter) ManagedMCPSelection() SelectionLayout {
	return SelectionLayout{File: ".mcp.json"}
}

func TestNewRegistryRejectsDuplicateAndUnknownClients(t *testing.T) {
	if _, err := NewRegistry(stubAdapter{id: domain.ClientClaude}, stubAdapter{id: domain.ClientClaude}); err == nil {
		t.Fatal("a duplicate client id was accepted")
	}
	if _, err := NewRegistry(stubAdapter{id: domain.ClientID("not-a-client")}); err == nil {
		t.Fatal("an id outside domain.ClientDefinitions was accepted")
	}
	if _, err := NewRegistry(nil); err == nil {
		t.Fatal("a nil adapter was accepted")
	}
}

func TestRegistryAllFollowsDomainOrder(t *testing.T) {
	registry, err := NewRegistry(
		stubAdapter{id: domain.ClientWindsurf},
		stubAdapter{id: domain.ClientCodex},
		stubAdapter{id: domain.ClientClaude},
	)
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}
	want := []domain.ClientID{}
	for _, definition := range domain.ClientDefinitions() {
		switch definition.ID {
		case domain.ClientWindsurf, domain.ClientCodex, domain.ClientClaude:
			want = append(want, definition.ID)
		}
	}
	got := registry.All()
	if len(got) != len(want) {
		t.Fatalf("All() returned %d adapters, want %d", len(got), len(want))
	}
	for index, adapter := range got {
		if adapter.ID() != want[index] {
			t.Fatalf("All()[%d] = %q, want %q", index, adapter.ID(), want[index])
		}
	}
}

func TestAsReportsOnlyImplementedCapabilities(t *testing.T) {
	registry, err := NewRegistry(
		lifecycleAdapter{stubAdapter{id: domain.ClientClaude}},
		stubAdapter{id: domain.ClientCursor},
	)
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}
	if _, ok := As[SelectionReader](registry, domain.ClientClaude); !ok {
		t.Fatal("an implemented capability was not reported")
	}
	if _, ok := As[SelectionReader](registry, domain.ClientCursor); ok {
		t.Fatal("an unimplemented capability was reported")
	}
	if _, ok := As[SelectionReader](registry, domain.ClientKiro); ok {
		t.Fatal("an unregistered client reported a capability")
	}
}

// A nil registry must stay inert rather than resolve to "every client": the
// dispatchers built on it fail with ErrRegistryRequired instead of silently
// dispatching.
func TestNilRegistryResolvesNothing(t *testing.T) {
	var registry *Registry
	if _, ok := registry.Lookup(domain.ClientClaude); ok {
		t.Fatal("a nil registry resolved a client")
	}
	if adapters := registry.All(); len(adapters) != 0 {
		t.Fatalf("a nil registry returned %d adapters", len(adapters))
	}
	if _, ok := As[SelectionReader](registry, domain.ClientClaude); ok {
		t.Fatal("a nil registry reported a capability")
	}
}
