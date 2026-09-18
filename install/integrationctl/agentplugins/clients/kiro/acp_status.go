package kiro

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

func consumeKiroMCPStatus(document map[string]any, expected map[string]*kiroACPServerState) error {
	params, ok := document["params"].(map[string]any)
	if !ok {
		return fmt.Errorf("the Kiro MCP status params are malformed")
	}
	rawServers, hasServers := params["servers"]
	_, hasServerName := params["serverName"]
	if hasServers && hasServerName {
		return fmt.Errorf("the Kiro MCP status mixes array and legacy server shapes")
	}
	if _, hasAlias := params["name"]; hasAlias {
		return fmt.Errorf("the Kiro MCP status contains an unknown or conflicting server identity")
	}
	sessionID, ok := params["sessionId"].(string)
	if !ok || strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("the Kiro MCP status is missing its session identity")
	}
	if hasServers {
		return consumeKiroMCPStatusArray(rawServers, sessionID, expected)
	}
	if !hasServerName {
		return fmt.Errorf("the Kiro MCP status has no recognized server shape")
	}
	name, ok := params["serverName"].(string)
	if !ok || strings.TrimSpace(name) == "" {
		return fmt.Errorf("the Kiro MCP status has no unambiguous server identity")
	}
	return consumeKiroMCPServer(name, sessionID, params, expected)
}

func consumeKiroMCPStatusArray(rawServers any, sessionID string, expected map[string]*kiroACPServerState) error {
	servers, ok := rawServers.([]any)
	if !ok || len(servers) == 0 {
		return fmt.Errorf("the Kiro MCP status servers are malformed or empty")
	}
	seen := make(map[string]struct{}, len(servers))
	for _, rawServer := range servers {
		server, ok := rawServer.(map[string]any)
		if !ok {
			return fmt.Errorf("the Kiro MCP status contains a malformed server record")
		}
		if _, conflicting := server["serverName"]; conflicting {
			return fmt.Errorf("the Kiro MCP status array record contains a conflicting identity field")
		}
		name, ok := server["name"].(string)
		if !ok || strings.TrimSpace(name) == "" {
			return fmt.Errorf("the Kiro MCP status server has no unambiguous identity")
		}
		if _, duplicate := seen[name]; duplicate {
			return fmt.Errorf("the Kiro MCP status contains duplicate server identity %q", name)
		}
		seen[name] = struct{}{}
		if err := consumeKiroMCPServer(name, sessionID, server, expected); err != nil {
			return err
		}
	}
	for name, state := range expected {
		if _, present := seen[name]; !present && (state.connecting || state.connected) {
			return fmt.Errorf("%w: full Kiro MCP status snapshot omitted planned server %s", shared.ErrRecognizedNegativeEvidence, name)
		}
	}
	return nil
}

func allKiroServersConnected(expected map[string]*kiroACPServerState, sessionID string) bool {
	for _, state := range expected {
		if !state.connected || state.sessionID != sessionID {
			return false
		}
	}
	return true
}
