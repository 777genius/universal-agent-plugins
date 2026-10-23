package gemini

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
)

// NativeRegistry selects actual workspace configuration without replacing the
// detected user profile or changing Gemini's native home environment.
func (*Adapter) NativeRegistry(in clients.PlanInput) (string, string) {
	if in.TargetContext.Scope == domain.ScopeProject {
		return filepath.Join(in.TargetContext.ProjectRoot, ".gemini"), in.Client.ExecutablePath
	}
	return in.Client.ConfigRoot, in.Client.ExecutablePath
}

func (*Adapter) ProjectTargetRoot(client domain.DetectedClient, mode domain.PackageMode, managedRoot string, target domain.TargetContext) (string, string, error) {
	if target.Scope != domain.ScopeProject {
		return "", "", fmt.Errorf("Gemini project layout requires project scope")
	}
	if _, err := geminiNativeRoot(client, target, ""); err != nil {
		return "", "", err
	}
	if err := pathpolicy.RequireContainedChild(target.ProjectRoot, managedRoot); err != nil {
		return "", "", fmt.Errorf("Gemini prepared package must stay in the selected project: %w", err)
	}
	return shared.ManagedTargetRoot(client, mode, managedRoot)
}

func geminiPlanRoot(client domain.DetectedClient, plan domain.DeliveryPlan) (string, error) {
	if (plan.Scope == domain.ScopeProject) != (plan.TargetContext.Scope == domain.ScopeProject) {
		return "", fmt.Errorf("Gemini plan scope differs from its target context")
	}
	return geminiNativeRoot(client, plan.TargetContext, plan.NativeRegistryRoot)
}

func geminiNativeRoot(client domain.DetectedClient, target domain.TargetContext, planned string) (string, error) {
	validated, err := domain.NewTargetContext(target.Scope, target.ProjectRoot, target.UserConfigRoot)
	if err != nil {
		return "", err
	}
	if validated.UserConfigRoot != "" && validated.UserConfigRoot != client.ConfigRoot {
		return "", fmt.Errorf("Gemini detected user profile differs from the selected target context")
	}
	root := client.ConfigRoot
	if validated.Scope == domain.ScopeProject {
		root = filepath.Join(validated.ProjectRoot, ".gemini")
	}
	if err := validateProfile(root, planned); err != nil {
		return "", err
	}
	return root, nil
}
