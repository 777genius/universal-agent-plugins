package cline

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.ProjectTargetLayout  = (*Adapter)(nil)
	_ clients.NativeRegistryLayout = (*Adapter)(nil)
)

func clineProjectSelected(plan domain.DeliveryPlan) bool {
	return plan.Scope == domain.ScopeProject || plan.TargetContext.Scope == domain.ScopeProject || plan.TargetContext.ProjectRoot != ""
}

func (*Adapter) NativeRegistry(in clients.PlanInput) (string, string) {
	if in.TargetContext.Scope == domain.ScopeProject || in.TargetContext.ProjectRoot != "" {
		return in.TargetContext.ProjectRoot, ""
	}
	return in.Client.ConfigRoot, ""
}

func (*Adapter) ProjectTargetRoot(client domain.DetectedClient, mode domain.PackageMode, managed string, target domain.TargetContext) (string, string, error) {
	root, err := clineProjectRoot(client, target, "")
	if err != nil {
		return "", "", err
	}
	if managed != filepath.Join(root, ".uap", "managed") {
		return "", "", fmt.Errorf("Cline project managed root differs from selected project")
	}
	return shared.ManagedTargetRoot(client, mode, managed)
}

func clineProjectRoot(client domain.DetectedClient, target domain.TargetContext, planned string) (string, error) {
	selected, err := domain.NewTargetContext(target.Scope, target.ProjectRoot, target.UserConfigRoot)
	if err != nil {
		return "", err
	}
	if client.ClientID != domain.ClientCline || selected.Scope != domain.ScopeProject || selected.UserConfigRoot != client.ConfigRoot || client.ConfigRoot == "" {
		return "", fmt.Errorf("Cline project requires matching scope and original user profile")
	}
	root := selected.ProjectRoot
	if clineRootsOverlap(root, client.ConfigRoot) {
		return "", fmt.Errorf("Cline project must not overlap the original user profile")
	}
	if err := pathpolicy.RequireContainedChild(filepath.Dir(client.ConfigRoot), client.ConfigRoot); err != nil {
		return "", err
	}
	if planned != "" && planned != root {
		return "", fmt.Errorf("Cline project namespace differs from selected project")
	}
	for _, child := range []string{filepath.Join(root, ".cline", "skills"), filepath.Join(root, ".uap", "clients", "cline", "mcp.json")} {
		if err := pathpolicy.RequireContainedChild(root, child); err != nil {
			return "", err
		}
	}
	return root, nil
}

func clineProjectPlanRoot(client domain.DetectedClient, plan domain.DeliveryPlan) (string, error) {
	if plan.ClientID != domain.ClientCline || plan.Scope != domain.ScopeProject || plan.TargetContext.Scope != domain.ScopeProject || plan.NativeRegistryExecutable != "" {
		return "", fmt.Errorf("Cline project requires exact client, scope and file-only namespace")
	}
	root, err := clineProjectRoot(client, plan.TargetContext, plan.NativeRegistryRoot)
	if err != nil {
		return "", err
	}
	if plan.NativeRegistryRoot != root {
		return "", fmt.Errorf("Cline project namespace is missing")
	}
	if err := pathpolicy.ValidateLeafID(plan.PhysicalArtifactID); err != nil {
		return "", err
	}
	managed := filepath.Join(root, ".uap", "managed")
	target := filepath.Join(managed, "clients", "cline")
	if plan.TargetAnchor != managed || plan.TargetRoot != target || plan.ActivePath != filepath.Join(target, plan.PhysicalArtifactID) {
		return "", fmt.Errorf("Cline managed package differs from selected project")
	}
	return root, pathpolicy.RequireContainedChild(root, plan.ActivePath)
}

func refineClineProjectPlan(in clients.PlanInput, plan *domain.DeliveryPlan) error {
	root, err := clineProjectPlanRoot(in.Client, *plan)
	if err != nil {
		return err
	}
	if plan.Status == domain.PlanUnsupported || !shared.OnlyNativeComponents(plan.Components) {
		return nil
	}
	shared.PromoteNativeReady(plan, root, true)
	plan.Activation, plan.Verification = domain.ActivationPrepared, domain.VerificationPackageValid
	plan.UserActions = shared.AppendUnique(plan.UserActions, clineProjectAction(*plan))
	return nil
}

func clineRootsOverlap(first, second string) bool {
	for _, pair := range [][2]string{{first, second}, {second, first}} {
		relative, err := filepath.Rel(pair[0], pair[1])
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
