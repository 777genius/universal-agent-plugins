package cline

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	clineProjectSkillKind = "cline_project_skill_directory"
	clineProjectMCPKind   = "cline_project_mcp_server"
)

func clineProjectComponents(root string) shared.ProjectComponents {
	return shared.ProjectComponents{ProjectRoot: root, SkillsRoot: filepath.Join(root, ".cline", "skills"),
		ConfigPaths: nativeconfig.Paths{JSON: filepath.Join(root, ".uap", "clients", "cline", "mcp.json")},
		Codec:       nativeconfig.CodecCline, SkillKind: clineProjectSkillKind, MCPKind: clineProjectMCPKind}
}

func clineProjectOwned(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	selected := []domain.NativeObjectOwnership{}
	for _, object := range objects {
		if object.Kind != "managed_package_directory" {
			selected = append(selected, object)
		}
	}
	return selected
}

func clineProjectObjects(staging string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) ([]domain.NativeObjectOwnership, error) {
	layout := clineProjectComponents(plan.TargetContext.ProjectRoot)
	objects := []domain.NativeObjectOwnership{}
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		var object domain.NativeObjectOwnership
		var err error
		switch component.Kind {
		case domain.ComponentSkill:
			skill, ok := envelope.Skills[component.Name]
			if !ok {
				return nil, fmt.Errorf("planned Cline project skill is missing")
			}
			relative := filepath.ToSlash(filepath.Dir(filepath.FromSlash(skill.RelativePath)))
			if skill.RelativePath == "" {
				relative = "skills/" + component.Name
			}
			object, err = layout.SkillObject(component.Name, relative, staging)
		case domain.ComponentMCPServer:
			var projection ClineProjection
			projection, err = ReadClineProjection(staging)
			if err == nil {
				server, ok := projection.Servers[component.Name]
				if !ok {
					return nil, fmt.Errorf("projected Cline MCP server is missing")
				}
				object, err = layout.MCPObject(component.Name, server, layout.ConfigPaths.JSON)
			}
		default:
			return nil, fmt.Errorf("unsupported Cline project component %q", component.Kind)
		}
		if err != nil {
			return nil, err
		}
		objects = append(objects, object)
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].ObjectID < objects[j].ObjectID })
	return objects, layout.ValidateObjects(objects)
}

func clineProjectServers(active string, objects []domain.NativeObjectOwnership) (map[string]nativeconfig.Server, error) {
	servers := map[string]nativeconfig.Server{}
	for _, object := range objects {
		if object.Kind != clineProjectMCPKind {
			continue
		}
		projection, err := ReadClineProjection(active)
		if err != nil {
			return nil, err
		}
		server, ok := projection.Servers[object.LogicalName]
		if !ok {
			return nil, fmt.Errorf("projected Cline MCP server is missing")
		}
		servers[object.LogicalName] = server
	}
	return servers, nil
}

func validateClineProjectSelection(plan domain.DeliveryPlan, objects []domain.NativeObjectOwnership) error {
	selected := map[string]bool{}
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		kind := clineProjectSkillKind
		if component.Kind == domain.ComponentMCPServer {
			kind = clineProjectMCPKind
		}
		selected[kind+":"+component.Name] = true
	}
	owned := clineProjectOwned(objects)
	if len(owned) != len(selected) {
		return fmt.Errorf("cline delivered component selection is incomplete")
	}
	for _, object := range owned {
		if !selected[object.ObjectID] {
			return fmt.Errorf("cline delivered component differs from selection")
		}
	}
	return clineProjectComponents(plan.TargetContext.ProjectRoot).ValidateObjects(owned)
}
