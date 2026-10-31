package kimi

import (
	"context"
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

func projectKimiComponents(ctx context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := kimiPlanRoot(domain.DetectedClient{ConfigRoot: in.Plan.TargetContext.UserConfigRoot}, in.Plan); err != nil {
		return nil, err
	}
	if err := pathpolicy.RequireContainedChild(in.Plan.TargetContext.ProjectRoot, in.StagingPath); err != nil {
		return nil, err
	}
	if err := shared.DeliverManagedStdio(in.DeliverLauncher(), in.StagingPath, in.Envelope, in.Plan); err != nil {
		return nil, err
	}
	entries := map[string]any{}
	for _, name := range shared.SupportedMCPNames(in.Plan) {
		entry, err := renderKimiProjectServer(in.Envelope.MCP.Servers[name], in.Plan.ActivePath, in.PluginDataPath, in.StagingPath)
		if err != nil {
			return nil, err
		}
		entries[name] = entry
	}
	if len(entries) > 0 {
		if err := shared.WriteJSON(filepath.Join(in.StagingPath, "mcp.json"), map[string]any{"mcpServers": entries}); err != nil {
			return nil, err
		}
	}
	return kimiProjectObjects(in.StagingPath, in.Envelope, in.Plan)
}

func renderKimiProjectServer(source domain.MCPServer, active, data, staging string) (map[string]any, error) {
	entry := shared.CloneObject(source.Decoded)
	delete(entry, "type")
	switch source.Type {
	case "stdio":
		if !filepath.IsAbs(data) || filepath.Clean(data) != data {
			return nil, fmt.Errorf("kimi selected stdio data path must be absolute and clean")
		}
		if err := shared.ApplyStdioDataContract(entry, active, data, staging); err != nil {
			return nil, err
		}
		if err := bindKimiStdio(entry, source, active, data); err != nil {
			return nil, err
		}
		entry["transport"] = "stdio"
	case "streamable-http":
		entry["transport"] = "http"
	case "sse":
		entry["transport"] = "sse"
	default:
		return nil, fmt.Errorf("unsupported Kimi project MCP transport %q", source.Type)
	}
	return entry, nil
}

func bindKimiStdio(entry map[string]any, source domain.MCPServer, active, data string) error {
	rawCWD, _ := source.Decoded["cwd"].(string)
	cwd, err := pathcontract.ExpandCWD(rawCWD, active, data)
	if err != nil {
		return err
	}
	absolute, ok := entry["cwd"].(string)
	if !ok {
		return fmt.Errorf("kimi projected stdio cwd is missing")
	}
	args := []string{}
	if raw, present := entry["args"]; present {
		values, ok := raw.([]any)
		if !ok {
			return fmt.Errorf("kimi stdio arguments must be strings")
		}
		for _, value := range values {
			text, ok := value.(string)
			if !ok {
				return fmt.Errorf("kimi stdio argument must be string")
			}
			args = append(args, text)
		}
	}
	command, _ := source.Decoded["command"].(string)
	if strings.HasPrefix(command, "${package.root}/") {
		command = "./" + strings.TrimPrefix(command, "${package.root}/")
	}
	entry["command"] = filepath.Join(active, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)
	entry["args"] = managedstdio.Arguments(active, data, absolute, cwd.Anchor, command, args)
	delete(entry, "cwd")
	return nil
}

func readKimiFile(path string) ([]byte, error) {
	file, err := nativeconfig.New().ReadExactFile(path)
	if err != nil {
		return nil, err
	}
	if len(file.Body) > 1<<20 {
		return nil, fmt.Errorf("kimi MCP file exceeds inspection limit")
	}
	return file.Body, nil
}

func readKimiProjectEntry(root, name string) (map[string]any, error) {
	path := filepath.Join(root, "mcp.json")
	if err := pathpolicy.RequireContainedChild(root, path); err != nil {
		return nil, err
	}
	body, err := readKimiFile(path)
	if err != nil {
		return nil, err
	}
	document, err := shared.DecodeStrictJSONObject(body)
	if err != nil {
		return nil, err
	}
	entries, ok := document["mcpServers"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("kimi projected MCP collection is missing")
	}
	entry, ok := entries[name].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("kimi selected MCP entry is missing")
	}
	return entry, nil
}

func kimiProjectServer(root, name string) (nativeconfig.Server, error) {
	entry, err := readKimiProjectEntry(root, name)
	if err != nil {
		return nativeconfig.Server{}, err
	}
	body, err := json.Marshal(entry)
	if err != nil {
		return nativeconfig.Server{}, err
	}
	var server nativeconfig.Server
	if err := json.Unmarshal(body, &server); err != nil {
		return server, err
	}
	switch entry["transport"] {
	case "stdio":
		server.Type = "stdio"
	case "http":
		server.Type, server.RemoteTransport = "remote", "streamable-http"
	case "sse":
		server.Type, server.RemoteTransport = "remote", "sse"
	default:
		return server, fmt.Errorf("kimi native transport is unsupported")
	}
	server.StdioValuesResolved, server.CWDResolved = true, true
	return server, nil
}
