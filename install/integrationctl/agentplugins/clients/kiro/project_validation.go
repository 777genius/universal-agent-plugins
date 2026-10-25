package kiro

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.ProfileBindingValidator = (*Adapter)(nil)

// The user lane retains its existing registry rules. Project ownership is
// explicit and cannot be inferred from a detected global profile.
func (*Adapter) ValidateBindingProfile(root string, binding domain.ClientBinding) error {
	if binding.Scope != string(domain.ScopeProject) {
		return nil
	}
	return validateKiroProjectBinding(root, binding.PhysicalArtifact, binding)
}

func validateKiroProjectionRoot(root string, plan domain.DeliveryPlan) error {
	if _, err := kiroProjectPlanRoot(domain.DetectedClient{ConfigRoot: plan.TargetContext.UserConfigRoot}, plan); err != nil {
		return err
	}
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || strings.TrimSpace(root) != root {
		return fmt.Errorf("kiro project package path must be clean and absolute")
	}
	return pathpolicy.RequireContainedChild(plan.TargetContext.ProjectRoot, root)
}

func validateKiroProjectBinding(root, artifact string, binding domain.ClientBinding) error {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Base(root) != ".kiro" {
		return fmt.Errorf("kiro project registry must be an exact absolute root")
	}
	if binding.Scope != string(domain.ScopeProject) || binding.NativeProfileRoot != root || binding.PhysicalArtifact != artifact {
		return fmt.Errorf("kiro binding scope, artifact or native root differs from selected project")
	}
	if err := pathpolicy.ValidateLeafID(artifact); err != nil {
		return err
	}
	project := filepath.Dir(root)
	expected := filepath.Join(project, ".uap", "managed", "clients", "kiro", artifact)
	if binding.TargetLocator != expected {
		return fmt.Errorf("kiro binding package differs from selected project")
	}
	if err := pathpolicy.RequireContainedChild(project, expected); err != nil {
		return err
	}
	return validateKiroProjectObjects(root, binding.NativeObjects)
}

func validateKiroProjectObjects(root string, objects []domain.NativeObjectOwnership) error {
	// Reuse portable path/duplicate/digest validation without changing Kiro's
	// existing persisted receipt IDs or its exact JSONC transaction codec.
	normalized := make([]domain.NativeObjectOwnership, 0, len(objects))
	for _, object := range objects {
		var prefix string
		switch object.Kind {
		case SkillObjectKind:
			prefix = "kiro-skill:"
		case MCPObjectKind:
			prefix = "kiro-mcp:"
		case "managed_package_directory":
			continue
		default:
			return fmt.Errorf("unexpected Kiro project native object kind %q", object.Kind)
		}
		if object.ObjectID != prefix+object.LogicalName {
			return fmt.Errorf("kiro project receipt identity differs from its logical name")
		}
		object.ObjectID = object.Kind + ":" + object.LogicalName
		normalized = append(normalized, object)
	}
	layout := shared.ProjectComponents{ProjectRoot: filepath.Dir(root), SkillsRoot: filepath.Join(root, "skills"),
		ConfigPaths: nativeconfig.Paths{JSON: filepath.Join(root, "settings", "mcp.json")}, Codec: nativeconfig.CodecMCPServers,
		SkillKind: SkillObjectKind, MCPKind: MCPObjectKind}
	return layout.ValidateObjects(normalized)
}
