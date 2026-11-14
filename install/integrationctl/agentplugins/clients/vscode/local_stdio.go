package vscode

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
)

// Use the existing positional helper protocol and portable path/environment
// contract. The primary native hook executable remains the host's fixed runtime;
// only optional package MCP processes use the supplied managed stdio helper.
func projectLocalMCP(in clients.ProjectionInput, names []string) error {
	if len(names) == 0 {
		return nil
	}
	servers := make(map[string]map[string]any, len(names))
	for _, name := range names {
		decoded := shared.CloneObject(in.Envelope.MCP.Servers[name].Decoded)
		command, _ := decoded["command"].(string)
		authoredCWD, _ := decoded["cwd"].(string)
		cwd, err := pathcontract.ParseCWD(authoredCWD)
		if err != nil {
			return err
		}
		if err := shared.ApplyStdioDataContract(decoded, in.Plan.ActivePath, in.PluginDataPath, in.StagingPath); err != nil {
			return err
		}
		absoluteCWD, _ := decoded["cwd"].(string)
		args, err := localStdioArgs(decoded["args"])
		if err != nil {
			return err
		}
		decoded["command"] = filepath.Join(in.Plan.ActivePath, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)
		decoded["args"] = managedstdio.Arguments(in.Plan.ActivePath, in.PluginDataPath, absoluteCWD, cwd.Anchor, command, args)
		servers[name] = decoded
	}
	return shared.WriteJSON(filepath.Join(in.StagingPath, "mcp.json"), map[string]any{"mcpServers": servers})
}

func localStdioArgs(raw any) ([]string, error) {
	switch args := raw.(type) {
	case nil:
		return nil, nil
	case []string:
		return append([]string(nil), args...), nil
	case []any:
		out := make([]string, len(args))
		for i, arg := range args {
			text, ok := arg.(string)
			if !ok {
				return nil, fmt.Errorf("local stdio arguments must be fixed strings")
			}
			out[i] = text
		}
		return out, nil
	default:
		return nil, fmt.Errorf("local stdio arguments must be an array")
	}
}
