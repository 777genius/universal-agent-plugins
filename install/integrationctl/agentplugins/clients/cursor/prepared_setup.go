package cursor

import (
	"path/filepath"
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.PreparedSetupCompleter = (*Adapter)(nil)

func (*Adapter) CanCompletePreparedSetup(request domain.ActivationRequest, outcome domain.ActivationOutcome) bool {
	if outcome.Activation != domain.ActivationPrepared || outcome.Verification != domain.VerificationInstalled || outcome.Policy != domain.PolicyAllowed || request.Plan.Scope != domain.ScopeProject || request.Plan.TargetContext.Scope != domain.ScopeProject || request.Plan.InstallIntent != domain.InstallIntentAutomatic {
		return false
	}
	root := request.Client.ConfigRoot
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Base(root) != ".cursor" {
		return false
	}
	project := request.Plan.TargetContext.ProjectRoot
	if !filepath.IsAbs(project) || filepath.Clean(project) != project || filepath.Dir(project) == project || request.Plan.NativeRegistryRoot != filepath.Join(project, ".cursor") {
		return false
	}
	// Only a fresh read-only observation of pending approval admits completion.
	// Skills+MCP packages stay Prepared after approval (MCP never qualifies Skill
	// discovery), and refused policy or profiles cannot be fixed by repetition:
	// both would otherwise re-run native setup on every add.
	if names, _ := cursorSelectedMCP(request.Plan.Components); len(names) == 0 || !slices.Contains(outcome.UserActions, cursorMCPApprovalPendingAction) {
		return false
	}
	for _, surface := range request.Client.Surfaces {
		if surface.ID == domain.CursorAgentSurfaceID && surface.Detected {
			return true
		}
	}
	return false
}
