package cline

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

func ProjectNative(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataPath string) error {
	servers := make(map[string]nativeconfig.Server)
	for _, name := range shared.SupportedMCPNames(plan) {
		portable := envelope.MCP.Servers[name]
		if portable.Type == "stdio" {
			portable.Decoded = shared.CloneObject(portable.Decoded)
			if err := shared.ApplyStdioDataContract(portable.Decoded, plan.ActivePath, dataPath, root); err != nil {
				return fmt.Errorf("project Cline MCP server %s: %w", name, err)
			}
		}
		server, err := clineNeutralServer(portable)
		if err != nil {
			return fmt.Errorf("project Cline MCP server %s: %w", name, err)
		}
		server.StdioValuesResolved = portable.Type == "stdio"
		// This first pass validates the codec without touching client state. The
		// exact configured path is bound later when ownership objects are built.
		settingsPath := filepath.Join(root, ".cline-settings-validation.json")
		if _, err := nativeconfig.DesiredReceipt(settingsPath, nativeconfig.CodecCline, name, server, nativeconfig.Placeholders{PackageRoot: plan.ActivePath, DataRoot: dataPath}); err != nil {
			return fmt.Errorf("project Cline MCP server %s: %w", name, err)
		}
		servers[name] = server
	}
	if len(servers) == 0 {
		return nil
	}
	return shared.WriteJSON(filepath.Join(root, clineProjectionFile), clineProjection{Servers: servers})
}

func BuildNativeObjects(stagingRoot string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) ([]domain.NativeObjectOwnership, error) {
	root := strings.TrimSpace(plan.NativeRegistryRoot)
	if root == "" || !filepath.IsAbs(root) {
		return nil, fmt.Errorf("Cline config root is unavailable")
	}
	objects := make([]domain.NativeObjectOwnership, 0, len(envelope.Skills)+len(envelope.MCP.Servers))
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		if err := pathpolicy.ValidateLeafID(component.Name); err != nil {
			return nil, fmt.Errorf("invalid Cline component name %q: %w", component.Name, err)
		}
		switch component.Kind {
		case domain.ComponentSkill:
			skill, ok := envelope.Skills[component.Name]
			if !ok {
				return nil, fmt.Errorf("planned Cline skill %q is missing", component.Name)
			}
			relative := filepath.FromSlash(strings.TrimSpace(skill.RelativePath))
			if relative == "" {
				relative = filepath.Join("skills", component.Name, "SKILL.md")
			}
			sourceRoot := filepath.Join(stagingRoot, filepath.Dir(relative))
			if err := pathpolicy.RequireContainedChild(stagingRoot, sourceRoot); err != nil {
				return nil, err
			}
			digest, err := shared.DigestSkillDirectory(sourceRoot)
			if err != nil {
				return nil, err
			}
			objects = append(objects, domain.NativeObjectOwnership{
				ObjectID: "cline-skill:" + component.Name, Kind: clineSkillObjectKind,
				LogicalName: component.Name, Path: filepath.Join(root, "skills", component.Name),
				SourceRelative: filepath.ToSlash(filepath.Dir(relative)), ManagedDigest: digest,
				ProtectionClass: "managed",
			})
		case domain.ComponentMCPServer:
			projection, err := readClineProjection(stagingRoot)
			if err != nil {
				return nil, err
			}
			server, ok := projection.Servers[component.Name]
			if !ok {
				return nil, fmt.Errorf("projected Cline MCP server %q is missing", component.Name)
			}
			settingsPath := clineMCPSettingsPath(root)
			if !filepath.IsAbs(settingsPath) {
				return nil, fmt.Errorf("Cline MCP settings path must be absolute")
			}
			receipt, err := nativeconfig.DesiredReceipt(settingsPath, nativeconfig.CodecCline, component.Name, server, nativeconfig.Placeholders{})
			if err != nil {
				return nil, err
			}
			objects = append(objects, domain.NativeObjectOwnership{
				ObjectID: "cline-mcp:" + component.Name, Kind: clineMCPObjectKind,
				LogicalName: component.Name, Path: settingsPath, ManagedDigest: receipt.Digest,
				SourceRelative: clineProjectionFile, ProtectionClass: "managed",
			})
		}
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].ObjectID < objects[j].ObjectID })
	return objects, nil
}

func readClineProjection(root string) (clineProjection, error) {
	body, err := os.ReadFile(filepath.Join(root, clineProjectionFile))
	if err != nil {
		return clineProjection{}, fmt.Errorf("read projected Cline MCP configuration: %w", err)
	}
	var projection clineProjection
	if err := json.Unmarshal(body, &projection); err != nil || projection.Servers == nil {
		return clineProjection{}, fmt.Errorf("decode projected Cline MCP configuration: %w", err)
	}
	for name, server := range projection.Servers {
		server.StdioValuesResolved = server.Type == "stdio"
		projection.Servers[name] = server
	}
	return projection, nil
}

func clineNeutralServer(server domain.MCPServer) (nativeconfig.Server, error) {
	getString := func(key string) string {
		value, _ := server.Decoded[key].(string)
		return value
	}
	result := nativeconfig.Server{Type: server.Type, Command: getString("command"), URL: getString("url")}
	if server.Type == "stdio" {
		if rawCWD, exists := server.Decoded["cwd"]; exists {
			cwd, ok := rawCWD.(string)
			if !ok {
				return result, fmt.Errorf("Cline stdio MCP server cwd must be a string")
			}
			result.CWD = cwd
		}
	}
	if server.Type == "streamable-http" || server.Type == "sse" {
		result.Type = "remote"
		result.RemoteTransport = server.Type
	}
	if values, ok := server.Decoded["args"].([]any); ok {
		for _, value := range values {
			text, ok := value.(string)
			if !ok {
				return result, fmt.Errorf("args must be strings")
			}
			result.Args = append(result.Args, text)
		}
	} else if values, ok := server.Decoded["args"].([]string); ok {
		result.Args = append([]string(nil), values...)
	}
	copyMap := func(key string) (map[string]string, error) {
		result := map[string]string{}
		switch values := server.Decoded[key].(type) {
		case nil:
			return nil, nil
		case map[string]any:
			for name, value := range values {
				text, ok := value.(string)
				if !ok {
					return nil, fmt.Errorf("%s values must be strings", key)
				}
				result[name] = text
			}
		case map[string]string:
			for name, value := range values {
				result[name] = value
			}
		default:
			return nil, fmt.Errorf("%s must be an object", key)
		}
		return result, nil
	}
	var err error
	result.Env, err = copyMap("env")
	if err != nil {
		return result, err
	}
	result.Headers, err = copyMap("headers")
	return result, err
}
