package claude

import (
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.PreparedSetupCompleter = (*Adapter)(nil)

// CanCompletePreparedSetup describes only the existing confirmed activation
// route. The core independently proves intact ownership before invoking it.
func (*Adapter) CanCompletePreparedSetup(request domain.ActivationRequest, outcome domain.ActivationOutcome) bool {
	if outcome.Activation != domain.ActivationPrepared || outcome.Verification != domain.VerificationInstalled || outcome.Policy != domain.PolicyAllowed || request.Plan.InstallIntent != domain.InstallIntentAutomatic || request.Plan.PackageMode != domain.PackageProjection || request.Plan.Status != domain.PlanReady || request.BackendExecutable == "" || request.DeclaredName == "" || claudeSetupRequiresOwnerChoice(outcome.UserActions) {
		return false
	}
	root := request.Client.ConfigRoot
	if !exactAbsolute(root) || !exactAbsolute(request.Plan.ActivePath) {
		return false
	}
	if request.Plan.Scope == domain.ScopeUser {
		return request.Plan.TargetContext.Scope == domain.ScopeUser && request.Plan.TargetAnchor == root && request.Plan.TargetRoot == filepath.Join(root, "skills") && filepath.Dir(request.Plan.ActivePath) == request.Plan.TargetRoot
	}
	if request.Plan.Scope != domain.ScopeProject || request.Plan.TargetContext.Scope != domain.ScopeProject {
		return false
	}
	return claudePreparedProjectLayout(request.Plan, root)
}

func claudePreparedProjectLayout(plan domain.DeliveryPlan, root string) bool {
	project := plan.TargetContext.ProjectRoot
	anchor := filepath.Join(project, ".claude")
	return exactAbsolute(project) && filepath.Dir(project) != project && plan.TargetContext.UserConfigRoot == root && plan.NativeRegistryRoot == anchor && plan.TargetAnchor == anchor && plan.TargetRoot == filepath.Join(anchor, "skills") && filepath.Dir(plan.ActivePath) == plan.TargetRoot
}

// These adapter-owned observations require an owner to change native policy
// or name precedence. Retrying activation cannot complete them and must not
// create a native mutation attempt for an otherwise unchanged installation.
func claudeSetupRequiresOwnerChoice(actions []string) bool {
	for _, action := range actions {
		for _, prefix := range []string{
			"the owned Claude Code project plugin is disabled;",
			"the exact Claude Code project plugin is disabled;",
			"Claude Code lists a same-name user plugin instead of the selected project copy;",
			"Claude Code does not expose one unique enabled plugin named ",
		} {
			if strings.HasPrefix(action, prefix) {
				return true
			}
		}
	}
	return false
}
