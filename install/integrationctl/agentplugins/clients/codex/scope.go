package codex

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

func (*Adapter) NativeRegistry(in clients.PlanInput) (string, string) {
	if in.TargetContext.Scope == domain.ScopeProject {
		return in.TargetContext.ProjectRoot, in.Client.ExecutablePath
	}
	return in.Client.ConfigRoot, in.Client.ExecutablePath
}

func (*Adapter) ProjectTargetRoot(client domain.DetectedClient, mode domain.PackageMode, managedRoot string, target domain.TargetContext) (string, string, error) {
	if target.Scope != domain.ScopeProject {
		return "", "", fmt.Errorf("codex project layout requires project scope")
	}
	if _, err := codexProjectRoot(client, target, ""); err != nil {
		return "", "", err
	}
	if err := pathpolicy.RequireContainedChild(target.ProjectRoot, managedRoot); err != nil {
		return "", "", fmt.Errorf("codex prepared package escapes selected project: %w", err)
	}
	return shared.ManagedTargetRoot(client, mode, managedRoot)
}

func codexProjectRoot(client domain.DetectedClient, target domain.TargetContext, planned string) (string, error) {
	if target.Scope != domain.ScopeProject {
		return "", fmt.Errorf("codex component delivery requires project target context")
	}
	validated, err := domain.NewTargetContext(target.Scope, target.ProjectRoot, target.UserConfigRoot)
	if err != nil {
		return "", err
	}
	if validated.UserConfigRoot != client.ConfigRoot {
		return "", fmt.Errorf("codex original user profile differs from target context")
	}
	if err := validateProfile(validated.ProjectRoot, planned); err != nil {
		return "", err
	}
	for _, relative := range []string{".codex", filepath.Join(".agents", "skills")} {
		if err := pathpolicy.RequireContainedChild(validated.ProjectRoot, filepath.Join(validated.ProjectRoot, relative)); err != nil {
			return "", err
		}
	}
	return validated.ProjectRoot, nil
}

func codexProjectPlanRoot(client domain.DetectedClient, plan domain.DeliveryPlan) (string, error) {
	if plan.Scope != domain.ScopeProject {
		return "", fmt.Errorf("codex plan scope differs from project context")
	}
	return codexProjectRoot(client, plan.TargetContext, plan.NativeRegistryRoot)
}
