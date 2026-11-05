package windsurf

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	DevinGlobalSkillKind = "devin_global_skill_directory"
	DevinGlobalMCPKind = "devin_global_mcp_server"
)

func devinGlobalComponents(root, activePath string) shared.ProjectComponents {
	return shared.ProjectComponents{
		ProjectRoot: root,
		SkillsRoot: filepath.Join(root, "skills"),
		ManagedPackagePath: activePath,
		ConfigPaths: nativeconfig.Paths{JSON: filepath.Join(root, "mcp_config.json")},
		Codec: nativeconfig.CodecDevin,
		SkillKind: DevinGlobalSkillKind, MCPKind: DevinGlobalMCPKind,
	}
}

func isDevinGlobalPlan(plan domain.DeliveryPlan) bool {
	return plan.ClientID == domain.ClientWindsurf && plan.Scope == domain.ScopeUser &&
		plan.TargetContext.Scope == domain.ScopeUser &&
		plan.NativeRegistryRoot != "" && plan.NativeRegistryExecutable == "" &&
		filepath.Base(plan.NativeRegistryRoot) == "devin"
}

func validateDevinGlobalProjectionRoot(stagingRoot string, plan domain.DeliveryPlan) error {
	if !isDevinGlobalPlan(plan) || plan.TargetContext.ProjectRoot != "" ||
		plan.ActivePath != filepath.Join(plan.TargetRoot, plan.PhysicalArtifactID) ||
		plan.TargetRoot == "" || !filepath.IsAbs(plan.NativeRegistryRoot) ||
		filepath.Clean(plan.NativeRegistryRoot) != plan.NativeRegistryRoot {
		return fmt.Errorf("Devin Global projection differs from selected profile")
	}
	if err := pathpolicy.ValidateLeafID(plan.PhysicalArtifactID); err != nil {
		return err
	}
	if err := pathpolicy.RequireContainedChild(plan.TargetRoot, plan.ActivePath); err != nil {
		return err
	}
	return pathpolicy.RequireContainedChild(plan.TargetRoot, stagingRoot)
}

func devinGlobalObjects(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) ([]domain.NativeObjectOwnership, error) {
	if err := validateDevinGlobalProjectionRoot(root, plan); err != nil {
		return nil, err
	}
	layout := devinGlobalComponents(plan.NativeRegistryRoot, root)
	var objects []domain.NativeObjectOwnership
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		object, err := devinProjectObject(layout, root, envelope, component)
		if err != nil {
			return nil, err
		}
		if object != nil {
			objects = append(objects, *object)
		}
	}
	return objects, layout.ValidateObjects(objects)
}

func hasDevinGlobalObjects(objects []domain.NativeObjectOwnership) bool {
	for _, object := range objects {
		if object.Kind == DevinGlobalSkillKind || object.Kind == DevinGlobalMCPKind {
			return true
		}
	}
	return false
}
