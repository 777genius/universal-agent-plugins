package opencode

import (
	"encoding/json"
	"fmt"
	"io"
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

func ProjectNative(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataRoot string) error {
	configRoot := strings.TrimSpace(plan.NativeRegistryRoot)
	if configRoot == "" || !filepath.IsAbs(configRoot) {
		return fmt.Errorf("OpenCode config root is unavailable")
	}
	jsonPath, jsoncPath := filepath.Join(configRoot, "opencode.json"), filepath.Join(configRoot, "opencode.jsonc")
	selected, err := selectOpenCodeConfig(jsonPath, jsoncPath)
	if err != nil {
		return err
	}
	projection := openCodeProjection{Version: 1, ConfigPath: selected, ConfigJSON: jsonPath, ConfigJSONC: jsoncPath,
		PackageRoot: plan.ActivePath, DataRoot: dataRoot, MCPServers: map[string]nativeconfig.Server{}, ResolvedCWD: map[string]bool{}}
	for _, component := range plan.Components {
		if component.Kind != domain.ComponentMCPServer || component.Support == domain.SupportUnsupported {
			continue
		}
		server, ok := envelope.MCP.Servers[component.Name]
		if !ok {
			return fmt.Errorf("planned OpenCode MCP server %q is missing", component.Name)
		}
		neutral, err := neutralOpenCodeServer(server)
		if err != nil {
			return fmt.Errorf("project OpenCode MCP server %q: %w", component.Name, err)
		}
		if neutral.Type == "stdio" {
			command, cwd, pathErr := shared.ResolveStdioPaths(neutral.Command, neutral.CWD, plan.ActivePath, dataRoot, root)
			if pathErr != nil {
				return fmt.Errorf("project OpenCode MCP server %q: %w", component.Name, pathErr)
			}
			neutral.Command, neutral.CWD = command, cwd
			projection.ResolvedCWD[component.Name] = true
		}

		projection.MCPServers[component.Name] = neutral
	}
	body, err := json.MarshalIndent(projection, "", "  ")
	if err != nil {
		return err
	}
	projectionPath := filepath.Join(root, openCodeProjectionFile)
	if _, err := os.Lstat(projectionPath); err == nil {
		return fmt.Errorf("package contains reserved OpenCode projection path %q", openCodeProjectionFile)
	} else if !os.IsNotExist(err) {
		return err
	}
	return atomicfile.Write(projectionPath, append(body, '\n'), 0o600)
}

func selectOpenCodeConfig(jsonPath, jsoncPath string) (string, error) {
	jsonExists, err := regularNativeFileExists(jsonPath)
	if err != nil {
		return "", err
	}
	jsoncExists, err := regularNativeFileExists(jsoncPath)
	if err != nil {
		return "", err
	}
	if jsonExists && jsoncExists {
		return "", nativeconfig.ErrAmbiguousConfig
	}
	if jsoncExists {
		return jsoncPath, nil
	}
	return jsonPath, nil
}

func regularNativeFileExists(path string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return false, fmt.Errorf("native config must be a regular file")
	}
	return true, nil
}

