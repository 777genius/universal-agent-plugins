package copilot

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.ProjectTargetLayout     = (*Adapter)(nil)
	_ clients.NativeRegistryLayout    = (*Adapter)(nil)
	_ clients.PlanPrecondition        = (*Adapter)(nil)
	_ clients.ProfileBindingValidator = (*Adapter)(nil)
)

func (*Adapter) NativeRegistry(in clients.PlanInput) (string, string) {
	if in.TargetContext.Scope == domain.ScopeProject {
		return in.TargetContext.ProjectRoot, ""
	}
	return in.Client.ConfigRoot, in.Client.ExecutablePath
}

func (*Adapter) ProjectTargetRoot(client domain.DetectedClient, mode domain.PackageMode, managedRoot string, target domain.TargetContext) (string, string, error) {
	if _, err := copilotProjectRoot(client, target, ""); err != nil {
		return "", "", err
	}
	if err := pathpolicy.RequireContainedChild(target.ProjectRoot, managedRoot); err != nil {
		return "", "", err
	}
	return shared.ManagedTargetRoot(client, mode, managedRoot)
}

func (*Adapter) CheckPlanPrecondition(in clients.PlanInput, plan *domain.DeliveryPlan) error {
	if in.TargetContext.Scope == domain.ScopeProject || plan.Scope == domain.ScopeProject {
		_, err := copilotPlanRoot(in.Client, *plan)
		return err
	}
	return nil
}

func copilotPlanRoot(client domain.DetectedClient, plan domain.DeliveryPlan) (string, error) {
	if plan.Scope != domain.ScopeProject || plan.TargetContext.Scope != domain.ScopeProject {
		return "", fmt.Errorf("copilot project plan requires matching project scope")
	}
	return copilotProjectRoot(client, plan.TargetContext, plan.NativeRegistryRoot)
}

func copilotProjectRoot(client domain.DetectedClient, target domain.TargetContext, planned string) (string, error) {
	selected, err := domain.NewTargetContext(target.Scope, target.ProjectRoot, target.UserConfigRoot)
	if err != nil {
		return "", err
	}
	if selected.Scope != domain.ScopeProject || selected.UserConfigRoot != client.ConfigRoot {
		return "", fmt.Errorf("copilot project scope or original user profile differs from selected target")
	}
	if planned != "" && planned != selected.ProjectRoot {
		return "", fmt.Errorf("copilot native registry differs from selected project")
	}
	for _, relative := range []string{".mcp.json", filepath.Join(".github", "mcp.json"), filepath.Join(".github", "skills")} {
		if err := pathpolicy.RequireContainedChild(selected.ProjectRoot, filepath.Join(selected.ProjectRoot, relative)); err != nil {
			return "", err
		}
	}
	return selected.ProjectRoot, nil
}
