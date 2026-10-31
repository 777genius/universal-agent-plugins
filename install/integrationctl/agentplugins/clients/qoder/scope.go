package qoder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.ProjectTargetLayout     = (*Adapter)(nil)
	_ clients.NativeRegistryLayout    = (*Adapter)(nil)
	_ clients.PlanQualifier           = (*Adapter)(nil)
	_ clients.ProfileBindingValidator = (*Adapter)(nil)
)

func projectRequested(plan domain.DeliveryPlan) bool {
	return plan.Scope == domain.ScopeProject || plan.TargetContext.Scope == domain.ScopeProject || plan.TargetContext.ProjectRoot != ""
}

func (*Adapter) NativeRegistry(in clients.PlanInput) (string, string) {
	if in.TargetContext.Scope == domain.ScopeProject || in.TargetContext.ProjectRoot != "" {
		return filepath.Join(in.TargetContext.ProjectRoot, ".uap", "managed", "clients", "qoder"), ""
	}
	return in.Client.ConfigRoot, in.Client.ExecutablePath
}

func projectRoot(client domain.DetectedClient, target domain.TargetContext) (string, error) {
	selected, err := domain.NewTargetContext(target.Scope, target.ProjectRoot, target.UserConfigRoot)
	if err != nil {
		return "", err
	}
	if client.ClientID != domain.ClientQoder || selected.Scope != domain.ScopeProject || selected.UserConfigRoot == "" || selected.UserConfigRoot != client.ConfigRoot {
		return "", fmt.Errorf("qoder project requires explicit project scope and the original user profile")
	}
	canonical, err := filepath.EvalSymlinks(selected.ProjectRoot)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(selected.ProjectRoot)
	if err != nil {
		return "", err
	}
	if canonical != selected.ProjectRoot || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("qoder project root must remain an existing canonical directory")
	}
	root := filepath.Join(selected.ProjectRoot, ".uap", "managed", "clients", "qoder")
	if pathsOverlap(selected.ProjectRoot, client.ConfigRoot) {
		return "", fmt.Errorf("qoder project and original user profile must not overlap")
	}
	if err := pathpolicy.RequireContainedChild(selected.ProjectRoot, root); err != nil {
		return "", err
	}
	return root, nil
}

func pathsOverlap(left, right string) bool {
	return left == right || strings.HasPrefix(left, right+string(filepath.Separator)) || strings.HasPrefix(right, left+string(filepath.Separator))
}

func (*Adapter) ProjectTargetRoot(client domain.DetectedClient, mode domain.PackageMode, _ string, target domain.TargetContext) (string, string, error) {
	if mode != domain.PackageNative {
		return "", "", fmt.Errorf("qoder project preparation requires native package delivery")
	}
	root, err := projectRoot(client, target)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(target.ProjectRoot, ".uap", "managed"), root, nil
}

func validateProjectPlan(client domain.DetectedClient, plan domain.DeliveryPlan) error {
	if plan.ClientID != domain.ClientQoder || plan.Scope != domain.ScopeProject || plan.TargetContext.Scope != domain.ScopeProject || !plan.SelectedDelivery.IsZero() {
		return fmt.Errorf("qoder project lifecycle requires matching client, scope and delivery")
	}
	root, err := projectRoot(client, plan.TargetContext)
	if err != nil {
		return err
	}
	if err := pathpolicy.ValidateLeafID(plan.DeclaredName); err != nil {
		return err
	}
	if err := pathpolicy.ValidateLeafID(plan.PhysicalArtifactID); err != nil {
		return err
	}
	if plan.NativeRegistryRoot != root || plan.NativeRegistryExecutable != "" || plan.TargetAnchor != filepath.Join(plan.TargetContext.ProjectRoot, ".uap", "managed") || plan.TargetRoot != root || plan.ActivePath != filepath.Join(root, plan.PhysicalArtifactID) {
		return fmt.Errorf("qoder project artifact differs from the exact selected managed namespace")
	}
	return pathpolicy.RequireContainedChild(plan.TargetContext.ProjectRoot, plan.ActivePath)
}

func (*Adapter) QualifyPlan(in clients.PlanInput, plan *domain.DeliveryPlan) error {
	if !projectRequested(*plan) && in.TargetContext.ProjectRoot == "" && in.TargetContext.Scope != domain.ScopeProject {
		return nil
	}
	if plan.Scope != domain.ScopeProject || in.TargetContext.Scope != domain.ScopeProject {
		return fmt.Errorf("qoder project plan scope mismatch")
	}
	if _, err := projectRoot(in.Client, in.TargetContext); err != nil {
		return err
	}
	for i := range plan.Components {
		if plan.Components[i].Kind != domain.ComponentSkill {
			plan.Components[i].Support = domain.SupportUnsupported
			plan.Components[i].Reason = "Qoder project preparation supports skill bundles only; native MCP, hooks and other resources are unqualified"
		}
	}
	return nil
}

func (*Adapter) ValidateBindingProfile(root string, binding domain.ClientBinding) error {
	if binding.Scope != string(domain.ScopeProject) {
		if err := validateProfile(root, ""); err != nil {
			return err
		}
		if binding.NativeProfileRoot == "" || binding.NativeProfileRoot != root {
			return fmt.Errorf("recorded Qoder user profile differs from selected root")
		}
		return nil
	}
	project := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(root))))
	if binding.ClientID != "qoder" || binding.NativeProfileRoot != root || root != filepath.Join(project, ".uap", "managed", "clients", "qoder") || !binding.SelectedDelivery.IsZero() {
		return fmt.Errorf("qoder project binding profile differs from the selected namespace")
	}
	if err := pathpolicy.ValidateLeafID(binding.PhysicalArtifact); err != nil {
		return err
	}
	if binding.TargetLocator != filepath.Join(root, binding.PhysicalArtifact) {
		return fmt.Errorf("qoder project binding locator differs from physical artifact")
	}
	if len(binding.NativeObjects) != 1 {
		return fmt.Errorf("qoder project binding requires its exact managed package receipt")
	}
	for _, object := range binding.NativeObjects {
		if object.Kind != "managed_package_directory" || object.Path != binding.TargetLocator || object.ObjectID != "package:qoder:"+binding.PhysicalArtifact || object.ManagedDigest == "" {
			return fmt.Errorf("qoder project binding contains foreign native ownership")
		}
	}
	return pathpolicy.RequireContainedChild(project, binding.TargetLocator)
}
