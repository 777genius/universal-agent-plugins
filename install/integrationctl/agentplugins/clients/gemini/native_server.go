package gemini

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func GeminiNativeServer(server domain.MCPServer) (nativeconfig.Server, error) {
	switch server.Type {
	case "stdio":
		return geminiNativeStdioServer(server)
	case "streamable-http", "sse":
		return geminiNativeRemoteServer(server)
	default:
		return nativeconfig.Server{}, fmt.Errorf("unsupported transport %q", server.Type)
	}
}

func geminiDecodedString(decoded map[string]any, key string) (string, error) {
	value, exists := decoded[key]
	if !exists {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	return text, nil
}

func geminiNativeStdioServer(server domain.MCPServer) (nativeconfig.Server, error) {
	result := nativeconfig.Server{Type: "stdio"}
	var err error
	result.Command, err = geminiDecodedString(server.Decoded, "command")
	if err != nil {
		return result, err
	}
	result.CWD, err = geminiDecodedString(server.Decoded, "cwd")
	if err != nil {
		return result, err
	}
	if strings.HasPrefix(result.CWD, "./") {
		result.CWD = "${PLUGIN_ROOT}/" + strings.TrimPrefix(result.CWD, "./")
	}
	if result.CWD == "" {
		result.CWD = "${PLUGIN_ROOT}"
	}
	if err := geminiCopyStdioArgs(server.Decoded, &result); err != nil {
		return result, err
	}
	if err := geminiCopyStdioEnv(server.Decoded, &result); err != nil {
		return result, err
	}
	return result, nil
}

func geminiCopyStdioArgs(decoded map[string]any, result *nativeconfig.Server) error {
	raw, ok := decoded["args"]
	if !ok {
		return nil
	}
	values, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("args must be an array")
	}
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("args must contain strings")
		}
		result.Args = append(result.Args, text)
	}
	return nil
}

func geminiCopyStdioEnv(decoded map[string]any, result *nativeconfig.Server) error {
	if raw, ok := decoded["env"]; ok {
		values, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("env must be an object")
		}
		result.Env = map[string]string{}
		for key, value := range values {
			text, ok := value.(string)
			if !ok {
				return fmt.Errorf("env values must be strings")
			}
			result.Env[key] = text
		}
	}
	if result.Env == nil {
		result.Env = map[string]string{}
	}
	result.Env["PLUGIN_ROOT"] = "${PLUGIN_ROOT}"
	result.Env["PLUGIN_DATA"] = "${PLUGIN_DATA}"
	return nil
}

func geminiNativeRemoteServer(server domain.MCPServer) (nativeconfig.Server, error) {
	result := nativeconfig.Server{Type: "remote", RemoteTransport: server.Type}
	var err error
	result.URL, err = geminiDecodedString(server.Decoded, "url")
	if err != nil {
		return result, err
	}
	raw, ok := server.Decoded["headers"]
	if !ok {
		return result, nil
	}
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
	return result, nil
}

func MaterializeGeminiServer(server nativeconfig.Server, packageRoot, dataRoot string, observationRoot ...string) (nativeconfig.Server, error) {
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
	return GeminiNativeServer(domain.MCPServer{Name: name, Type: typeName, Decoded: decoded})
}
