package windsurf

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	DevinProjectSkillKind = "devin_project_skill_directory"
	DevinProjectMCPKind   = "devin_project_mcp_server"
)

func devinComponents(root string) shared.ProjectComponents {
	return shared.ProjectComponents{ProjectRoot: filepath.Dir(root), SkillsRoot: filepath.Join(root, "skills"),
		ConfigPaths: nativeconfig.Paths{JSON: filepath.Join(root, "mcp_config.json")}, Codec: nativeconfig.CodecDevin,
		SkillKind: DevinProjectSkillKind, MCPKind: DevinProjectMCPKind}
}

func devinProjectObjects(stagingRoot string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) ([]domain.NativeObjectOwnership, error) {
	target, err := domain.NewTargetContext(plan.TargetContext.Scope, plan.TargetContext.ProjectRoot, plan.TargetContext.UserConfigRoot)
	if err != nil {
		return nil, err
	}
	if plan.Scope != domain.ScopeProject || target.Scope != domain.ScopeProject || plan.NativeRegistryRoot != filepath.Join(target.ProjectRoot, ".devin") {
		return nil, fmt.Errorf("devin projection differs from selected project")
	}
	if err := validateDevinProjectionRoot(stagingRoot, plan); err != nil {
		return nil, err
	}
	if err := pathpolicy.RequireContainedChild(target.ProjectRoot, stagingRoot); err != nil {
		return nil, err
	}
	layout := devinComponents(plan.NativeRegistryRoot)
	objects := []domain.NativeObjectOwnership{}
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		object, err := devinProjectObject(layout, stagingRoot, envelope, component)
		if err != nil {
			return nil, err
		}
		if object != nil {
			objects = append(objects, *object)
		}
	}
	return objects, layout.ValidateObjects(objects)
}

func devinProjectObject(layout shared.ProjectComponents, stagingRoot string, envelope domain.PackageEnvelope, component domain.ComponentDecision) (*domain.NativeObjectOwnership, error) {
	switch component.Kind {
	case domain.ComponentSkill:
		// Skill names are filesystem leafs; MCP names remain JSON member keys.
		if err := pathpolicy.ValidateLeafID(component.Name); err != nil {
			return nil, err
		}
		skill, ok := envelope.Skills[component.Name]
		if !ok {
			return nil, fmt.Errorf("planned Devin skill %q is missing", component.Name)
		}
		relative := filepath.Dir(filepath.FromSlash(skill.RelativePath))
		if skill.RelativePath == "" {
			relative = filepath.Join("skills", component.Name)
		}
		object, err := layout.SkillObject(component.Name, filepath.ToSlash(relative), stagingRoot)
		return &object, err
	case domain.ComponentMCPServer:
		server, err := devinProjectedServer(stagingRoot, component.Name)
		if err != nil {
			return nil, err
		}
		object, err := layout.MCPObject(component.Name, server, layout.ConfigPaths.JSON)
		return &object, err
	default:
		return nil, fmt.Errorf("unsupported Devin project component kind %q", component.Kind)
	}
}

func devinProjectedServer(activeRoot, name string) (nativeconfig.Server, error) {
	file, err := nativeconfig.New().ReadExactFile(filepath.Join(activeRoot, "mcp.json"))
	if err != nil {
		return nativeconfig.Server{}, err
	}
	document, err := shared.DecodeStrictJSONObject(file.Body)
	if err != nil {
		return nativeconfig.Server{}, err
	}
	entries, ok := document["mcpServers"].(map[string]any)
	if !ok {
		return nativeconfig.Server{}, fmt.Errorf("devin package has no mcpServers object")
	}
	entry, ok := entries[name].(map[string]any)
	if !ok {
		return nativeconfig.Server{}, fmt.Errorf("devin projected MCP server %q is missing", name)
	}
	body, err := json.Marshal(entry)
	if err != nil {
		return nativeconfig.Server{}, err
	}
	server, err := decodeProjectedWindsurfServer(body)
	if err != nil {
		return server, err
	}
	server.StdioValuesResolved, server.CWDResolved = true, true
	return server, nil
}

func devinDesiredServers(activeRoot string, objects []domain.NativeObjectOwnership) (map[string]nativeconfig.Server, error) {
	servers := map[string]nativeconfig.Server{}
	for _, object := range objects {
		if object.Kind != DevinProjectMCPKind {
			continue
		}
		server, err := devinProjectedServer(activeRoot, object.LogicalName)
		if err != nil {
			return nil, err
		}
		servers[object.LogicalName] = server
	}
	return servers, nil
}

// Package directory ownership belongs to the shared artifact lifecycle, not the
// native component writer. Other unknown kinds remain visible and fail closed.
func devinNativeObjects(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	result := []domain.NativeObjectOwnership{}
	for _, object := range objects {
		if object.Kind != "managed_package_directory" {
			result = append(result, object)
		}
	}
	return result
}
