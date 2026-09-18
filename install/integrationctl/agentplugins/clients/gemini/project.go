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
		return nil, fmt.Errorf("Gemini config root is unavailable")
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
			skill, ok := envelope.Skills[component.Name]
			if !ok {
				return nil, fmt.Errorf("planned Gemini skill %q is missing", component.Name)
			}
			relative := filepath.FromSlash(strings.TrimSpace(skill.RelativePath))
			if relative == "" {
				relative = filepath.Join("skills", component.Name, "SKILL.md")
			}
			sourceRelative := filepath.Dir(relative)
			sourceRoot := filepath.Join(stagingRoot, sourceRelative)
			if err := pathpolicy.RequireContainedChild(stagingRoot, sourceRoot); err != nil {
				return nil, fmt.Errorf("unsafe Gemini skill source %q: %w", component.Name, err)
			}
			digest, err := shared.DigestSkillDirectory(sourceRoot)
			if err != nil {
				return nil, fmt.Errorf("digest Gemini skill %q: %w", component.Name, err)
			}
			objects = append(objects, domain.NativeObjectOwnership{
				ObjectID: "gemini-skill:" + component.Name, Kind: geminiSkillObjectKind,
				LogicalName: component.Name, Path: filepath.Join(configRoot, "skills", component.Name),
				SourceRelative: filepath.ToSlash(sourceRelative), ManagedDigest: digest, ProtectionClass: "managed",
			})
		case domain.ComponentMCPServer:
			server, ok := envelope.MCP.Servers[component.Name]
			if !ok {
				return nil, fmt.Errorf("planned Gemini MCP server %q is missing", component.Name)
			}
			native, err := geminiNativeServer(server)
			if err != nil {
				return nil, fmt.Errorf("project Gemini MCP server %q: %w", component.Name, err)
			}
			native, err = materializeGeminiServer(native, plan.ActivePath, pluginDataPath, stagingRoot)
			if err != nil {
				return nil, fmt.Errorf("bind Gemini MCP server %q: %w", component.Name, err)
			}
			receipt, err := nativeconfig.DesiredReceipt(filepath.Join(configRoot, "settings.json"), nativeconfig.CodecGemini, component.Name, native, nativeconfig.Placeholders{PackageRoot: plan.ActivePath, DataRoot: pluginDataPath})
			if err != nil {
				return nil, err
			}
			objects = append(objects, domain.NativeObjectOwnership{
				ObjectID: "gemini-mcp:" + component.Name, Kind: geminiMCPObjectKind,
				LogicalName: component.Name, Path: filepath.Join(configRoot, "settings.json"),
				ManagedDigest: receipt.Digest, ProtectionClass: "managed",
			})
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

func geminiNativeServer(server domain.MCPServer) (nativeconfig.Server, error) {
	getString := func(key string) (string, error) {
		value, exists := server.Decoded[key]
		if !exists {
			return "", nil
		}
		text, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("%s must be a string", key)
		}
		return text, nil
	}
	result := nativeconfig.Server{}
	switch server.Type {
	case "stdio":
		result.Type = "stdio"
		var err error
		result.Command, err = getString("command")
		if err != nil {
			return result, err
		}
		result.CWD, err = getString("cwd")
		if err != nil {
			return result, err
		}
		if strings.HasPrefix(result.CWD, "./") {
			result.CWD = "${PLUGIN_ROOT}/" + strings.TrimPrefix(result.CWD, "./")
		}
		if result.CWD == "" {
			result.CWD = "${PLUGIN_ROOT}"
		}
		if raw, ok := server.Decoded["args"]; ok {
			values, ok := raw.([]any)
			if !ok {
				return result, fmt.Errorf("args must be an array")
			}
			for _, value := range values {
				text, ok := value.(string)
				if !ok {
					return result, fmt.Errorf("args must contain strings")
				}
				result.Args = append(result.Args, text)
			}
		}
		if raw, ok := server.Decoded["env"]; ok {
			values, ok := raw.(map[string]any)
			if !ok {
				return result, fmt.Errorf("env must be an object")
			}
			result.Env = map[string]string{}
			for key, value := range values {
				text, ok := value.(string)
				if !ok {
					return result, fmt.Errorf("env values must be strings")
				}
				result.Env[key] = text
			}
		}
		if result.Env == nil {
			result.Env = map[string]string{}
		}
		result.Env["PLUGIN_ROOT"] = "${PLUGIN_ROOT}"
		result.Env["PLUGIN_DATA"] = "${PLUGIN_DATA}"
	case "streamable-http", "sse":
		result.Type, result.RemoteTransport = "remote", server.Type
		var err error
		result.URL, err = getString("url")
		if err != nil {
			return result, err
		}
		if raw, ok := server.Decoded["headers"]; ok {
			values, ok := raw.(map[string]any)
			if !ok {
				return result, fmt.Errorf("headers must be an object")
			}
			result.Headers = map[string]string{}
			for key, value := range values {
				text, ok := value.(string)
				if !ok {
					return result, fmt.Errorf("header values must be strings")
				}
				result.Headers[key] = text
			}
		}
	default:
		return result, fmt.Errorf("unsupported transport %q", server.Type)
	}
	return result, nil
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
		return nativeconfig.Server{}, fmt.Errorf("Gemini package has no mcpServers object")
	}
	decoded, ok := servers[name].(map[string]any)
	if !ok {
		return nativeconfig.Server{}, fmt.Errorf("Gemini MCP server %q is missing", name)
	}
	typeName, _ := decoded["type"].(string)
	return geminiNativeServer(domain.MCPServer{Name: name, Type: typeName, Decoded: decoded})
}
