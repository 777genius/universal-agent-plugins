package vscode

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
)

func readWorkspaceMCP(root string) (map[string]any, map[string]any, error) {
	path := filepath.Join(root, "mcp.json")
	if err := pathpolicy.RequireContainedChild(root, path); err != nil {
		return nil, nil, err
	}
	file, err := nativeconfig.New().ReadExactFile(path)
	if err != nil {
		return nil, nil, err
	}
	if !file.Exists || len(file.Body) > 1<<20 {
		return nil, nil, fmt.Errorf("VS Code workspace package MCP document is missing or exceeds inspection limit")
	}
	document, err := shared.DecodeStrictJSONObject(file.Body)
	if err != nil {
		return nil, nil, err
	}
	servers, ok := document["mcpServers"].(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("VS Code workspace package MCP collection is missing")
	}
	return document, servers, nil
}

func readWorkspaceServer(root, name string) (nativeconfig.Server, error) {
	_, servers, err := readWorkspaceMCP(root)
	if err != nil {
		return nativeconfig.Server{}, err
	}
	entry, ok := servers[name].(map[string]any)
	if !ok {
		return nativeconfig.Server{}, fmt.Errorf("VS Code workspace selected MCP server is missing")
	}
	return decodeWorkspaceServer(entry)
}

func decodeWorkspaceServer(entry map[string]any) (nativeconfig.Server, error) {
	for key := range entry {
		switch key {
		case "type", "command", "args", "env", "cwd", "url", "headers":
		default:
			return nativeconfig.Server{}, fmt.Errorf("unsupported VS Code projected MCP field %q", key)
		}
	}
	body, err := json.Marshal(entry)
	if err != nil {
		return nativeconfig.Server{}, err
	}
	var server nativeconfig.Server
	if err := json.Unmarshal(body, &server); err != nil {
		return server, err
	}
	switch server.Type {
	case "", "stdio":
		server.Type = "stdio"
	case "http":
		server.Type, server.RemoteTransport = "remote", "streamable-http"
	case "sse":
		server.Type, server.RemoteTransport = "remote", "sse"
	default:
		return server, fmt.Errorf("VS Code workspace projected MCP transport is unsupported")
	}
	server.StdioValuesResolved, server.CWDResolved = true, true
	return server, nil
}

func projectWorkspaceStdio(in clients.ProjectionInput) error {
	if err := shared.DeliverManagedStdio(in.DeliverLauncher(), in.StagingPath, in.Envelope, in.Plan); err != nil {
		return err
	}
	if len(shared.SupportedMCPNames(in.Plan)) == 0 {
		return nil
	}
	document, entries, err := readWorkspaceMCP(in.StagingPath)
	if err != nil {
		return err
	}
	for _, name := range shared.SupportedMCPNames(in.Plan) {
		source := in.Envelope.MCP.Servers[name]
		if source.Type != "stdio" {
			continue
		}
		server, err := readWorkspaceServer(in.StagingPath, name)
		if err != nil {
			return err
		}
		server, err = bindWorkspaceStdio(server, source, in.Plan.ActivePath, in.PluginDataPath)
		if err != nil {
			return err
		}
		body, err := json.Marshal(server)
		if err != nil {
			return err
		}
		var native map[string]any
		if err := json.Unmarshal(body, &native); err != nil {
			return err
		}
		entries[name] = native
	}
	return shared.WriteJSON(filepath.Join(in.StagingPath, "mcp.json"), document)
}

func bindWorkspaceStdio(server nativeconfig.Server, source domain.MCPServer, activePath, dataPath string) (nativeconfig.Server, error) {
	rawCWD, _ := source.Decoded["cwd"].(string)
	cwd, err := pathcontract.ExpandCWD(rawCWD, activePath, dataPath)
	if err != nil {
		return server, err
	}
	command, _ := source.Decoded["command"].(string)
	if strings.HasPrefix(command, "${package.root}/") {
		command = "./" + strings.TrimPrefix(command, "${package.root}/")
	}
	server.Args = managedstdio.Arguments(activePath, dataPath, server.CWD, cwd.Anchor, command, server.Args)
	server.Command = filepath.Join(activePath, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)
	server.CWD = ""
	return server, nil
}
