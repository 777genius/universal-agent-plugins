package claude

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.ProjectTargetLayout  = (*Adapter)(nil)
	_ clients.NativeRegistryLayout = (*Adapter)(nil)
)

func (*Adapter) NativeRegistry(in clients.PlanInput) (string, string) {
	if in.TargetContext.Scope == domain.ScopeProject {
		return filepath.Join(in.TargetContext.ProjectRoot, ".claude"), in.Client.ExecutablePath
	}
	return in.Client.ConfigRoot, in.Client.ExecutablePath
}

func (*Adapter) ProjectTargetRoot(client domain.DetectedClient, mode domain.PackageMode, _ string, target domain.TargetContext) (string, string, error) {
	if mode != domain.PackageProjection {
		return "", "", fmt.Errorf("claude project skills-directory installation requires projected delivery")
	}
	root, err := claudeProjectRoot(client, target, "")
	if err != nil {
		return "", "", err
	}
	return root, filepath.Join(root, "skills"), nil
}

func claudeProjectRoot(client domain.DetectedClient, target domain.TargetContext, planned string) (string, error) {
	validated, err := domain.NewTargetContext(target.Scope, target.ProjectRoot, target.UserConfigRoot)
	if err != nil {
		return "", err
	}
	if validated.Scope != domain.ScopeProject || validated.UserConfigRoot != client.ConfigRoot {
		return "", fmt.Errorf("claude project context differs from selected scope or original user configuration")
	}
	root := filepath.Join(validated.ProjectRoot, ".claude")
	if planned != "" && planned != root {
		return "", fmt.Errorf("claude native registry differs from selected project")
	}
	if err := pathpolicy.RequireContainedChild(validated.ProjectRoot, root); err != nil {
		return "", err
	}
	return root, nil
}

func claudePlanProjectRoot(client domain.DetectedClient, plan domain.DeliveryPlan) (string, error) {
	if plan.Scope != domain.ScopeProject || plan.TargetContext.Scope != domain.ScopeProject {
		return "", fmt.Errorf("claude project lifecycle requires matching project scope")
	}
	root, err := claudeProjectRoot(client, plan.TargetContext, plan.NativeRegistryRoot)
	if err != nil {
		return "", err
	}
	if plan.TargetAnchor != root || plan.TargetRoot != filepath.Join(root, "skills") || filepath.Dir(plan.ActivePath) != plan.TargetRoot {
		return "", fmt.Errorf("claude project delivery differs from exact skills-directory layout")
	}
	return root, nil
}

func claudeListingScope(scope domain.InstallScope) domain.InstallScope {
	if scope == domain.ScopeProject {
		return scope
	}
	return domain.ScopeUser
}
