package codex

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	codexProjectSkillKind = "codex_project_skill_directory"
	codexProjectMCPKind   = "codex_project_mcp_server"
)

func projectCodexComponents(in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	if _, err := codexProjectPlanRoot(domain.DetectedClient{ConfigRoot: in.Plan.TargetContext.UserConfigRoot}, in.Plan); err != nil {
		return nil, err
	}
	if err := shared.ProjectOpenAIMCP(in.StagingPath, in.Envelope, shared.SupportedMCPNames(in.Plan), in.Hints, in.Plan.ActivePath, in.PluginDataPath); err != nil {
		return nil, err
	}
	return codexProjectObjects(in.StagingPath, in.Envelope, in.Plan)
}

func codexProjectObjects(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) ([]domain.NativeObjectOwnership, error) {
	if _, err := codexProjectPlanRoot(domain.DetectedClient{ConfigRoot: plan.TargetContext.UserConfigRoot}, plan); err != nil {
		return nil, err
	}
	objects := make([]domain.NativeObjectOwnership, 0, len(plan.Components))
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		object, err := codexProjectObject(root, envelope, plan, component)
		if err != nil {
			return nil, err
		}
		objects = append(objects, object)
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].ObjectID < objects[j].ObjectID })
	return objects, validateCodexProjectObjects(plan.NativeRegistryRoot, objects)
}

func codexProjectObject(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, component domain.ComponentDecision) (domain.NativeObjectOwnership, error) {
	layout := codexProjectLayout(plan.NativeRegistryRoot)
	switch component.Kind {
	case domain.ComponentSkill:
		skill, ok := envelope.Skills[component.Name]
		if !ok {
			return domain.NativeObjectOwnership{}, fmt.Errorf("planned Codex skill is missing")
		}
		relative := skill.RelativePath
		if relative == "" {
			relative = filepath.ToSlash(filepath.Join("skills", component.Name, "SKILL.md"))
		}
		return layout.SkillObject(component.Name, filepath.ToSlash(filepath.Dir(filepath.FromSlash(relative))), root)
	case domain.ComponentMCPServer:
		server, err := readCodexProjectServer(root, component.Name)
		if err != nil {
			return domain.NativeObjectOwnership{}, err
		}
		return layout.MCPObject(component.Name, server, codexProjectConfigPath(plan.NativeRegistryRoot))
	default:
		return domain.NativeObjectOwnership{}, fmt.Errorf("unsupported Codex project component kind %q", component.Kind)
	}
}

func codexProjectLayout(root string) shared.ProjectComponents {
	return shared.ProjectComponents{ProjectRoot: root, SkillsRoot: filepath.Join(root, ".agents", "skills"),
		ConfigPaths: nativeconfig.Paths{JSON: codexProjectConfigPath(root)}, Codec: nativeconfig.CodecCodex,
		SkillKind: codexProjectSkillKind, MCPKind: codexProjectMCPKind}
}

func codexProjectConfigPath(root string) string { return filepath.Join(root, ".codex", "config.toml") }
func codexProjectSkillPath(root, name string) string {
	return filepath.Join(root, ".agents", "skills", name)
}

func codexProjectOwned(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	var owned []domain.NativeObjectOwnership
	for _, object := range objects {
		if object.Kind == codexProjectSkillKind || object.Kind == codexProjectMCPKind {
			owned = append(owned, object)
		}
	}
	return owned
}

func validateCodexProjectObjects(root string, objects []domain.NativeObjectOwnership) error {
	for _, object := range objects {
		if object.Kind != codexProjectSkillKind && object.Kind != codexProjectMCPKind && object.Kind != "managed_package_directory" {
			return fmt.Errorf("codex project binding contains an unexpected native object kind")
		}
	}
	return codexProjectLayout(root).ValidateObjects(codexProjectOwned(objects))
}
