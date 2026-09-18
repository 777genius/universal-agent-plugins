package windsurf

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
)

func windsurfServer(server domain.MCPServer, packageRoot, dataRoot string) (nativeconfig.Server, error) {
	var result nativeconfig.Server
	var err error
	switch server.Type {
	case "stdio":
		result, err = windsurfStdioServer(server.Decoded, packageRoot, dataRoot)
	case "streamable-http", "sse":
		result, err = windsurfRemoteServer(server.Type, server.Decoded)
	default:
		return nativeconfig.Server{}, fmt.Errorf("unsupported Windsurf MCP transport %q", server.Type)
	}
	if err != nil {
		return result, err
	}
	resolved, err := resolveWindsurfPlaceholders(result, packageRoot, dataRoot)
	if err != nil {
		return result, err
	}
	if resolved.Type != "stdio" {
		return resolved, nil
	}
	return bindWindsurfStdioLauncher(server.Decoded, resolved, packageRoot, dataRoot)
}

func windsurfStdioServer(decoded map[string]any, packageRoot, dataRoot string) (nativeconfig.Server, error) {
	result := nativeconfig.Server{Type: "stdio"}
	command, ok := decoded["command"].(string)
	if !ok || strings.TrimSpace(command) == "" {
		return result, fmt.Errorf("stdio command is required")
	}
	result.Command = command
	if err := copyWindsurfStdioArgs(decoded, &result); err != nil {
		return result, err
	}
	if err := copyWindsurfStdioEnv(decoded, &result, packageRoot, dataRoot); err != nil {
		return result, err
	}
	return result, nil
}

func copyWindsurfStdioArgs(decoded map[string]any, result *nativeconfig.Server) error {
	rawArgs, ok := decoded["args"].([]any)
	if !ok {
		return nil
	}
	for _, raw := range rawArgs {
		value, ok := raw.(string)
		if !ok {
			return fmt.Errorf("stdio args must contain strings")
		}
		result.Args = append(result.Args, value)
	}
	return nil
}

func copyWindsurfStdioEnv(decoded map[string]any, result *nativeconfig.Server, packageRoot, dataRoot string) error {
	rawEnv, exists := decoded["env"]
	if exists {
		result.Env = map[string]string{}
		switch values := rawEnv.(type) {
		case map[string]any:
			for key, raw := range values {
				value, ok := raw.(string)
				if !ok {
					return fmt.Errorf("stdio env values must be strings")
				}
				result.Env[key] = value
			}
		case map[string]string:
			for key, value := range values {
				result.Env[key] = value
			}
		default:
			return fmt.Errorf("stdio env must be an object")
		}
	}
	if _, exists := result.Env["PLUGIN_ROOT"]; exists {
		return fmt.Errorf("stdio env PLUGIN_ROOT is reserved and client-managed")
	}
	if _, exists := result.Env["PLUGIN_DATA"]; exists {
		return fmt.Errorf("stdio env PLUGIN_DATA is reserved and client-managed")
	}
	if result.Env == nil {
		result.Env = map[string]string{}
	}
	result.Env["PLUGIN_ROOT"] = packageRoot
	result.Env["PLUGIN_DATA"] = dataRoot
	return nil
}

func windsurfRemoteServer(serverType string, decoded map[string]any) (nativeconfig.Server, error) {
	result := nativeconfig.Server{Type: "remote", RemoteTransport: serverType}
	url, ok := decoded["url"].(string)
	if !ok || strings.TrimSpace(url) == "" {
		return result, fmt.Errorf("remote url is required")
	}
	result.URL = url
	rawHeaders, ok := decoded["headers"].(map[string]any)
	if !ok {
		return result, nil
	}
	result.Headers = map[string]string{}
	for key, raw := range rawHeaders {
		value, ok := raw.(string)
		if !ok {
			return result, fmt.Errorf("remote header values must be strings")
		}
		result.Headers[key] = value
	}
	return result, nil
}

func bindWindsurfStdioLauncher(decoded map[string]any, resolved nativeconfig.Server, packageRoot, dataRoot string) (nativeconfig.Server, error) {
	if !filepath.IsAbs(packageRoot) || !filepath.IsAbs(dataRoot) {
		return resolved, fmt.Errorf("managed stdio roots must be absolute")
	}
	rawCWD := ""
	if value, exists := decoded["cwd"]; exists {
		var ok bool
		rawCWD, ok = value.(string)
		if !ok {
			return resolved, fmt.Errorf("stdio cwd must be a string")
		}
	}
	cwd, err := pathcontract.ExpandCWD(rawCWD, packageRoot, dataRoot)
	if err != nil {
		return resolved, err
	}
	root := packageRoot
	if cwd.Anchor == pathcontract.Data {
		root = dataRoot
	}
	absoluteCWD := root
	if cwd.Relative != "" {
		absoluteCWD = strings.TrimRight(root, string(filepath.Separator)) + string(filepath.Separator) + filepath.FromSlash(cwd.Relative)
	}
	resolved.StdioValuesResolved = true
	resolved.Args = managedstdio.Arguments(packageRoot, dataRoot, absoluteCWD, cwd.Anchor, commandForLauncher(decoded), resolved.Args)
	resolved.Command = filepath.Join(packageRoot, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)
	return resolved, nil
}
