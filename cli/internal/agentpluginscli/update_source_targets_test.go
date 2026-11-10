package agentpluginscli

import (
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Signed owner-only admission must not silently narrow pre-existing local
// source compatibility checks for the same shared physical backend.
func TestUpdateSourceTargetsKeepsLocalCopilotVSCodeSurface(t *testing.T) {
	installation := domain.Installation{
		Clients: []domain.ClientBinding{{
			ClientID:         string(domain.ClientCopilot),
			Scope:            string(domain.ScopeUser),
			Materialization:  "materialized",
			AffectedSurfaces: []string{"copilot", "vscode"},
		}},
	}
	check := func(label string, got, want []domain.ClientID) {
		t.Helper()
		sortTargets(got)
		sortTargets(want)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: target set = %v, want %v", label, got, want)
		}
	}
	selectedCopilot := []domain.ClientID{domain.ClientCopilot}
	check("local update", updateSourceTargets(installation, "user", selectedCopilot),
		[]domain.ClientID{domain.ClientCopilot, domain.ClientVSCode})
	check("local repair", repairSourceTargets(installation, installation.Clients[0],
		domain.ClientCopilot, domain.ScopeUser),
		[]domain.ClientID{domain.ClientCopilot, domain.ClientVSCode})

	installation.OriginMode = domain.OriginModeDirectory
	check("signed Copilot repair", repairSourceTargets(installation, installation.Clients[0],
		domain.ClientCopilot, domain.ScopeUser),
		[]domain.ClientID{domain.ClientCopilot})
	check("signed Copilot owner", updateSourceTargets(installation, "user", selectedCopilot),
		[]domain.ClientID{domain.ClientCopilot})
	check("explicit signed sibling", updateSourceTargets(installation, "user",
		[]domain.ClientID{domain.ClientCopilot, domain.ClientVSCode}),
		[]domain.ClientID{domain.ClientCopilot, domain.ClientVSCode})
}
