package kiro

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
)

// The workspace JSON does not need an undocumented cwd field: the trusted
// launcher establishes the selected package/data cwd before starting stdio.
func projectKiroStdio(in clients.ProjectionInput) error {
	if err := shared.DeliverManagedStdio(in.DeliverLauncher(), in.StagingPath, in.Envelope, in.Plan); err != nil {
		return err
	}
	servers := map[string]any{}
	for _, name := range shared.SupportedMCPNames(in.Plan) {
		entry, err := projectedKiroMCPServer(in.StagingPath, name)
		if err != nil {
			return err
		}
		delete(entry, "type") // Kiro selects local/remote from command/url, not portable transport tags.
		source := in.Envelope.MCP.Servers[name]
		if source.Type == "stdio" {
			if err := bindKiroProjectStdio(entry, source, in.Plan.ActivePath, in.PluginDataPath); err != nil {
				return err
			}
		}
		servers[name] = entry
	}
	if len(servers) == 0 {
		return nil
	}
	return shared.WriteJSON(filepath.Join(in.StagingPath, "mcp.json"), map[string]any{"mcpServers": servers})
}

func bindKiroProjectStdio(entry map[string]any, source domain.MCPServer, packageRoot, dataRoot string) error {
	if !filepath.IsAbs(packageRoot) || !filepath.IsAbs(dataRoot) {
		return fmt.Errorf("kiro stdio requires absolute package/data roots")
	}
	rawCWD, _ := source.Decoded["cwd"].(string)
	cwd, err := pathcontract.ExpandCWD(rawCWD, packageRoot, dataRoot)
	if err != nil {
		return err
	}
	absoluteCWD, ok := entry["cwd"].(string)
	if !ok {
		return fmt.Errorf("kiro projected stdio cwd is missing")
	}
	args, err := kiroProjectArguments(entry)
	if err != nil {
		return err
	}
	command, _ := source.Decoded["command"].(string)
	if strings.HasPrefix(command, "${package.root}/") {
		command = "./" + strings.TrimPrefix(command, "${package.root}/")
	}
	entry["command"] = filepath.Join(packageRoot, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)
	entry["args"] = managedstdio.Arguments(packageRoot, dataRoot, absoluteCWD, cwd.Anchor, command, args)
	delete(entry, "cwd")
	return nil
}

func kiroProjectArguments(entry map[string]any) ([]string, error) {
	raw, present := entry["args"]
	if !present {
		return nil, nil
	}
	array, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("kiro projected stdio args must be an array")
	}
	result := make([]string, len(array))
	for i, value := range array {
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("kiro stdio args must contain strings")
		}
		result[i] = text
	}
	return result, nil
}
