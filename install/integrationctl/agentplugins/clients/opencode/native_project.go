package opencode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func ProjectOpenCodeNative(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataRoot string) error {
	configRoot := strings.TrimSpace(plan.NativeRegistryRoot)
	if configRoot == "" || !filepath.IsAbs(configRoot) {
		return fmt.Errorf("OpenCode config root is unavailable")
	}
	jsonPath, jsoncPath := filepath.Join(configRoot, "opencode.json"), filepath.Join(configRoot, "opencode.jsonc")
	selected, err := selectOpenCodeConfig(jsonPath, jsoncPath)
	if err != nil {
		return err
	}
	projection := OpenCodeProjection{Version: 1, ConfigPath: selected, ConfigJSON: jsonPath, ConfigJSONC: jsoncPath,
		PackageRoot: plan.ActivePath, DataRoot: dataRoot, MCPServers: map[string]nativeconfig.Server{}, ResolvedCWD: map[string]bool{}}
	if err := projectOpenCodeMCPServers(&projection, root, envelope, plan, dataRoot); err != nil {
		return err
	}
	return writeOpenCodeProjection(root, projection)
}

func projectOpenCodeMCPServers(projection *OpenCodeProjection, root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataRoot string) error {
	for _, component := range plan.Components {
		if component.Kind != domain.ComponentMCPServer || component.Support == domain.SupportUnsupported {
			continue
		}
		server, ok := envelope.MCP.Servers[component.Name]
		if !ok {
			return fmt.Errorf("planned OpenCode MCP server %q is missing", component.Name)
		}
		neutral, err := NeutralOpenCodeServer(server)
		if err != nil {
			return fmt.Errorf("project OpenCode MCP server %q: %w", component.Name, err)
		}
		if err := bindOpenCodeStdioServer(projection, component.Name, &neutral, plan.ActivePath, dataRoot, root); err != nil {
			return fmt.Errorf("project OpenCode MCP server %q: %w", component.Name, err)
		}
		projection.MCPServers[component.Name] = neutral
	}
	return nil
}

func bindOpenCodeStdioServer(projection *OpenCodeProjection, name string, server *nativeconfig.Server, packageRoot, dataRoot, root string) error {
	if server.Type != "stdio" {
		return nil
	}
	command, cwd, err := shared.ResolveStdioPaths(server.Command, server.CWD, packageRoot, dataRoot, root)
	if err != nil {
		return err
	}
	server.Command, server.CWD = command, cwd
	projection.ResolvedCWD[name] = true
	return nil
}

func writeOpenCodeProjection(root string, projection OpenCodeProjection) error {
	body, err := json.MarshalIndent(projection, "", "  ")
	if err != nil {
		return err
	}
	projectionPath := filepath.Join(root, OpenCodeProjectionFile)
	if _, err := os.Lstat(projectionPath); err == nil {
		return fmt.Errorf("package contains reserved OpenCode projection path %q", OpenCodeProjectionFile)
	} else if !os.IsNotExist(err) {
		return err
	}
	return atomicfile.Write(projectionPath, append(body, '\n'), 0o600)
}

func NeutralOpenCodeServer(server domain.MCPServer) (nativeconfig.Server, error) {
	switch server.Type {
	case "stdio":
		return neutralOpenCodeStdioServer(server)
	case "sse":
		return nativeconfig.Server{}, fmt.Errorf("internal consistency: declared SSE is not supported by OpenCode (remote starts with Streamable HTTP)")
	case "streamable-http":
		return neutralOpenCodeRemoteServer(server)
	default:
		return nativeconfig.Server{}, fmt.Errorf("unsupported transport %q", server.Type)
	}
}

func neutralOpenCodeStdioServer(server domain.MCPServer) (nativeconfig.Server, error) {
	command, ok := server.Decoded["command"].(string)
	if !ok || strings.TrimSpace(command) == "" {
		return nativeconfig.Server{}, fmt.Errorf("stdio command is required")
	}
	args, err := openCodeStrings(server.Decoded["args"])
	if err != nil {
		return nativeconfig.Server{}, fmt.Errorf("args: %w", err)
	}
	env, err := openCodeStringMap(server.Decoded["env"])
	if err != nil {
		return nativeconfig.Server{}, fmt.Errorf("env: %w", err)
	}
	env, err = reservedOpenCodeEnv(env)
	if err != nil {
		return nativeconfig.Server{}, err
	}
	cwd, err := openCodeOptionalString(server.Decoded["cwd"])
	if err != nil {
		return nativeconfig.Server{}, fmt.Errorf("cwd: %w", err)
	}
	cwd, err = normalizeOpenCodeCWD(cwd)
	if err != nil {
		return nativeconfig.Server{}, fmt.Errorf("cwd: %w", err)
	}
	if strings.TrimSpace(cwd) == "" {
		cwd = "${PLUGIN_ROOT}"
	}
	return nativeconfig.Server{Type: "stdio", Command: command, Args: args, Env: env, CWD: cwd}, nil
}

func reservedOpenCodeEnv(env map[string]string) (map[string]string, error) {
	if _, exists := env["PLUGIN_ROOT"]; exists {
		return nil, fmt.Errorf("env: PLUGIN_ROOT is reserved and client-managed")
	}
	if _, exists := env["PLUGIN_DATA"]; exists {
		return nil, fmt.Errorf("env: PLUGIN_DATA is reserved and client-managed")
	}
	if env == nil {
		env = map[string]string{}
	}
	env["PLUGIN_ROOT"] = "${PLUGIN_ROOT}"
	env["PLUGIN_DATA"] = "${PLUGIN_DATA}"
	return env, nil
}

