package providers

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// VerifierAvailable reports whether this backend can observe the delivered plan
// with an exact client-side verifier. The switch is the one the use case used to
// carry; it moves into per-client adapters later in the refactor.
func (Activator) VerifierAvailable(client domain.DetectedClient, plan domain.DeliveryPlan, backendExecutable string) bool {
	switch client.ClientID {
	case domain.ClientGemini, domain.ClientOpenCode, domain.ClientCline, domain.ClientWindsurf:
		// These clients expose an exact, read-only native configuration verifier.
		// It is intentionally independent of an optional client executable.
		return len(plan.Components) > 0
	}
	if strings.TrimSpace(backendExecutable) == "" {
		return false
	}
	switch client.ClientID {
	case domain.ClientCodex, domain.ClientClaude, domain.ClientCopilot, domain.ClientVSCode:
		return true
	case domain.ClientKiro:
		if !strings.Contains(strings.ToLower(backendExecutable), "kiro") || len(plan.Components) == 0 {
			return false
		}
		for _, component := range plan.Components {
			if component.Support == domain.SupportUnsupported {
				continue
			}
			if component.Kind != domain.ComponentSkill && component.Kind != domain.ComponentMCPServer {
				return false
			}
		}
		return true
	default:
		return false
	}
}
