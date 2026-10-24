package copilot

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	copilotProjectSkillKind = "copilot_project_skill_directory"
	copilotProjectMCPKind   = "copilot_project_mcp_server"
)

func copilotProjectObjects(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) ([]domain.NativeObjectOwnership, error) {
	project, err := copilotPlanRoot(domain.DetectedClient{ConfigRoot: plan.TargetContext.UserConfigRoot}, plan)
	if err != nil {
		return nil, err
	}
	if err := pathpolicy.RequireContainedChild(project, root); err != nil {
		return nil, err
	}
	layout, err := copilotComponents(project)
	if err != nil {
		return nil, err
	}
	objects := make([]domain.NativeObjectOwnership, 0, len(plan.Components))
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		object, err := copilotProjectObject(layout, root, envelope, component)
		if err != nil {
			return nil, err
		}
		objects = append(objects, object)
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].ObjectID < objects[j].ObjectID })
	return objects, layout.ValidateObjects(objects)
}

func copilotProjectObject(layout shared.ProjectComponents, root string, envelope domain.PackageEnvelope, component domain.ComponentDecision) (domain.NativeObjectOwnership, error) {
	switch component.Kind {
	case domain.ComponentSkill:
		skill, ok := envelope.Skills[component.Name]
		if !ok {
			return domain.NativeObjectOwnership{}, fmt.Errorf("copilot planned skill is missing")
		}
		relative := filepath.ToSlash(filepath.Dir(filepath.FromSlash(skill.RelativePath)))
		if skill.RelativePath == "" {
			relative = "skills/" + component.Name
		}
		return layout.SkillObject(component.Name, relative, root)
	case domain.ComponentMCPServer:
		server, err := readCopilotProjectServer(root, component.Name)
		if err != nil {
			return domain.NativeObjectOwnership{}, err
		}
		return layout.MCPObject(component.Name, server, layout.ConfigPaths.JSON)
	default:
		return domain.NativeObjectOwnership{}, fmt.Errorf("copilot project component %q is unsupported", component.Kind)
	}
}

func copilotNativeObjects(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	selected := make([]domain.NativeObjectOwnership, 0, len(objects))
	for _, object := range objects {
		if object.Kind != "managed_package_directory" {
			selected = append(selected, object)
		}
	}
	return selected
}