func neutralOpenCodeServer(server domain.MCPServer) (nativeconfig.Server, error) {
	switch server.Type {
	case "stdio":
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
		if _, exists := env["PLUGIN_ROOT"]; exists {
			return nativeconfig.Server{}, fmt.Errorf("env: PLUGIN_ROOT is reserved and client-managed")
		}
		if _, exists := env["PLUGIN_DATA"]; exists {
			return nativeconfig.Server{}, fmt.Errorf("env: PLUGIN_DATA is reserved and client-managed")
		}
		if env == nil {
			env = map[string]string{}
		}
		env["PLUGIN_ROOT"] = "${PLUGIN_ROOT}"
		env["PLUGIN_DATA"] = "${PLUGIN_DATA}"
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
	case "sse":
		return nativeconfig.Server{}, fmt.Errorf("internal consistency: declared SSE is not supported by OpenCode (remote starts with Streamable HTTP)")
	case "streamable-http":
		url, ok := server.Decoded["url"].(string)
		if !ok || strings.TrimSpace(url) == "" {
			return nativeconfig.Server{}, fmt.Errorf("remote url is required")
		}
		headers, err := openCodeStringMap(server.Decoded["headers"])
		if err != nil {
			return nativeconfig.Server{}, fmt.Errorf("headers: %w", err)
		}
		return nativeconfig.Server{Type: "remote", URL: url, Headers: headers}, nil
	default:
		return nativeconfig.Server{}, fmt.Errorf("unsupported transport %q", server.Type)
	}
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

func readOpenCodeProjection(root string) (openCodeProjection, error) {
	body, err := os.ReadFile(filepath.Join(root, openCodeProjectionFile))
	if err != nil {
		return openCodeProjection{}, fmt.Errorf("read OpenCode native projection: %w", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	var projection openCodeProjection
	if err := decoder.Decode(&projection); err != nil || projection.Version != 1 {
		return openCodeProjection{}, fmt.Errorf("decode OpenCode native projection")
	}
	if err := requireJSONEOF(decoder); err != nil {
		return openCodeProjection{}, err
	}
	if !filepath.IsAbs(projection.ConfigPath) || !filepath.IsAbs(projection.ConfigJSON) || !filepath.IsAbs(projection.ConfigJSONC) || !filepath.IsAbs(projection.PackageRoot) {
		return openCodeProjection{}, fmt.Errorf("OpenCode projection contains relative paths")
	}
	if projection.ConfigPath != projection.ConfigJSON && projection.ConfigPath != projection.ConfigJSONC {
		return openCodeProjection{}, fmt.Errorf("OpenCode projection selects an unexpected config path")
	}
	for name, resolved := range projection.ResolvedCWD {
		server, ok := projection.MCPServers[name]
		if !ok || !resolved || server.Type != "stdio" || !filepath.IsAbs(server.CWD) {
			return openCodeProjection{}, fmt.Errorf("invalid resolved OpenCode cwd provenance")
		}
		server.CWDResolved = true
		projection.MCPServers[name] = server
	}
	return projection, nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("native projection has trailing JSON")
	}
	return nil
}

func BuildNativeObjects(stagingRoot string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) ([]domain.NativeObjectOwnership, error) {
	projection, err := readOpenCodeProjection(stagingRoot)
	if err != nil {
		return nil, err
	}
	objects := make([]domain.NativeObjectOwnership, 0, len(projection.MCPServers)+len(envelope.Skills))
	placeholders := nativeconfig.Placeholders{PackageRoot: projection.PackageRoot, DataRoot: projection.DataRoot}
	for name, server := range projection.MCPServers {
		receipt, err := nativeconfig.DesiredReceipt(projection.ConfigPath, nativeconfig.CodecOpenCode, name, server, placeholders)
		if err != nil {
			return nil, err
		}
		objects = append(objects, domain.NativeObjectOwnership{ObjectID: "opencode-mcp:" + name, Kind: openCodeMCPObjectKind,
			LogicalName: name, Path: receipt.Path, ManagedDigest: receipt.Digest, ProtectionClass: "managed"})
	}
	for _, component := range plan.Components {
		if component.Kind != domain.ComponentSkill || component.Support == domain.SupportUnsupported {
			continue
		}
		skill, ok := envelope.Skills[component.Name]
		if !ok {
			return nil, fmt.Errorf("planned OpenCode skill %q is missing", component.Name)
		}
		if err := pathpolicy.ValidateLeafID(component.Name); err != nil {
			return nil, fmt.Errorf("invalid OpenCode skill name %q: %w", component.Name, err)
		}
		relative := filepath.FromSlash(strings.TrimSpace(skill.RelativePath))
		if relative == "" {
			relative = filepath.Join("skills", component.Name, "SKILL.md")
		}
		source := filepath.Join(stagingRoot, filepath.Dir(relative))
		if err := pathpolicy.RequireContainedChild(stagingRoot, source); err != nil {
			return nil, err
		}
		digest, err := shared.DigestSkillDirectory(source)
		if err != nil {
			return nil, err
		}
		target := filepath.Join(plan.NativeRegistryRoot, "skills", component.Name)
		if err := pathpolicy.RequireContainedChild(plan.NativeRegistryRoot, target); err != nil {
			return nil, err
		}
		objects = append(objects, domain.NativeObjectOwnership{ObjectID: "opencode-skill:" + component.Name, Kind: openCodeSkillKind,
			LogicalName: component.Name, Path: target, SourceRelative: filepath.ToSlash(filepath.Dir(relative)), ManagedDigest: digest, ProtectionClass: "managed"})
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].ObjectID < objects[j].ObjectID })
	return objects, nil
}
