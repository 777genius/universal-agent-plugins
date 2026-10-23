package opencode

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

// OpenCode's project config and skills have distinct native roots. This layout
// is operational authority; it is never inferred from a staged package.
type openCodeNativeLayout struct {
	Root, Skills string
	Scope        domain.InstallScope
}

func userOpenCodeLayout(root string) openCodeNativeLayout {
	return openCodeNativeLayout{Root: root, Skills: filepath.Join(root, "skills"), Scope: domain.ScopeUser}
}

func projectOpenCodeLayout(root string) openCodeNativeLayout {
	return openCodeNativeLayout{Root: root, Skills: filepath.Join(root, ".opencode", "skills"), Scope: domain.ScopeProject}
}

func (layout openCodeNativeLayout) validate() error {
	if layout.Scope != domain.ScopeProject {
		// Legacy user callers retain their existing per-object preconditions.
		return nil
	}
	if _, err := domain.NewTargetContext(domain.ScopeProject, layout.Root, ""); err != nil {
		return err
	}
	for _, path := range []string{layout.Skills, filepath.Join(layout.Root, "opencode.json"), filepath.Join(layout.Root, "opencode.jsonc")} {
		if err := pathpolicy.RequireContainedChild(layout.Root, path); err != nil {
			return err
		}
	}
	return nil
}

func (*Adapter) NativeRegistry(in clients.PlanInput) (string, string) {
	if in.TargetContext.Scope == domain.ScopeProject {
		return in.TargetContext.ProjectRoot, in.Client.ExecutablePath
	}
	return in.Client.ConfigRoot, in.Client.ExecutablePath
}

func (*Adapter) ProjectTargetRoot(client domain.DetectedClient, mode domain.PackageMode, managedRoot string, target domain.TargetContext) (string, string, error) {
	if target.Scope != domain.ScopeProject {
		return "", "", fmt.Errorf("opencode project layout requires project scope")
	}
	if _, err := openCodeTargetLayout(client, target, ""); err != nil {
		return "", "", err
	}
	if err := pathpolicy.RequireContainedChild(target.ProjectRoot, managedRoot); err != nil {
		return "", "", fmt.Errorf("opencode prepared package must stay in the selected project: %w", err)
	}
	return shared.ManagedTargetRoot(client, mode, managedRoot)
}

func openCodeTargetLayout(client domain.DetectedClient, target domain.TargetContext, planned string) (openCodeNativeLayout, error) {
	validated, err := domain.NewTargetContext(target.Scope, target.ProjectRoot, target.UserConfigRoot)
	if err != nil {
		return openCodeNativeLayout{}, err
	}
	if validated.UserConfigRoot != "" && validated.UserConfigRoot != client.ConfigRoot {
		return openCodeNativeLayout{}, fmt.Errorf("opencode detected user profile differs from the selected context")
	}
	layout := userOpenCodeLayout(client.ConfigRoot)
	if validated.Scope == domain.ScopeProject {
		layout = projectOpenCodeLayout(validated.ProjectRoot)
	}
	if planned != "" && planned != layout.Root {
		return openCodeNativeLayout{}, fmt.Errorf("opencode native root differs from the selected context")
	}
	if layout.Root == "" && layout.Scope != domain.ScopeProject {
		return layout, nil
	}
	return layout, layout.validate()
}

func openCodePlanLayout(plan domain.DeliveryPlan) (openCodeNativeLayout, error) {
	if (plan.Scope == domain.ScopeProject) != (plan.TargetContext.Scope == domain.ScopeProject) {
		return openCodeNativeLayout{}, fmt.Errorf("opencode plan scope differs from its target context")
	}
	if plan.Scope == domain.ScopeProject {
		target, err := domain.NewTargetContext(plan.TargetContext.Scope, plan.TargetContext.ProjectRoot, plan.TargetContext.UserConfigRoot)
		if err != nil {
			return openCodeNativeLayout{}, err
		}
		if plan.NativeRegistryRoot != target.ProjectRoot {
			return openCodeNativeLayout{}, fmt.Errorf("opencode native root differs from the selected project")
		}
		layout := projectOpenCodeLayout(target.ProjectRoot)
		return layout, layout.validate()
	}
	layout := userOpenCodeLayout(plan.NativeRegistryRoot)
	return layout, layout.validate()
}

func openCodeActivationLayout(request domain.ActivationRequest) (openCodeNativeLayout, error) {
	if (request.Plan.Scope == domain.ScopeProject) != (request.Plan.TargetContext.Scope == domain.ScopeProject) {
		return openCodeNativeLayout{}, fmt.Errorf("opencode plan scope differs from its target context")
	}
	return openCodeTargetLayout(request.Client, request.Plan.TargetContext, request.Plan.NativeRegistryRoot)
}
