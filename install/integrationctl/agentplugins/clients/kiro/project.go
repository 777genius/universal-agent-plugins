package kiro

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func BuildNativeObjects(stagingRoot string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) ([]domain.NativeObjectOwnership, error) {
	configRoot := strings.TrimSpace(plan.NativeRegistryRoot)
	if configRoot == "" {
		return nil, fmt.Errorf("the Kiro config root is unavailable")
	}
	objects := make([]domain.NativeObjectOwnership, 0, len(envelope.Skills)+len(envelope.MCP.Servers))
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		switch component.Kind {
		case domain.ComponentSkill:
			object, err := kiroSkillOwnership(stagingRoot, configRoot, envelope, component)
			if err != nil {
				return nil, err
			}
			objects = append(objects, object)
		case domain.ComponentMCPServer:
			object, err := kiroMCPOwnership(stagingRoot, configRoot, component)
			if err != nil {
				return nil, err
			}
			objects = append(objects, object)
		}
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].ObjectID < objects[j].ObjectID })
	return objects, nil
}

func kiroSkillOwnership(stagingRoot, configRoot string, envelope domain.PackageEnvelope, component domain.ComponentDecision) (domain.NativeObjectOwnership, error) {
	if err := pathpolicy.ValidateLeafID(component.Name); err != nil {
		return domain.NativeObjectOwnership{}, fmt.Errorf("invalid Kiro skill name %q: %w", component.Name, err)
	}
	skill, ok := envelope.Skills[component.Name]
	if !ok {
		return domain.NativeObjectOwnership{}, fmt.Errorf("planned Kiro skill %q is missing", component.Name)
	}
	relative := filepath.FromSlash(strings.TrimSpace(skill.RelativePath))
	if relative == "" {
		relative = filepath.Join("skills", component.Name, "SKILL.md")
	}
	sourceRoot := filepath.Join(stagingRoot, filepath.Dir(relative))
	if err := pathpolicy.RequireContainedChild(stagingRoot, sourceRoot); err != nil {
		return domain.NativeObjectOwnership{}, fmt.Errorf("unsafe Kiro skill source %q: %w", component.Name, err)
	}
	digest, err := shared.DigestSkillDirectory(sourceRoot)
	if err != nil {
		return domain.NativeObjectOwnership{}, fmt.Errorf("digest Kiro skill %q: %w", component.Name, err)
	}
	target := filepath.Join(configRoot, "skills", component.Name)
	if err := ValidateNativePath(configRoot, target); err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	return domain.NativeObjectOwnership{
		ObjectID: "kiro-skill:" + component.Name, Kind: kiroSkillObjectKind,
		LogicalName: component.Name, Path: target,
		SourceRelative: filepath.ToSlash(filepath.Dir(relative)), ManagedDigest: digest,
		ProtectionClass: "managed",
	}, nil
}

func kiroMCPOwnership(stagingRoot, configRoot string, component domain.ComponentDecision) (domain.NativeObjectOwnership, error) {
	if err := pathpolicy.ValidateLeafID(component.Name); err != nil {
		return domain.NativeObjectOwnership{}, fmt.Errorf("invalid Kiro MCP server name %q: %w", component.Name, err)
	}
	server, err := projectedKiroMCPServer(stagingRoot, component.Name)
	if err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	return domain.NativeObjectOwnership{
		ObjectID: "kiro-mcp:" + component.Name, Kind: kiroMCPObjectKind,
		LogicalName: component.Name, Path: filepath.Join(configRoot, "settings", "mcp.json"),
		ManagedDigest: shared.DigestJSONObject(server), ProtectionClass: "managed",
	}, nil
}

func projectedKiroMCPServer(root, name string) (map[string]any, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("prepared Kiro package path is unavailable")
	}
	body, err := os.ReadFile(filepath.Join(root, "mcp.json"))
	if err != nil {
		return nil, fmt.Errorf("read projected Kiro MCP configuration: %w", err)
	}
	document, err := shared.DecodeStrictJSONObject(body)
	if err != nil {
		return nil, fmt.Errorf("decode projected Kiro MCP configuration: %w", err)
	}
	servers, ok := document["mcpServers"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("projected Kiro MCP configuration has no mcpServers object")
	}
	server, ok := servers[name].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("projected Kiro MCP server %q is missing", name)
	}
	return server, nil
}

func ProjectMCP(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataPath string) error {
	return shared.ProjectMCPServers(shared.MCPProjection{
		Root:       root,
		Envelope:   envelope,
		Names:      shared.SupportedMCPNames(plan),
		Dialect:    shared.MCPDialectKiro,
		PluginRoot: plan.ActivePath,
		DataPath:   dataPath,
	})
}
