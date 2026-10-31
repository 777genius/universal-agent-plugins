package cursor

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
)

// The project lane uses the existing trusted launcher to establish cwd rather
// than assuming Cursor interprets the portable cwd field or plugin placeholders.
func projectCursorStdio(in clients.ProjectionInput) error {
	if err := shared.DeliverManagedStdio(in.DeliverLauncher(), in.StagingPath, in.Envelope, in.Plan); err != nil {
		return err
	}
	if len(shared.SupportedMCPNames(in.Plan)) == 0 {
		return nil
	}
	file, err := nativeconfig.New().ReadExactFile(filepath.Join(in.StagingPath, "mcp.json"))
	if err != nil {
		return err
	}
	document, err := shared.DecodeStrictJSONObject(file.Body)
	if err != nil {
		return err
	}
	servers, ok := document["mcpServers"].(map[string]any)
	if !ok {
		return fmt.Errorf("cursor package has no MCP collection")
	}
	for _, name := range shared.SupportedMCPNames(in.Plan) {
		server := in.Envelope.MCP.Servers[name]
		if server.Type != "stdio" {
			continue
		}
		entry, ok := servers[name].(map[string]any)
		if !ok {
			return fmt.Errorf("cursor projected stdio entry is missing")
		}
		if err := bindCursorStdio(entry, server, in.Plan.ActivePath, in.PluginDataPath); err != nil {
			return err
		}
	}
	return shared.WriteJSON(filepath.Join(in.StagingPath, "mcp.json"), document)
}

func bindCursorStdio(entry map[string]any, server domain.MCPServer, activePath, dataPath string) error {
	if !filepath.IsAbs(activePath) || !filepath.IsAbs(dataPath) {
		return fmt.Errorf("cursor managed stdio roots must be absolute")
	}
	rawCWD, _ := server.Decoded["cwd"].(string)
	cwd, err := pathcontract.ExpandCWD(rawCWD, activePath, dataPath)
	if err != nil {
		return err
	}
	absoluteCWD, ok := entry["cwd"].(string)
	if !ok {
		return fmt.Errorf("cursor projected stdio cwd is missing")
	}
	rawArgs, _ := entry["args"].([]any)
	args := make([]string, 0, len(rawArgs))
	for _, value := range rawArgs {
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("cursor stdio arguments must be strings")
		}
		args = append(args, text)
	}
	command, _ := server.Decoded["command"].(string)
	if strings.HasPrefix(command, "${package.root}/") {
		command = "./" + strings.TrimPrefix(command, "${package.root}/")
	}
	entry["command"] = filepath.Join(activePath, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)
	entry["args"] = managedstdio.EncodedArguments(activePath, dataPath, absoluteCWD, cwd.Anchor, command, args)
	delete(entry, "cwd")
	return nil
}
