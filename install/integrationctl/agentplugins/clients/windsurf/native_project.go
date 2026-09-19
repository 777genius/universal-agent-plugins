package windsurf

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
)

func ProjectWindsurfMCP(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataPath string) error {
	if !shared.HasSupportedMCP(plan.Components) {
		return nil
	}
	servers := make(map[string]any)
	for _, name := range shared.SupportedMCPNames(plan) {
		server, ok := envelope.MCP.Servers[name]
		if !ok {
			return fmt.Errorf("the Windsurf MCP server %q is missing from the package envelope", name)
		}
		resolved, err := WindsurfServer(server, plan.ActivePath, dataPath)
		if err != nil {
			return fmt.Errorf("project Windsurf MCP server %q: %w", name, err)
		}
		if server.Type == "stdio" {
			if err := observeWindsurfStdio(root, dataPath, plan.ActivePath, server); err != nil {
				return err
			}
		}
		projected, err := StandardWindsurfServer(resolved, server.Type)
		if err != nil {
			return fmt.Errorf("project Windsurf MCP server %q: %w", name, err)
		}
		servers[name] = projected
	}
	return shared.WriteJSON(filepath.Join(root, "mcp.json"), map[string]any{
		"$schema":    domain.MCPSchemaV1,
		"mcpServers": servers,
	})
}

func observeWindsurfStdio(root, dataPath, activePath string, server domain.MCPServer) error {
	if err := observeWindsurfStdioCommand(root, server); err != nil {
		return err
	}
	return observeWindsurfStdioCWD(root, dataPath, activePath, server)
}

func observeWindsurfStdioCommand(root string, server domain.MCPServer) error {
	command, _ := server.Decoded["command"].(string)
	if !strings.HasPrefix(command, "./") {
		return nil
	}
	relative, e := pathcontract.ParseCommand(command)
	if e != nil {
		return e
	}
	observation := pathcontract.Resolve(root, relative)
	if observation.State != pathcontract.Resolved {
		return fmt.Errorf("stdio command escapes PLUGIN_ROOT or is unavailable: %w", observation.Err)
	}
	return nil
}

func observeWindsurfStdioCWD(root, dataPath, activePath string, server domain.MCPServer) error {
	rawCWD, _ := server.Decoded["cwd"].(string)
	cwd, e := pathcontract.ExpandCWD(rawCWD, activePath, dataPath)
	if e != nil {
		return e
	}
	anchor := root
	if cwd.Anchor == pathcontract.Data {
		anchor = dataPath
	}
	observation := pathcontract.Resolve(anchor, cwd.Relative)
	if observation.State != pathcontract.Resolved {
		return fmt.Errorf("stdio cwd unavailable: %w", observation.Err)
	}
	return nil
}

func BuildWindsurfNativeObjects(stagingRoot string, plan domain.DeliveryPlan) ([]domain.NativeObjectOwnership, error) {
	if strings.TrimSpace(plan.NativeRegistryRoot) == "" || !shared.HasSupportedMCP(plan.Components) {
		return nil, nil
	}
	configPath, err := windsurfConfigPath(plan.NativeRegistryRoot)
	if err != nil {
		return nil, err
	}
	servers, err := readProjectedWindsurfServers(stagingRoot)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	objects := make([]domain.NativeObjectOwnership, 0, len(names))
	for _, name := range names {
		receipt, err := desiredWindsurfReceipt(configPath, name, servers[name])
		if err != nil {
			return nil, fmt.Errorf("preview Windsurf MCP ownership %q: %w", name, err)
		}
		objects = append(objects, domain.NativeObjectOwnership{
			ObjectID:        "windsurf:mcp:" + name,
			Kind:            WindsurfMCPObjectKind,
			LogicalName:     name,
			Path:            configPath,
			SourceRelative:  "mcp.json",
			ManagedDigest:   receipt.Digest,
			ProtectionClass: "managed_entry",
		})
	}
	return objects, nil
}

