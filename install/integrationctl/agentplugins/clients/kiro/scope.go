package kiro

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

func (*Adapter) NativeRegistry(in clients.PlanInput) (string, string) {
	if in.TargetContext.Scope == domain.ScopeProject {
		return filepath.Join(in.TargetContext.ProjectRoot, ".kiro"), ""
	}
	return in.Client.ConfigRoot, in.Client.ExecutablePath
}

func (*Adapter) ProjectTargetRoot(client domain.DetectedClient, mode domain.PackageMode, managedRoot string, target domain.TargetContext) (string, string, error) {
	if _, err := kiroProjectRoot(client, target, ""); err != nil {
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
	_, err := kiroProjectRoot(in.Client, in.TargetContext, plan.NativeRegistryRoot)
	return err
}

func kiroProjectRoot(client domain.DetectedClient, target domain.TargetContext, planned string) (string, error) {
	validated, err := domain.NewTargetContext(target.Scope, target.ProjectRoot, target.UserConfigRoot)
	if err != nil {
		return "", err
	}
	if validated.Scope != domain.ScopeProject {
		return "", fmt.Errorf("kiro workspace configuration requires project scope")
	}
	if validated.UserConfigRoot != "" && validated.UserConfigRoot != client.ConfigRoot {
		return "", fmt.Errorf("kiro detected user profile differs from selected target")
	}
	root := filepath.Join(validated.ProjectRoot, ".kiro")
	if planned != "" && planned != root {
		return "", fmt.Errorf("kiro native registry differs from selected project")
	}
	return root, pathpolicy.RequireContainedChild(validated.ProjectRoot, root)
}

func kiroProjectPlanRoot(client domain.DetectedClient, plan domain.DeliveryPlan) (string, error) {
	if plan.Scope != domain.ScopeProject || plan.TargetContext.Scope != domain.ScopeProject {
		return "", fmt.Errorf("kiro project lifecycle requires matching project scope")
	}
	root, err := kiroProjectRoot(client, plan.TargetContext, plan.NativeRegistryRoot)
	if err != nil {
		return "", err
	}
	if err := pathpolicy.ValidateLeafID(plan.PhysicalArtifactID); err != nil {
		return "", err
	}
	expected := filepath.Join(plan.TargetContext.ProjectRoot, ".uap", "managed", "clients", "kiro", plan.PhysicalArtifactID)
	if plan.ActivePath != expected {
		return "", fmt.Errorf("kiro managed package differs from selected project")
	}
	return root, pathpolicy.RequireContainedChild(plan.TargetContext.ProjectRoot, expected)
}

func refineProjectPlan(in clients.PlanInput, plan *domain.DeliveryPlan) error {
	root, err := kiroProjectPlanRoot(in.Client, *plan)
	if err != nil {
		return err
	}
	plan.NativeRegistryRoot, plan.NativeRegistryExecutable = root, ""
	if plan.Status == domain.PlanUnsupported || !shared.OnlyNativeComponents(plan.Components) {
		return nil
	}
	plan.Status, plan.Activation, plan.Verification = domain.PlanReady, domain.ActivationPrepared, domain.VerificationPackageValid
	plan.UserActions = []string{KiroProjectAction}
	return nil
}
