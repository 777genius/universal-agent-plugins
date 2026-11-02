package grok

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.ProjectTargetLayout     = (*Adapter)(nil)
	_ clients.NativeRegistryLayout    = (*Adapter)(nil)
	_ clients.PlanPrecondition        = (*Adapter)(nil)
	_ clients.PlanQualifier           = (*Adapter)(nil)
	_ clients.ProfileBindingValidator = (*Adapter)(nil)
)

func (*Adapter) NativeRegistry(in clients.PlanInput) (string, string) {
	if in.TargetContext.Scope == domain.ScopeProject {
		return filepath.Join(in.TargetContext.ProjectRoot, ".grok"), in.Client.ExecutablePath
	}
	return in.Client.ConfigRoot, in.Client.ExecutablePath
}
func (*Adapter) ProjectTargetRoot(client domain.DetectedClient, mode domain.PackageMode, _ string, target domain.TargetContext) (string, string, error) {
	if mode != domain.PackageNative {
		return "", "", fmt.Errorf("grok project preparation requires native plugin-directory delivery")
	}
	root, err := grokProjectRoot(client, target, "")
	if err != nil {
		return "", "", err
	}
	return root, filepath.Join(root, "plugins"), nil
}
func grokProjectRoot(client domain.DetectedClient, target domain.TargetContext, planned string) (string, error) {
	for _, surface := range client.Surfaces {
		if surface.ID == "grok_custom_home" && surface.Detected {
			return "", fmt.Errorf("grok project preparation cannot exhaust Claude-compatible sources with custom GROK_HOME")
		}
	}
	selected, err := domain.NewTargetContext(target.Scope, target.ProjectRoot, target.UserConfigRoot)
	if err != nil {
		return "", err
	}
	if client.ClientID != domain.ClientGrok || selected.Scope != domain.ScopeProject || selected.UserConfigRoot != client.ConfigRoot || filepath.Base(client.ConfigRoot) != ".grok" {
		return "", fmt.Errorf("grok project requires matching scope and original ordinary .grok user profile; custom GROK_HOME compatibility sources are unqualified")
	}
	root := filepath.Join(selected.ProjectRoot, ".grok")
	if root == client.ConfigRoot || strings.HasPrefix(root, client.ConfigRoot+string(filepath.Separator)) || strings.HasPrefix(client.ConfigRoot, root+string(filepath.Separator)) {
		return "", fmt.Errorf("grok project registry must not overlap the original user profile")
	}
	if planned != "" && planned != root {
		return "", fmt.Errorf("grok native registry differs from selected project")
	}
	if err := pathpolicy.RequireContainedChild(selected.ProjectRoot, filepath.Join(root, "plugins")); err != nil {
		return "", err
	}
	return root, nil
}
func grokPlanRoot(client domain.DetectedClient, plan domain.DeliveryPlan) (string, error) {
	if plan.ClientID != domain.ClientGrok || plan.Scope != domain.ScopeProject || plan.TargetContext.Scope != domain.ScopeProject {
		return "", fmt.Errorf("grok project lifecycle requires matching client and project scope")
	}
	root, err := grokProjectRoot(client, plan.TargetContext, plan.NativeRegistryRoot)
	if err != nil {
		return "", err
	}
	if err := pathpolicy.ValidateLeafID(plan.DeclaredName); err != nil {
		return "", err
	}
	if err := pathpolicy.ValidateLeafID(plan.PhysicalArtifactID); err != nil {
		return "", err
	}
	if plan.TargetAnchor != root || plan.TargetRoot != filepath.Join(root, "plugins") || plan.ActivePath != filepath.Join(root, "plugins", plan.PhysicalArtifactID) {
		return "", fmt.Errorf("grok project artifact differs from exact selected plugin directory")
	}
	return root, pathpolicy.RequireContainedChild(root, plan.ActivePath)
}
func (*Adapter) CheckPlanPrecondition(in clients.PlanInput, plan *domain.DeliveryPlan) error {
	if plan.Scope != domain.ScopeProject && in.TargetContext.Scope != domain.ScopeProject && in.TargetContext.ProjectRoot == "" {
		return nil
	}
	if plan.Scope != domain.ScopeProject || in.TargetContext.Scope != domain.ScopeProject {
		return fmt.Errorf("grok project plan scope mismatch")
	}
	if _, err := grokProjectRoot(in.Client, in.TargetContext, plan.NativeRegistryRoot); err != nil {
		return err
	}
	if in.Client.ExecutablePath == "" {
		plan.Status = domain.PlanUnsupported
		plan.Warnings = append(plan.Warnings, "pinned_grok_inspect_required")
		plan.UserActions = append(plan.UserActions, "Grok Build 1.0.67 or 1.0.70 CLI is required for read-only project source checks")
	}
	return nil
}
func (*Adapter) QualifyPlan(in clients.PlanInput, plan *domain.DeliveryPlan) error {
	if plan.Scope != domain.ScopeProject && in.TargetContext.Scope != domain.ScopeProject && in.TargetContext.ProjectRoot == "" {
		return nil
	}
	if plan.Scope != domain.ScopeProject || in.TargetContext.Scope != domain.ScopeProject {
		return fmt.Errorf("grok project plan scope mismatch")
	}
	if _, err := grokProjectRoot(in.Client, in.TargetContext, plan.NativeRegistryRoot); err != nil {
		return err
	}
	for i := range plan.Components {
		if plan.Components[i].Kind == domain.ComponentMCPServer {
			plan.Components[i].Support = domain.SupportUnsupported
			plan.Components[i].Reason = "Grok project bundled MCP initialization and tools are unqualified at pinned 1.0.67 and 1.0.70"
		}
	}
	return nil
}
func (*Adapter) ValidateBindingProfile(root string, binding domain.ClientBinding) error {
	if binding.Scope != string(domain.ScopeProject) {
		return nil
	}
	if binding.ClientID != "grok" || binding.NativeProfileRoot != root || filepath.Base(root) != ".grok" {
		return fmt.Errorf("grok project binding native profile differs from selected project")
	}
	if err := pathpolicy.ValidateLeafID(binding.PhysicalArtifact); err != nil {
		return err
	}
	if binding.TargetLocator != filepath.Join(root, "plugins", binding.PhysicalArtifact) {
		return fmt.Errorf("grok project binding locator differs from physical artifact")
	}
	for _, object := range binding.NativeObjects {
		if object.Kind != "managed_package_directory" || object.Path != binding.TargetLocator {
			return fmt.Errorf("grok project binding contains foreign native ownership")
		}
	}
	return pathpolicy.RequireContainedChild(filepath.Dir(root), binding.TargetLocator)
}
