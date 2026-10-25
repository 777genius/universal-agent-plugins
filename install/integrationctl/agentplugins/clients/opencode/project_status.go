package opencode

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.ActivationPreflighter = (*Adapter)(nil)

// The provider calls this before its generic attestation shortcut. A native
// config receipt proves exact installed bytes, not the effective session config.
func (*Adapter) PreflightActivation(_ clients.Env, request domain.ActivationRequest) error {
	if request.Plan.Scope != domain.ScopeProject && request.Plan.TargetContext.Scope != domain.ScopeProject {
		return nil
	}
	if _, err := openCodeActivationLayout(request); err != nil {
		return err
	}
	if request.ActivationComplete {
		return fmt.Errorf("opencode project discovery cannot be established by activation attestation; inspect effective config, skills and MCP servers in a fresh session in the selected project")
	}
	return nil
}

func openCodeProjectStatus(request domain.ActivationRequest, outcome domain.ActivationOutcome) domain.ActivationOutcome {
	if request.Plan.Scope != domain.ScopeProject || outcome.Verification != domain.VerificationInstalled {
		return outcome
	}
	outcome.Activation = domain.ActivationPrepared
	outcome.ActivationAttested = false
	outcome.UserActions = shared.AppendUnique(outcome.UserActions, "OpenCode project configuration is installed; effective config, native skill discovery, permissions and MCP initialization remain unobserved; start OpenCode in the selected project and inspect debug config, debug skill and mcp list")
	outcome.UserActions = shared.AppendUnique(outcome.UserActions, "OpenCode merges ancestor, user, custom-directory, inline and managed configuration; preserved higher-priority settings or same-name skills can shadow the managed project components")
	return outcome
}
