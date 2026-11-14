// Package shared holds the helpers more than one client adapter needs. It is
// the DRY side of the clients contract: a predicate or a projection that two
// clients would otherwise copy lives here once, exported, and the client
// packages call it.
//
// It may reach for the path policy, the path contract, atomicfile and filetree,
// but never for a concrete client package, for clients/all, or upward into
// providers, planner or usecase.
package shared

import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"

// OnlyNativeComponents reports whether every supported component is a skill or
// an MCP server, and that at least one such component exists. It is the
// readiness predicate for clients that deliver exactly those two kinds
// natively: an empty selection is not "ready", and anything else in the
// selection means the client cannot claim the whole package.
func OnlyNativeComponents(components []domain.ComponentDecision) bool {
	found := false
	for _, component := range components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		if component.Kind != domain.ComponentSkill && component.Kind != domain.ComponentMCPServer {
			return false
		}
		found = true
	}
	return found
}

// ComponentKindPresent reports whether the selection contains a supported
// component of this kind.
func ComponentKindPresent(components []domain.ComponentDecision, kind domain.ComponentKind) bool {
	for _, component := range components {
		if component.Kind == kind && component.Support != domain.SupportUnsupported {
			return true
		}
	}
	return false
}

// HasSupportedMCP reports whether the selection contains a supported MCP
// server.
func HasSupportedMCP(components []domain.ComponentDecision) bool {
	return ComponentKindPresent(components, domain.ComponentMCPServer)
}

// SupportedMCPNames returns the MCP server names a delivery actually selected,
// in the order domain fixes for them.
func SupportedMCPNames(plan domain.DeliveryPlan) []string {
	return domain.SelectedMCPNames(plan)
}
