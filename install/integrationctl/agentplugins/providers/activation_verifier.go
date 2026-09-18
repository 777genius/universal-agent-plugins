package providers

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// VerifierAvailable reports whether this backend can observe the delivered plan
// with an exact client-side verifier. CLI-registry clients answer through the
// adapter; native-config clients and Kiro stay in the legacy switch until 7b/7c.
func (activator Activator) VerifierAvailable(client domain.DetectedClient, plan domain.DeliveryPlan, backendExecutable string) bool {
	if activator.Registry != nil {
		if verifier, ok := clients.As[clients.ReadOnlyVerifier](activator.Registry, client.ClientID); ok {
			return verifier.VerifierAvailable(client, plan, backendExecutable)
		}
	}
	switch client.ClientID {
	case domain.ClientGemini, domain.ClientOpenCode, domain.ClientCline, domain.ClientWindsurf:
		// These clients expose an exact, read-only native configuration verifier.
		// It is intentionally independent of an optional client executable.
		return len(plan.Components) > 0
	}
	if strings.TrimSpace(backendExecutable) == "" {
		return false
	}
	if client.ClientID != domain.ClientKiro {
		return false
	}
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
}
