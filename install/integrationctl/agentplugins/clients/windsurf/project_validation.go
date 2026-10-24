package windsurf

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.ProfileBindingValidator = (*Adapter)(nil)

const DevinProjectAction = "Devin project configuration is installed; open the selected project and inspect skills and MCP servers. Native discovery, imported aliases, enablement, workspace trust, permissions, cloud/team policy and MCP initialization remain unobserved"

func requireCurrentDevin(client domain.DetectedClient) error {
	name := strings.ToLower(filepath.Base(client.ExecutablePath))
	if client.ClientID != domain.ClientWindsurf || !filepath.IsAbs(client.ExecutablePath) || filepath.Clean(client.ExecutablePath) != client.ExecutablePath || (name != "devin" && name != "devin.exe") || client.Version != "3000.11.205" {
		return fmt.Errorf("devin project configuration requires the explicitly selected primary devin executable with observed exact version 3000.11.205; legacy Windsurf, unknown or other versions require separate qualification")
	}
	return nil
}

func (*Adapter) ValidateBindingProfile(root string, binding domain.ClientBinding) error {
	if binding.Scope != string(domain.ScopeProject) {
		return nil
	}
	return validateDevinProjectBinding(root, binding.PhysicalArtifact, binding)
}

func validateDevinProjectionRoot(root string, plan domain.DeliveryPlan) error {
	if _, err := devinProjectPlanRoot(domain.DetectedClient{ClientID: domain.ClientWindsurf, ConfigRoot: plan.TargetContext.UserConfigRoot}, plan); err != nil {
		return err
	}
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || strings.TrimSpace(root) != root {
		return fmt.Errorf("devin package path must be clean and absolute")
	}
	return pathpolicy.RequireContainedChild(plan.TargetContext.ProjectRoot, root)
}

func validateDevinProjectBinding(root, artifact string, binding domain.ClientBinding) error {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Base(root) != ".devin" {
		return fmt.Errorf("devin project registry must be an exact absolute root")
	}
	if binding.ClientID != string(domain.ClientWindsurf) || binding.Scope != string(domain.ScopeProject) || binding.NativeProfileRoot != root || binding.PhysicalArtifact != artifact {
		return fmt.Errorf("devin binding scope, artifact or native root differs from selected project")
	}
	if err := pathpolicy.ValidateLeafID(artifact); err != nil {
		return err
	}
	project := filepath.Dir(root)
	expected := filepath.Join(project, ".uap", "managed", "clients", "windsurf", artifact)
	if binding.TargetLocator != expected {
		return fmt.Errorf("devin binding package differs from selected project")
	}
	if err := pathpolicy.RequireContainedChild(project, expected); err != nil {
		return err
	}
	return devinComponents(root).ValidateObjects(devinNativeObjects(binding.NativeObjects))
}
