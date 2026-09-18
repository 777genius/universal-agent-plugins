package kiro

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.Projector = (*Adapter)(nil)

// Project writes Kiro's MCP document and records the native objects it owns.
func (*Adapter) Project(_ context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	if err := shared.ProjectMCPServers(shared.MCPProjection{
		Root:       in.StagingPath,
		Envelope:   in.Envelope,
		Names:      shared.SupportedMCPNames(in.Plan),
		Dialect:    shared.MCPDialectKiro,
		PluginRoot: in.Plan.ActivePath,
		DataPath:   in.PluginDataPath,
	}); err != nil {
		return nil, err
	}
	return BuildNativeObjects(in.StagingPath, in.Envelope, in.Plan)
}

func BuildNativeObjects(stagingRoot string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) ([]domain.NativeObjectOwnership, error) {
	configRoot := strings.TrimSpace(plan.NativeRegistryRoot)
	if configRoot == "" {
		return nil, kiroErrorf("Kiro config root is unavailable")
	}
	objects := make([]domain.NativeObjectOwnership, 0, len(envelope.Skills)+len(envelope.MCP.Servers))
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		switch component.Kind {
		case domain.ComponentSkill:
			object, err := buildKiroSkillObject(stagingRoot, configRoot, envelope, component)
			if err != nil {
				return nil, err
			}
			objects = append(objects, object)
		case domain.ComponentMCPServer:
			object, err := buildKiroMCPObject(stagingRoot, configRoot, component)
			if err != nil {
				return nil, err
			}
			objects = append(objects, object)
		}
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].ObjectID < objects[j].ObjectID })
	return objects, nil
}

func buildKiroSkillObject(stagingRoot, configRoot string, envelope domain.PackageEnvelope, component domain.ComponentDecision) (domain.NativeObjectOwnership, error) {
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

func buildKiroMCPObject(stagingRoot, configRoot string, component domain.ComponentDecision) (domain.NativeObjectOwnership, error) {
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
