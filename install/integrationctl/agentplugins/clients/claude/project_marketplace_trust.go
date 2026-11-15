package claude

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Explicit native registration and noninteractive MCP health do not establish
// interactive workspace trust. Keep verification inert and use the existing
// confirmed-setup repair path when this persistent preference is absent.
func finishClaudeMarketplaceTrust(env clients.Env, request domain.ActivationRequest, outcome domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	trusted, err := claudeProjectTrustRecorded(env, request)
	if err != nil {
		return shared.FailedActivation(outcome, "inspect exact Claude Code project trust", err)
	}
	if !trusted {
		if request.VerifyOnly {
			outcome.Activation, outcome.Verification = domain.ActivationPrepared, domain.VerificationInstalled
			outcome.UserActions = shared.AppendUnique(outcome.UserActions, "the selected Claude Code project has no recorded workspace trust; retry its confirmed installation to finish exact project setup")
			return outcome, nil
		}
		changed, err := grantClaudeProjectTrust(env, request)
		if err != nil {
			return shared.FailedActivation(outcome, "retry exact Claude Code project trust", err)
		}
		if changed {
			outcome.NativeEffect = domain.NativeEffectCommitted
		}
	}
	outcome.Activation, outcome.Verification = domain.ActivationActive, domain.VerificationInstalled
	return outcome, nil
}

func claudeProjectTrustRecorded(env clients.Env, request domain.ActivationRequest) (bool, error) {
	project, err := claudeTrustableProject(request)
	if err != nil {
		return false, err
	}
	snapshot, err := env.NativeConfig.ReadExactFile(filepath.Join(request.Client.ConfigRoot, ".claude.json"))
	if err != nil || !snapshot.Exists {
		return false, err
	}
	if len(snapshot.Body) > 1<<20 {
		return false, fmt.Errorf("claude project trust document exceeds limit")
	}
	trusted, err := renderClaudeProjectTrust(snapshot.Body, project)
	if err != nil {
		return false, err
	}
	return bytes.Equal(snapshot.Body, trusted), nil
}
