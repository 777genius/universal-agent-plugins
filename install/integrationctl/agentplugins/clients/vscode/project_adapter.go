package vscode

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// ProjectAdapter prepares one selected local workspace's owned skills and MCP
// entries. It deliberately embeds neither the user Copilot adapter nor Local.
type ProjectAdapter struct{}

func NewProject() *ProjectAdapter { return &ProjectAdapter{} }

var (
	_ clients.Adapter                 = (*ProjectAdapter)(nil)
	_ clients.HostDetector            = (*ProjectAdapter)(nil)
	_ clients.ProjectTargetLayout     = (*ProjectAdapter)(nil)
	_ clients.NativeRegistryLayout    = (*ProjectAdapter)(nil)
	_ clients.PlanPrecondition        = (*ProjectAdapter)(nil)
	_ clients.PlanRefiner             = (*ProjectAdapter)(nil)
	_ clients.ProfileBindingValidator = (*ProjectAdapter)(nil)
	_ clients.CompatibilityLimiter    = (*ProjectAdapter)(nil)
)

func (*ProjectAdapter) ID() domain.ClientID { return domain.ClientVSCode }

func (*ProjectAdapter) DetectSurfaces(host clients.Host) clients.Detection {
	// This detector probes code and the original VS Code profile. It does not
	// use the Copilot sibling to establish presence or workspace authority.
	return New().DetectSurfaces(host)
}

func (*ProjectAdapter) NativeRegistry(in clients.PlanInput) (string, string) {
	return filepath.Join(in.TargetContext.ProjectRoot, ".vscode"), ""
}

func (*ProjectAdapter) ProjectTargetRoot(client domain.DetectedClient, mode domain.PackageMode, managed string, target domain.TargetContext) (string, string, error) {
	root, err := workspaceRoot(client, target, "")
	if err != nil {
		return "", "", err
	}
	if managed != filepath.Join(root, ".uap", "managed") {
		return "", "", fmt.Errorf("VS Code managed root differs from selected project")
	}
	return shared.ManagedTargetRoot(client, mode, managed)
}

func (*ProjectAdapter) CheckPlanPrecondition(in clients.PlanInput, plan *domain.DeliveryPlan) error {
	if plan.Scope != domain.ScopeProject || plan.TargetContext.Scope != domain.ScopeProject {
		return fmt.Errorf("VS Code project adapter requires explicit project composition and scope")
	}
	_, err := workspaceRoot(in.Client, plan.TargetContext, plan.NativeRegistryRoot)
	return err
}

func (*ProjectAdapter) RefinePlan(ctx context.Context, in clients.PlanInput, plan *domain.DeliveryPlan) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root, err := workspacePlanRoot(in.Client, *plan)
	if err != nil {
		return err
	}
	if plan.Status == domain.PlanUnsupported {
		return nil
	}
	shared.PromoteNativeReady(plan, filepath.Join(root, ".vscode"), shared.OnlyNativeComponents(plan.Components))
	plan.Activation = domain.ActivationPrepared
	plan.UserActions = shared.AppendUnique(plan.UserActions, workspaceAction(root))
	return nil
}

func (*ProjectAdapter) ClientLimitations(domain.PackageEnvelope) []string { return nil }

func (*ProjectAdapter) ComponentLimitations(_ domain.PackageEnvelope, item domain.ComponentDecision) ([]string, []string) {
	if item.Kind != domain.ComponentSkill && item.Kind != domain.ComponentMCPServer {
		return []string{"vscode_project_component_unsupported"}, nil
	}
	return nil, []string{"vscode_project_prepared_files_only"}
}

func workspaceRoot(client domain.DetectedClient, target domain.TargetContext, planned string) (string, error) {
	selected, err := domain.NewTargetContext(target.Scope, target.ProjectRoot, target.UserConfigRoot)
	if err != nil {
		return "", err
	}
	if client.ClientID != domain.ClientVSCode || selected.Scope != domain.ScopeProject || selected.UserConfigRoot == "" || selected.UserConfigRoot != client.ConfigRoot {
		return "", fmt.Errorf("VS Code workspace requires matching project scope and original user profile")
	}
	root := selected.ProjectRoot
	if planned != "" && planned != filepath.Join(root, ".vscode") {
		return "", fmt.Errorf("VS Code native registry differs from selected workspace")
	}
	for _, pair := range [][2]string{{root, client.ConfigRoot}, {client.ConfigRoot, root}} {
		rel, err := filepath.Rel(pair[0], pair[1])
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("VS Code project overlaps the original user profile")
		}
	}
	for _, path := range []string{filepath.Join(root, ".github", "skills"), filepath.Join(root, ".vscode", "mcp.json")} {
		if err := pathpolicy.RequireContainedChild(root, path); err != nil {
			return "", err
		}
	}
	return root, nil
}

func workspacePlanRoot(client domain.DetectedClient, plan domain.DeliveryPlan) (string, error) {
	if plan.ClientID != domain.ClientVSCode || plan.Scope != domain.ScopeProject || plan.TargetContext.Scope != domain.ScopeProject || plan.NativeRegistryExecutable != "" {
		return "", fmt.Errorf("VS Code project plan requires matching file-only workspace authority")
	}
	root, err := workspaceRoot(client, plan.TargetContext, plan.NativeRegistryRoot)
	if err != nil {
		return "", err
	}
	if plan.NativeRegistryRoot != filepath.Join(root, ".vscode") {
		return "", fmt.Errorf("VS Code project native registry is missing")
	}
	if err := pathpolicy.ValidateLeafID(plan.PhysicalArtifactID); err != nil {
		return "", err
	}
	managed := filepath.Join(root, ".uap", "managed")
	target := filepath.Join(managed, "clients", "vscode")
	if plan.TargetAnchor != managed || plan.TargetRoot != target || plan.ActivePath != filepath.Join(target, plan.PhysicalArtifactID) {
		return "", fmt.Errorf("VS Code managed package differs from selected workspace")
	}
	return root, pathpolicy.RequireContainedChild(root, plan.ActivePath)
}

func (*ProjectAdapter) ValidateBindingProfile(registry string, binding domain.ClientBinding) error {
	if binding.ClientID != string(domain.ClientVSCode) || binding.Scope != string(domain.ScopeProject) || filepath.Base(registry) != ".vscode" || binding.NativeProfileRoot != registry {
		return fmt.Errorf("VS Code binding workspace differs")
	}
	if err := pathpolicy.ValidateLeafID(binding.PhysicalArtifact); err != nil {
		return err
	}
	root := filepath.Dir(registry)
	expected := filepath.Join(root, ".uap", "managed", "clients", "vscode", binding.PhysicalArtifact)
	if binding.TargetLocator != expected {
		return fmt.Errorf("VS Code binding package differs from selected workspace")
	}
	if err := pathpolicy.RequireContainedChild(root, expected); err != nil {
		return err
	}
	for _, object := range binding.NativeObjects {
		if object.Kind == "managed_package_directory" && (object.Path != expected || object.ManagedDigest == "") {
			return fmt.Errorf("VS Code package ownership differs from selected workspace")
		}
	}
	return workspaceComponents(root).ValidateObjects(workspaceOwned(binding.NativeObjects))
}

func workspaceAction(root string) string {
	return fmt.Sprintf("VS Code project component files are installed for %q; open that folder, review workspace/server trust and inspect skills and MCP. Native discovery, effective Local/Copilot selection, approvals, MCP initialization and active sessions remain unobserved", root)
}
