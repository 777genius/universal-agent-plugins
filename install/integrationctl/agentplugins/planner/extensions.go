package planner

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Extension support is qualified by the owning client for each namespace;
// declaring native extension capability does not authorize arbitrary semantics.
func interpretExtensionDecisions(envelope domain.PackageEnvelope, decisions []domain.ComponentDecision, interpreter clients.ExtensionInterpreter) {
	if interpreter == nil {
		return
	}
	for i := range decisions {
		if decisions[i].Kind != domain.ComponentExtension {
			continue
		}
		decisions[i].Support, decisions[i].Reason = domain.SupportUnsupported, "extension_semantics_not_established"
		if interpreter != nil {
			decisions[i].Support, decisions[i].Reason = interpreter.InterpretExtension(decisions[i].Name, envelope.Manifest.Extensions[decisions[i].Name])
		}
	}
}
