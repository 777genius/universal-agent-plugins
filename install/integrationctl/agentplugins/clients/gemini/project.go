package gemini

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func BuildNativeObjects(stagingRoot string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, pluginDataPath string) ([]domain.NativeObjectOwnership, error) {
	configRoot := strings.TrimSpace(plan.NativeRegistryRoot)
	if configRoot == "" || !filepath.IsAbs(configRoot) {
		return nil, fmt.Errorf("the Gemini config root is unavailable")
	}
	objects := make([]domain.NativeObjectOwnership, 0, len(envelope.Skills)+len(envelope.MCP.Servers))
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		if err := pathpolicy.ValidateLeafID(component.Name); err != nil {
			return nil, fmt.Errorf("invalid Gemini component name %q: %w", component.Name, err)
		}
		switch component.Kind {
		case domain.ComponentSkill:
			object, err := geminiSkillOwnership(stagingRoot, configRoot, envelope, component)
			if err != nil {
				return nil, err
			}
			objects = append(objects, object)
		case domain.ComponentMCPServer:
			object, err := geminiMCPOwnership(stagingRoot, configRoot, envelope, plan, pluginDataPath, component)
			if err != nil {
				return nil, err
			}
			objects = append(objects, object)
		}
	}
	descriptorBody, err := json.Marshal(geminiDescriptor{DataRoot: pluginDataPath})
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(stagingRoot, geminiDescriptorName), append(descriptorBody, '\n'), 0o600); err != nil {
		return nil, fmt.Errorf("write Gemini projection descriptor: %w", err)
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].ObjectID < objects[j].ObjectID })
	return objects, nil
}

func geminiSkillOwnership(stagingRoot, configRoot string, envelope domain.PackageEnvelope, component domain.ComponentDecision) (domain.NativeObjectOwnership, error) {
	skill, ok := envelope.Skills[component.Name]
	if !ok {
		return domain.NativeObjectOwnership{}, fmt.Errorf("planned Gemini skill %q is missing", component.Name)
	}
	relative := filepath.FromSlash(strings.TrimSpace(skill.RelativePath))
	if relative == "" {
		relative = filepath.Join("skills", component.Name, "SKILL.md")
	}
	sourceRelative := filepath.Dir(relative)
	sourceRoot := filepath.Join(stagingRoot, sourceRelative)
	if err := pathpolicy.RequireContainedChild(stagingRoot, sourceRoot); err != nil {
		return domain.NativeObjectOwnership{}, fmt.Errorf("unsafe Gemini skill source %q: %w", component.Name, err)
	}
	digest, err := shared.DigestSkillDirectory(sourceRoot)
	if err != nil {
		return domain.NativeObjectOwnership{}, fmt.Errorf("digest Gemini skill %q: %w", component.Name, err)
	}
	return domain.NativeObjectOwnership{
		ObjectID: "gemini-skill:" + component.Name, Kind: geminiSkillObjectKind,
		LogicalName: component.Name, Path: filepath.Join(configRoot, "skills", component.Name),
		SourceRelative: filepath.ToSlash(sourceRelative), ManagedDigest: digest, ProtectionClass: "managed",
	}, nil
}

func geminiMCPOwnership(stagingRoot, configRoot string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, pluginDataPath string, component domain.ComponentDecision) (domain.NativeObjectOwnership, error) {
	server, ok := envelope.MCP.Servers[component.Name]
	if !ok {
		return domain.NativeObjectOwnership{}, fmt.Errorf("planned Gemini MCP server %q is missing", component.Name)
	}
	native, err := geminiNativeServer(server)
	if err != nil {
		return domain.NativeObjectOwnership{}, fmt.Errorf("project Gemini MCP server %q: %w", component.Name, err)
	}
	native, err = materializeGeminiServer(native, plan.ActivePath, pluginDataPath, stagingRoot)
	if err != nil {
		return domain.NativeObjectOwnership{}, fmt.Errorf("bind Gemini MCP server %q: %w", component.Name, err)
	}
	receipt, err := nativeconfig.DesiredReceipt(filepath.Join(configRoot, "settings.json"), nativeconfig.CodecGemini, component.Name, native, nativeconfig.Placeholders{PackageRoot: plan.ActivePath, DataRoot: pluginDataPath})
	if err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	return domain.NativeObjectOwnership{
		ObjectID: "gemini-mcp:" + component.Name, Kind: geminiMCPObjectKind,
		LogicalName: component.Name, Path: filepath.Join(configRoot, "settings.json"),
		ManagedDigest: receipt.Digest, ProtectionClass: "managed",
	}, nil
}

func materializeGeminiServer(server nativeconfig.Server, packageRoot, dataRoot string, observationRoot ...string) (nativeconfig.Server, error) {
	if server.Type != "stdio" {
		return server, nil
	}
	command, cwd, err := shared.ResolveStdioPaths(server.Command, server.CWD, packageRoot, dataRoot, observationRoot...)
	if err != nil {
		return server, err
	}
	server.Command, server.CWD = command, cwd
	server.CWDResolved = true
	return server, nil
}

func geminiServerFromPackage(root, name string) (nativeconfig.Server, error) {
	body, err := os.ReadFile(filepath.Join(root, "mcp.json"))
	if err != nil {
		return nativeconfig.Server{}, fmt.Errorf("read Gemini package MCP configuration: %w", err)
	}
	document, err := shared.DecodeStrictJSONObject(body)
	if err != nil {
		return nativeconfig.Server{}, err
	}
	servers, ok := document["mcpServers"].(map[string]any)
	if !ok {
		return nativeconfig.Server{}, fmt.Errorf("the Gemini package has no mcpServers object")
	}
	decoded, ok := servers[name].(map[string]any)
	if !ok {
		return nativeconfig.Server{}, fmt.Errorf("the Gemini MCP server %q is missing", name)
	}
	typeName, _ := decoded["type"].(string)
	return geminiNativeServer(domain.MCPServer{Name: name, Type: typeName, Decoded: decoded})
}
