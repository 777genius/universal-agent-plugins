package contracttest

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type exampleAdapter struct{ id domain.ClientID }

func (a exampleAdapter) ID() domain.ClientID { return a.id }

type selectionAdapter struct{ exampleAdapter }

func (selectionAdapter) ManagedMCPSelection() clients.SelectionLayout {
	return clients.SelectionLayout{File: ".mcp.json"}
}

func TestRunAdapterAcceptsAKnownClient(t *testing.T) {
	RunAdapter(t, exampleAdapter{id: domain.ClientClaude})
}

func TestAdapterViolationsRejectAnUnusableIdentity(t *testing.T) {
	cases := map[string]clients.Adapter{
		"nil":        nil,
		"empty id":   exampleAdapter{},
		"unknown":    exampleAdapter{id: "not-a-client"},
		"whitespace": exampleAdapter{id: " claude"},
	}
	for name, adapter := range cases {
		if violations := adapterViolations(adapter); len(violations) == 0 {
			t.Errorf("adapterViolations accepted the %s adapter", name)
		}
	}
}

func TestTraitParityDetectsADeclaredButUnimplementedCapability(t *testing.T) {
	registry, err := clients.NewRegistry(
		selectionAdapter{exampleAdapter{id: domain.ClientClaude}},
		exampleAdapter{id: domain.ClientCursor},
	)
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}
	implementsSelection := func(adapter clients.Adapter) bool {
		_, ok := adapter.(clients.SelectionReader)
		return ok
	}

	// No requirements is the state the harness ships in until domain carries the
	// traits, and it has to stay a clean no-op.
	RunTraitParity(t, registry, nil)

	RunTraitParity(t, registry, []CapabilityRequirement{{
		Name:       "SelectionReader (claude only)",
		Holds:      func(definition domain.ClientDefinition) bool { return definition.ID == domain.ClientClaude },
		Implements: implementsSelection,
	}})

	violated := []CapabilityRequirement{{
		Name:       "SelectionReader (every client)",
		Holds:      func(domain.ClientDefinition) bool { return true },
		Implements: implementsSelection,
	}}
	if violations := traitParityViolations(registry, violated); len(violations) != 1 {
		t.Fatalf("traitParityViolations reported %d violations, want 1: %v", len(violations), violations)
	}

	incomplete := []CapabilityRequirement{{Name: "no predicates"}}
	if violations := traitParityViolations(registry, incomplete); len(violations) == 0 {
		t.Fatal("traitParityViolations accepted a requirement without predicates")
	}
}

// An unchecked error from NewRegistry leaves a nil registry, which iterates
// zero adapters. Parity has to report that instead of passing on nothing.
func TestTraitParityRejectsANilRegistry(t *testing.T) {
	if violations := traitParityViolations(nil, nil); len(violations) != 1 {
		t.Fatalf("traitParityViolations passed on a nil registry: %v", violations)
	}
}
