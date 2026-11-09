package hermes

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	userSkillKind = "hermes_user_skill"
	projectSkillKind = "hermes_project_skill"
	neverMCPKind = "hermes_mcp_not_supported"
)

var (
	_ clients.Projector = (*Adapter)(nil)
	_ clients.ActiveNativeProjector = (*Adapter)(nil)
	_ clients.ReadOnlyVerifier = (*Adapter)(nil)
)

func components(plan domain.DeliveryPlan, selectedPackage string) (shared.ProjectComponents, error) {
	authority := plan.NativeRegistryRoot
	if !cleanRoot(authority) {
		return shared.ProjectComponents{}, fmt.Errorf("Hermes native root must be canonical and absolute")
	}
	layout := shared.ProjectComponents{
		SkillsRoot: filepath.Join(authority, "skills"),
		// There is no project-local Hermes MCP authority. The sentinel path
		// is never written: this layout admits Skills alone.
		ConfigPaths: nativeconfig.Paths{JSON: filepath.Join(authority, ".uap-no-mcp.json")},
		Codec: nativeconfig.CodecMCPServers,
		SkillKind: userSkillKind,
		MCPKind: neverMCPKind,
		ManagedPackagePath: selectedPackage,
	}
	switch plan.Scope {
	case domain.ScopeUser:
		if plan.TargetContext.UserConfigRoot != "" && plan.TargetContext.UserConfigRoot != authority {
			return layout, fmt.Errorf("Hermes user profile authority differs")
		}
		layout.ProjectRoot = authority
	case domain.ScopeProject:
		if !cleanRoot(plan.TargetContext.ProjectRoot) ||
			filepath.Join(plan.TargetContext.ProjectRoot, ".hermes") != authority {
			return layout, fmt.Errorf("Hermes project Skills root must be the exact selected project")
		}
		layout.ProjectRoot = plan.TargetContext.ProjectRoot
		layout.SkillKind = projectSkillKind
	default:
		return layout, fmt.Errorf("Hermes does not support this installation scope")
	}
	return layout, nil
}

func buildSkillObjects(ctx context.Context, staged string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) ([]domain.NativeObjectOwnership, error) {
	layout, err := components(plan, staged)
	if err != nil {
		return nil, err
	}
	objects := make([]domain.NativeObjectOwnership, 0, len(plan.Components))
	for _, component := range plan.Components {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if component.Kind != domain.ComponentSkill || component.Support == domain.SupportUnsupported {
			return nil, fmt.Errorf("Hermes package contains a non-Skills or unsupported component")
		}
		skill, ok := envelope.Skills[component.Name]
		if !ok {
			return nil, fmt.Errorf("Hermes selected skill %q is missing from verified package", component.Name)
		}
		relative := filepath.ToSlash(filepath.Join("skills", component.Name))
		if skill.RelativePath != "" {
			relative = filepath.ToSlash(filepath.Dir(filepath.FromSlash(skill.RelativePath)))
		}
		object, err := layout.SkillObject(component.Name, relative, staged)
		if err != nil {
			return nil, err
		}
		objects = append(objects, object)
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].ObjectID < objects[j].ObjectID })
	return objects, layout.ValidateObjects(objects)
}

func (*Adapter) Project(ctx context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	return buildSkillObjects(ctx, in.StagingPath, in.Envelope, in.Plan)
}

func (*Adapter) ProjectActiveNative(ctx context.Context, root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, _ string) ([]domain.NativeObjectOwnership, error) {
	if root != plan.ActivePath {
		return nil, fmt.Errorf("Hermes active package differs from recorded installation")
	}
	return buildSkillObjects(ctx, root, envelope, plan)
}

func (*Adapter) VerifierAvailable(_ domain.DetectedClient, plan domain.DeliveryPlan, _ string) bool {
	return plan.NativeRegistryRoot != "" && len(plan.Components) > 0
}

func ownedObjects(scope domain.InstallScope, objects []domain.NativeObjectOwnership) ([]domain.NativeObjectOwnership, error) {
	kind := userSkillKind
	if scope == domain.ScopeProject {
		kind = projectSkillKind
	}
	result := make([]domain.NativeObjectOwnership, 0, len(objects))
	for _, object := range objects {
		if object.Kind == "managed_package_directory" {
			continue // Generic package directory is owned and verified by the installer.
		}
		if object.Kind != kind {
			return nil, fmt.Errorf("Hermes refused foreign native ownership type")
		}
		result = append(result, object)
	}
	return result, nil
}
