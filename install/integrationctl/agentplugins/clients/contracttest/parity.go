package contracttest

import (
	"fmt"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// CapabilityRequirement is one "declaring this trait promises implementing this
// interface" rule. Holds reads the declaration from the domain definition;
// Implements answers the same question about the adapter, normally with a type
// assertion such as:
//
//	func(a clients.Adapter) bool { _, ok := a.(clients.Lifecycle); return ok }
type CapabilityRequirement struct {
	// Name is how a failure names the trait, for example "LifecycleKind=native_config".
	Name       string
	Holds      func(definition domain.ClientDefinition) bool
	Implements func(adapter clients.Adapter) bool
}

// RunTraitParity fails for every registered adapter whose client declares a
// trait that the adapter does not implement. It closes the hole As[T] opens: a
// renamed method turns a capability off silently, and only a declaration
// checked against the implementation catches that.
//
// The requirement set is empty until domain carries the declarative client
// traits (Part 9 of the installer-core refactor), and calling this with no
// requirements is a valid no-op that keeps the harness wired up in the
// meantime. Parity is deliberately one-directional: declaring a trait requires
// the interface, while implementing an interface nobody declared yet is how a
// capability is normally introduced.
func RunTraitParity(t *testing.T, registry *clients.Registry, requirements []CapabilityRequirement) {
	t.Helper()
	for _, violation := range traitParityViolations(registry, requirements) {
		t.Error(violation)
	}
}

func traitParityViolations(registry *clients.Registry, requirements []CapabilityRequirement) []string {
	violations := []string{}
	for _, requirement := range requirements {
		if requirement.Holds == nil || requirement.Implements == nil {
			violations = append(violations, fmt.Sprintf("capability requirement %q is incomplete", requirement.Name))
		}
	}
	if len(violations) > 0 {
		return violations
	}
	for _, adapter := range registry.All() {
		definition, ok := domain.ClientDefinitionFor(adapter.ID())
		if !ok {
			violations = append(violations, fmt.Sprintf("registered adapter %q has no domain definition", adapter.ID()))
			continue
		}
		for _, requirement := range requirements {
			if requirement.Holds(definition) && !requirement.Implements(adapter) {
				violations = append(violations, fmt.Sprintf("client %q declares %s but its adapter does not implement the interface that trait promises", adapter.ID(), requirement.Name))
			}
		}
	}
	return violations
}
