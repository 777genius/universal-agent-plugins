package kimi

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	kimiProjectSkillKind = "kimi_project_skill_directory"
	kimiProjectMCPKind   = "kimi_project_mcp_server"
)

func kimiComponents(root string) shared.ProjectComponents {
	return shared.ProjectComponents{ProjectRoot: filepath.Dir(root), SkillsRoot: filepath.Join(root, "skills"), ConfigPaths: nativeconfig.Paths{JSON: filepath.Join(root, "mcp.json")}, Codec: nativeconfig.CodecKimi, SkillKind: kimiProjectSkillKind, MCPKind: kimiProjectMCPKind}
}

func kimiProjectObjects(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) ([]domain.NativeObjectOwnership, error) {
	native, err := kimiPlanRoot(domain.DetectedClient{ConfigRoot: plan.TargetContext.UserConfigRoot}, plan)
	if err != nil {
		return nil, err
	}
	if err := pathpolicy.RequireContainedChild(plan.TargetContext.ProjectRoot, root); err != nil {
		return nil, err
	}
	layout := kimiComponents(native)
	objects := []domain.NativeObjectOwnership{}
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		var object domain.NativeObjectOwnership
		switch component.Kind {
		case domain.ComponentSkill:
			skill, ok := envelope.Skills[component.Name]
			if !ok {
				return nil, fmt.Errorf("kimi planned skill is missing")
			}
			relative := filepath.ToSlash(filepath.Dir(filepath.FromSlash(skill.RelativePath)))
			if skill.RelativePath == "" {
				relative = "skills/" + component.Name
			}
			object, err = layout.SkillObject(component.Name, relative, root)
		case domain.ComponentMCPServer:
			var server nativeconfig.Server
			server, err = kimiProjectServer(root, component.Name)
			if err == nil {
				object, err = layout.MCPObject(component.Name, server, layout.ConfigPaths.JSON)
			}
		default:
			return nil, fmt.Errorf("unsupported Kimi project component %q", component.Kind)
		}
		if err != nil {
			return nil, err
		}
		objects = append(objects, object)
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].ObjectID < objects[j].ObjectID })
	return objects, layout.ValidateObjects(objects)
}

func kimiDesiredServers(root string, objects []domain.NativeObjectOwnership) (map[string]nativeconfig.Server, error) {
	servers := map[string]nativeconfig.Server{}
	for _, object := range objects {
		if object.Kind != kimiProjectMCPKind {
			continue
		}
		server, err := kimiProjectServer(root, object.LogicalName)
		if err != nil {
			return nil, err
		}
		servers[object.LogicalName] = server
	}
	return servers, nil
}

func kimiNativeObjects(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	selected := []domain.NativeObjectOwnership{}
	for _, object := range objects {
		if object.Kind != "managed_package_directory" {
			selected = append(selected, object)
		}
	}
	return selected
}
