// Package contracttest holds the executable contract every client adapter has
// to satisfy, in-tree and out-of-tree alike. It is the counterweight to
// clients.As[T]: a capability is discovered by type assertion, so a renamed or
// misspelled method makes an adapter silently stop implementing an interface,
// and nothing but a test notices.
//
// The harness grows with the refactor: RunAdapter covers identity today, and
// RunHostDetector, RunPlanRefiner and the rest arrive with the parts that move
// each capability into the adapters.
//
// Each Run* function is a thin reporter over a pure violations function, so the
// harness can prove that it rejects a bad adapter instead of only that it
// accepts a good one.
package contracttest

import (
	"fmt"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// RunAdapter asserts the mandatory part of the contract: the adapter reports a
// stable id that domain actually defines.
func RunAdapter(t *testing.T, adapter clients.Adapter) {
	t.Helper()
	for _, violation := range adapterViolations(adapter) {
		t.Error(violation)
	}
}

func adapterViolations(adapter clients.Adapter) []string {
	if adapter == nil {
		return []string{"adapter is nil"}
	}
	id := adapter.ID()
	if id == "" {
		return []string{"adapter reports an empty client id"}
	}
	violations := []string{}
	if _, ok := domain.ClientDefinitionFor(id); !ok {
		violations = append(violations, fmt.Sprintf("adapter reports client id %q, which domain.ClientDefinitions does not define", id))
	}
	if repeated := adapter.ID(); repeated != id {
		violations = append(violations, fmt.Sprintf("adapter reported client id %q and then %q; the id must be stable", id, repeated))
	}
	return violations
}