func neutralOpenCodeRemoteServer(server domain.MCPServer) (nativeconfig.Server, error) {
	url, ok := server.Decoded["url"].(string)
	if !ok || strings.TrimSpace(url) == "" {
		return nativeconfig.Server{}, fmt.Errorf("remote url is required")
	}
	headers, err := openCodeStringMap(server.Decoded["headers"])
	if err != nil {
		return nativeconfig.Server{}, fmt.Errorf("headers: %w", err)
	}
	return nativeconfig.Server{Type: "remote", URL: url, Headers: headers}, nil
}

func openCodeOptionalString(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("must be a string")
	}
	return text, nil
}

func normalizeOpenCodeCWD(value string) (string, error) {
	parsed, err := pathcontract.ParseCWD(value)
	if err != nil {
		return "", err
	}
	root := "${PLUGIN_ROOT}"
	if parsed.Anchor == pathcontract.Data {
		root = "${PLUGIN_DATA}"
	}
	if parsed.Relative != "" {
		root += "/" + parsed.Relative
	}
	return root, nil
}

func openCodeStrings(value any) ([]string, error) {
	if value == nil {
		return nil, nil
	}
	if items, ok := value.([]string); ok {
		return append([]string(nil), items...), nil
	}
	values, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("must be an array")
	}
	result := make([]string, len(values))
	for index, value := range values {
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("item %d must be a string", index)
		}
		result[index] = text
	}
	return result, nil
}

func openCodeStringMap(value any) (map[string]string, error) {
	if value == nil {
		return nil, nil
	}
	if values, ok := value.(map[string]string); ok {
		result := make(map[string]string, len(values))
		for key, value := range values {
			result[key] = value
		}
		return result, nil
	}
	values, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("must be an object")
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("value for %q must be a string", key)
		}
		result[key] = text
	}
	return result, nil
}

func BuildOpenCodeNativeObjects(stagingRoot string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) ([]domain.NativeObjectOwnership, error) {
	projection, err := ReadOpenCodeProjection(stagingRoot)
	if err != nil {
		return nil, err
	}
	objects, err := openCodeMCPOwnerships(projection)
	if err != nil {
		return nil, err
	}
	skills, err := openCodeSkillOwnerships(stagingRoot, envelope, plan)
	if err != nil {
		return nil, err
	}
	objects = append(objects, skills...)
	sort.Slice(objects, func(i, j int) bool { return objects[i].ObjectID < objects[j].ObjectID })
	return objects, nil
}

func openCodeMCPOwnerships(projection OpenCodeProjection) ([]domain.NativeObjectOwnership, error) {
	objects := make([]domain.NativeObjectOwnership, 0, len(projection.MCPServers))
	placeholders := nativeconfig.Placeholders{PackageRoot: projection.PackageRoot, DataRoot: projection.DataRoot}
	for name, server := range projection.MCPServers {
		receipt, err := nativeconfig.DesiredReceipt(projection.ConfigPath, nativeconfig.CodecOpenCode, name, server, placeholders)
		if err != nil {
			return nil, err
		}
		objects = append(objects, domain.NativeObjectOwnership{ObjectID: "opencode-mcp:" + name, Kind: OpenCodeMCPObjectKind,
			LogicalName: name, Path: receipt.Path, ManagedDigest: receipt.Digest, ProtectionClass: "managed"})
	}
	return objects, nil
}

func openCodeSkillOwnerships(stagingRoot string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) ([]domain.NativeObjectOwnership, error) {
	var objects []domain.NativeObjectOwnership
	for _, component := range plan.Components {
		if component.Kind != domain.ComponentSkill || component.Support == domain.SupportUnsupported {
			continue
		}
		object, err := openCodeSkillOwnership(stagingRoot, envelope, plan, component)
		if err != nil {
			return nil, err
		}
		objects = append(objects, object)
	}
	return objects, nil
}

func openCodeSkillOwnership(stagingRoot string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, component domain.ComponentDecision) (domain.NativeObjectOwnership, error) {
	skill, ok := envelope.Skills[component.Name]
	if !ok {
		return domain.NativeObjectOwnership{}, fmt.Errorf("planned OpenCode skill %q is missing", component.Name)
	}
	if err := pathpolicy.ValidateLeafID(component.Name); err != nil {
		return domain.NativeObjectOwnership{}, fmt.Errorf("invalid OpenCode skill name %q: %w", component.Name, err)
	}
	relative := filepath.FromSlash(strings.TrimSpace(skill.RelativePath))
	if relative == "" {
		relative = filepath.Join("skills", component.Name, "SKILL.md")
	}
	source := filepath.Join(stagingRoot, filepath.Dir(relative))
	if err := pathpolicy.RequireContainedChild(stagingRoot, source); err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	digest, err := shared.DigestSkillDirectory(source)
	if err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	target := filepath.Join(plan.NativeRegistryRoot, "skills", component.Name)
	if err := pathpolicy.RequireContainedChild(plan.NativeRegistryRoot, target); err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	return domain.NativeObjectOwnership{ObjectID: "opencode-skill:" + component.Name, Kind: openCodeSkillKind,
		LogicalName: component.Name, Path: target, SourceRelative: filepath.ToSlash(filepath.Dir(relative)), ManagedDigest: digest, ProtectionClass: "managed"}, nil
}
