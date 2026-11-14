package codex

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Read the existing OpenAI projection so managed stdio paths/data and accepted
// compatibility auth references are materialized once by the shared projector.
func readCodexProjectServer(root, name string) (nativeconfig.Server, error) {
	var result nativeconfig.Server
	path := filepath.Join(root, ".mcp.json")
	if err := pathpolicy.RequireContainedChild(root, path); err != nil {
		return result, err
	}
	file, err := nativeconfig.New().ReadExactFile(path)
	if err != nil {
		return result, err
	}
	if !file.Exists || len(file.Body) > 1<<20 {
		return result, fmt.Errorf("codex projected MCP document is absent or exceeds inspection limit")
	}
	body := file.Body
	doc, err := shared.DecodeStrictJSONObject(body)
	if err != nil {
		return result, err
	}
	servers, ok := doc["mcpServers"].(map[string]any)
	if !ok {
		return result, fmt.Errorf("codex projection has no MCP server collection")
	}
	value, ok := servers[name].(map[string]any)
	if !ok {
		return result, fmt.Errorf("codex projected MCP server is missing")
	}
	body, err = json.Marshal(value)
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return result, fmt.Errorf("codex projected MCP server has invalid fields")
	}
	switch result.Type {
	case "", "stdio":
		result.Type, result.StdioValuesResolved, result.CWDResolved = "stdio", true, true
	case "http":
		result.Type, result.RemoteTransport = "remote", "streamable-http"
	default:
		return result, fmt.Errorf("codex project MCP requires stdio or streamable-http")
	}
	var auth nativeconfig.CodexAuthOptions
	if err := json.Unmarshal(body, &auth); err != nil {
		return result, fmt.Errorf("codex projected authentication references are invalid")
	}
	if auth != (nativeconfig.CodexAuthOptions{}) {
		result.CodexAuth = &auth
	}
	return result, nil
}

func validateCodexProjectStdio(root string, plan domain.DeliveryPlan, dataPath string) error {
	for _, name := range shared.SupportedMCPNames(plan) {
		server, err := readCodexProjectServer(root, name)
		if err != nil {
			return err
		}
		if server.Type == "stdio" && (server.Env["PLUGIN_ROOT"] != plan.ActivePath || dataPath != "" && server.Env["PLUGIN_DATA"] != dataPath) {
			return fmt.Errorf("codex projected stdio package/data environment differs from bound locators")
		}
	}
	return nil
}
