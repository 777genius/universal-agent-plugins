package vscode

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const workspaceSkillKind = "vscode_project_skill_directory"
const workspaceMCPKind = "vscode_project_mcp_server"

func workspaceComponents(root string) shared.ProjectComponents {
	return shared.ProjectComponents{ProjectRoot: root, SkillsRoot: filepath.Join(root, ".github", "skills"),
		ConfigPaths: nativeconfig.Paths{JSON: filepath.Join(root, ".vscode", "mcp.json")}, Codec: nativeconfig.CodecVSCode,
		SkillKind: workspaceSkillKind, MCPKind: workspaceMCPKind}
}

func workspaceOwned(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	selected := []domain.NativeObjectOwnership{}
	for _, object := range objects {
		if object.Kind != "managed_package_directory" {
			selected = append(selected, object)
		}
	}
	return selected
}

func workspaceObjects(staging string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) ([]domain.NativeObjectOwnership, error) {
	layout := workspaceComponents(plan.TargetContext.ProjectRoot)
	objects := []domain.NativeObjectOwnership{}
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		object, err := workspaceObject(layout, staging, envelope, component)
		if err != nil {
			return nil, err
		}
		objects = append(objects, object)
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].ObjectID < objects[j].ObjectID })
	return objects, layout.ValidateObjects(objects)
}

func workspaceObject(layout shared.ProjectComponents, staging string, envelope domain.PackageEnvelope, component domain.ComponentDecision) (domain.NativeObjectOwnership, error) {
	switch component.Kind {
	case domain.ComponentSkill:
		skill, ok := envelope.Skills[component.Name]
		if !ok {
			return domain.NativeObjectOwnership{}, fmt.Errorf("planned VS Code workspace skill is missing")
		}
		relative := filepath.ToSlash(filepath.Dir(filepath.FromSlash(skill.RelativePath)))
		if skill.RelativePath == "" {
			relative = "skills/" + component.Name
		}
		return layout.SkillObject(component.Name, relative, staging)
	case domain.ComponentMCPServer:
		server, err := readWorkspaceServer(staging, component.Name)
		if err != nil {
			return domain.NativeObjectOwnership{}, err
		}
		return layout.MCPObject(component.Name, server, layout.ConfigPaths.JSON)
	default:
		return domain.NativeObjectOwnership{}, fmt.Errorf("unsupported VS Code project component %q", component.Kind)
	}
}

func workspaceServers(active string, objects []domain.NativeObjectOwnership) (map[string]nativeconfig.Server, error) {
	servers := map[string]nativeconfig.Server{}
	for _, object := range objects {
		if object.Kind != workspaceMCPKind {
			continue
		}
		server, err := readWorkspaceServer(active, object.LogicalName)
		if err != nil {
			return nil, err
		}
		servers[object.LogicalName] = server
	}
	return servers, nil
}

func workspaceSelection(plan domain.DeliveryPlan, objects []domain.NativeObjectOwnership) error {
	selected := map[string]bool{}
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		kind := workspaceSkillKind
		if component.Kind == domain.ComponentMCPServer {
			kind = workspaceMCPKind
		}
		selected[kind+":"+component.Name] = true
	}
	owned := workspaceOwned(objects)
	if len(owned) != len(selected) {
		return fmt.Errorf("VS Code delivered component selection is incomplete")
	}
	for _, object := range owned {
		if !selected[object.ObjectID] {
			return fmt.Errorf("VS Code delivered component differs from selection")
		}
	}
	return workspaceComponents(plan.TargetContext.ProjectRoot).ValidateObjects(owned)
}