func readProjectedWindsurfServers(root string) (map[string]nativeconfig.Server, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("prepared Windsurf package path is unavailable")
	}
	body, err := os.ReadFile(filepath.Join(root, "mcp.json"))
	if err != nil {
		return nil, fmt.Errorf("read prepared Windsurf MCP projection: %w", err)
	}
	var document struct {
		Servers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(body, &document); err != nil || document.Servers == nil {
		return nil, fmt.Errorf("decode prepared Windsurf MCP projection")
	}
	result := make(map[string]nativeconfig.Server, len(document.Servers))
	for name, raw := range document.Servers {
		server, err := decodeProjectedWindsurfServer(raw)
		if err != nil {
			return nil, fmt.Errorf("decode prepared Windsurf MCP server %q: %w", name, err)
		}
		result[name] = server
	}
	return result, nil
}

func commandForLauncher(decoded map[string]any) string {
	value, _ := decoded["command"].(string)
	return value
}

func resolveWindsurfPlaceholders(server nativeconfig.Server, packageRoot, dataRoot string) (nativeconfig.Server, error) {
	if server.Type != "stdio" {
		return server, nil
	}
	var err error
	for index := range server.Args {
		if server.Args[index], err = resolveWindsurfPlaceholder(server.Args[index], packageRoot, dataRoot); err != nil {
			return server, err
		}
	}
	for key, value := range server.Env {
		if key == "PLUGIN_ROOT" || key == "PLUGIN_DATA" {
			continue
		}
		if server.Env[key], err = resolveWindsurfPlaceholder(value, packageRoot, dataRoot); err != nil {
			return server, err
		}
	}
	return server, nil
}

func resolveWindsurfPlaceholder(value, packageRoot, dataRoot string) (string, error) {
	for _, replacement := range []struct{ token, value string }{{"${PLUGIN_ROOT}", packageRoot}, {"${PLUGIN_DATA}", dataRoot}} {
		if strings.Contains(value, replacement.token) {
			if strings.TrimSpace(replacement.value) == "" {
				return "", fmt.Errorf("explicit value for %s is required", replacement.token)
			}
		}
	}
	return strings.NewReplacer("${PLUGIN_ROOT}", packageRoot, "${PLUGIN_DATA}", dataRoot).Replace(value), nil
}

func StandardWindsurfServer(server nativeconfig.Server, originalType string) (map[string]any, error) {
	if server.Type == "stdio" {
		if strings.TrimSpace(server.CWD) != "" {
			return nil, fmt.Errorf("the Windsurf stdio MCP server does not support cwd")
		}
		result := map[string]any{"type": "stdio", "command": server.Command}
		if len(server.Args) > 0 {
			result["args"] = server.Args
		}
		if len(server.Env) > 0 {
			result["env"] = server.Env
		}
		return result, nil
	}
	result := map[string]any{"type": originalType, "url": server.URL}
	if len(server.Headers) > 0 {
		result["headers"] = server.Headers
	}
	return result, nil
}

func decodeProjectedWindsurfServer(raw json.RawMessage) (nativeconfig.Server, error) {
	var entry struct {
		Type    string            `json:"type"`
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
	}
	if err := json.Unmarshal(raw, &entry); err != nil {
		return nativeconfig.Server{}, err
	}
	if entry.Type == "stdio" {
		return nativeconfig.Server{Type: "stdio", Command: entry.Command, Args: entry.Args, Env: entry.Env, StdioValuesResolved: true}, nil
	}
	if entry.Type == "streamable-http" || entry.Type == "sse" {
		return nativeconfig.Server{Type: "remote", URL: entry.URL, Headers: entry.Headers, RemoteTransport: entry.Type}, nil
	}
	return nativeconfig.Server{}, fmt.Errorf("unsupported projected transport %q", entry.Type)
}
