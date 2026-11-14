package cursor

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.ProjectTargetLayout  = (*Adapter)(nil)
	_ clients.NativeRegistryLayout = (*Adapter)(nil)
	_ clients.PlanPrecondition     = (*Adapter)(nil)
)

// NativeRegistry keeps the detected user profile distinct from workspace files.
func (*Adapter) NativeRegistry(in clients.PlanInput) (string, string) {
	if in.TargetContext.Scope == domain.ScopeProject {
		return filepath.Join(in.TargetContext.ProjectRoot, ".cursor"), ""
	}
	return in.Client.ConfigRoot, in.Client.ExecutablePath
}

func (*Adapter) ProjectTargetRoot(client domain.DetectedClient, mode domain.PackageMode, managedRoot string, target domain.TargetContext) (string, string, error) {
	if target.Scope != domain.ScopeProject {
		return "", "", fmt.Errorf("cursor project layout requires project scope")
	}
	if _, err := cursorNativeRoot(client, target, ""); err != nil {
		return "", "", err
	}
	if err := pathpolicy.RequireContainedChild(target.ProjectRoot, managedRoot); err != nil {
		return "", "", err
	}
	return shared.ManagedTargetRoot(client, mode, managedRoot)
}

func (*Adapter) CheckPlanPrecondition(in clients.PlanInput, plan *domain.DeliveryPlan) error {
	if in.TargetContext.Scope != domain.ScopeProject {
		return nil
	}
	_, err := cursorNativeRoot(in.Client, in.TargetContext, plan.NativeRegistryRoot)
	return err
}

func cursorPlanRoot(client domain.DetectedClient, plan domain.DeliveryPlan) (string, error) {
	if plan.Scope != domain.ScopeProject || plan.TargetContext.Scope != domain.ScopeProject {
		return "", fmt.Errorf("cursor component lifecycle requires matching project scope")
	}
	return cursorNativeRoot(client, plan.TargetContext, plan.NativeRegistryRoot)
}

func cursorNativeRoot(client domain.DetectedClient, target domain.TargetContext, planned string) (string, error) {
	validated, err := domain.NewTargetContext(target.Scope, target.ProjectRoot, target.UserConfigRoot)
	if err != nil {
		return "", err
	}
	if validated.Scope != domain.ScopeProject {
		return "", fmt.Errorf("cursor native components require project scope")
	}
	if validated.UserConfigRoot != "" && validated.UserConfigRoot != client.ConfigRoot {
		return "", fmt.Errorf("cursor detected user profile differs from selected target")
	}
	root := filepath.Join(validated.ProjectRoot, ".cursor")
	if planned != "" && planned != root {
		return "", fmt.Errorf("cursor native registry differs from selected project")
	}
	if err := pathpolicy.RequireContainedChild(validated.ProjectRoot, root); err != nil {
		return "", err
	}
	return root, nil
}
