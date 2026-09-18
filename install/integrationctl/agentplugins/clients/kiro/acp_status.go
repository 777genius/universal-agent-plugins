package kiro

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

func consumeKiroMCPStatus(document map[string]any, expected map[string]*kiroACPServerState) error {
	params, ok := document["params"].(map[string]any)
	if !ok {
		return fmt.Errorf("Kiro MCP status params are malformed")
	}
	rawServers, hasServers := params["servers"]
	_, hasServerName := params["serverName"]
	if hasServers && hasServerName {
		return fmt.Errorf("Kiro MCP status mixes array and legacy server shapes")
	}
	if _, hasAlias := params["name"]; hasAlias {
		return fmt.Errorf("Kiro MCP status contains an unknown or conflicting server identity")
	}
	sessionID, ok := params["sessionId"].(string)
	if !ok || strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("Kiro MCP status is missing its session identity")
	}
	if hasServers {
		servers, ok := rawServers.([]any)
		if !ok || len(servers) == 0 {
			return fmt.Errorf("Kiro MCP status servers are malformed or empty")
		}
		seen := make(map[string]struct{}, len(servers))
		for _, rawServer := range servers {
			server, ok := rawServer.(map[string]any)
			if !ok {
				return fmt.Errorf("Kiro MCP status contains a malformed server record")
			}
			if _, conflicting := server["serverName"]; conflicting {
				return fmt.Errorf("Kiro MCP status array record contains a conflicting identity field")
			}
			name, ok := server["name"].(string)
			if !ok || strings.TrimSpace(name) == "" {
				return fmt.Errorf("Kiro MCP status server has no unambiguous identity")
			}
			if _, duplicate := seen[name]; duplicate {
				return fmt.Errorf("Kiro MCP status contains duplicate server identity %q", name)
			}
			seen[name] = struct{}{}
			if err := consumeKiroMCPServer(name, sessionID, server, expected); err != nil {
				return err
			}
		}
		// The array form is a complete native-registry snapshot, not a delta.
		// Once a planned server has appeared, its omission from a later full
		// snapshot revokes the sticky state accumulated from older snapshots.
		// Failing immediately also prevents a previously complete snapshot from
		// surviving the settlement window after Kiro removes a server.
		for name, state := range expected {
			if _, present := seen[name]; !present && (state.connecting || state.connected) {
				return fmt.Errorf("%w: full Kiro MCP status snapshot omitted planned server %s", shared.ErrRecognizedNegativeEvidence, name)
			}
		}
		return nil
	}
	if !hasServerName {
		return fmt.Errorf("Kiro MCP status has no recognized server shape")
	}
	name, ok := params["serverName"].(string)
	if !ok || strings.TrimSpace(name) == "" {
		return fmt.Errorf("Kiro MCP status has no unambiguous server identity")
	}
	return consumeKiroMCPServer(name, sessionID, params, expected)
}

func consumeKiroMCPServer(name, sessionID string, server map[string]any, expected map[string]*kiroACPServerState) error {
	if authType, present := server["authType"]; present {
		value, valid := authType.(string)
		if !valid || strings.TrimSpace(value) == "" {
			return fmt.Errorf("Kiro MCP server %s has malformed auth type", name)
		}
	}
	if disabled, present := server["disabled"]; present {
		if _, valid := disabled.(bool); !valid {
			return fmt.Errorf("Kiro MCP server %s has malformed disabled state", name)
		}
	}
	if status, valid := server["status"].(string); !valid || strings.TrimSpace(status) == "" {
		return fmt.Errorf("Kiro MCP server %s has malformed status", name)
	}
	if rawTools, present := server["tools"]; present && rawTools != nil {
		tools, valid := rawTools.([]any)
		if !valid {
			return fmt.Errorf("Kiro MCP server %s has malformed tools", name)
		}
		seenToolNames := make(map[string]struct{}, len(tools))
		for _, rawTool := range tools {
			tool, valid := rawTool.(map[string]any)
			if !valid {
				return fmt.Errorf("Kiro MCP server %s has a malformed tool record", name)
			}
			toolName, valid := tool["name"].(string)
			if !valid || strings.TrimSpace(toolName) == "" {
				return fmt.Errorf("Kiro MCP server %s has an unnamed tool", name)
			}
			if _, duplicate := seenToolNames[toolName]; duplicate {
				return fmt.Errorf("Kiro MCP server %s has duplicate tool identity %s", name, toolName)
			}
			seenToolNames[toolName] = struct{}{}
			if disabled, valid := tool["disabled"].(bool); !valid {
				return fmt.Errorf("Kiro MCP server %s has a tool with malformed disabled state", name)
			} else if disabled {
				// Disabled tools are authoritative only for planned servers. An
				// unrelated native-registry entry must not veto this package.
				if _, planned := expected[name]; planned {
					return fmt.Errorf("%w: Kiro MCP server %s has a disabled tool", shared.ErrRecognizedNegativeEvidence, name)
				}
			}
		}
	}
	state, planned := expected[name]
	if !planned {
		// Kiro loads the complete native registry. Other well-formed server
		// notifications are unrelated to the package being verified.
		return nil
	}
	if state.sessionID != "" && state.sessionID != sessionID {
		return fmt.Errorf("%w: Kiro MCP server %s has conflicting session identities", shared.ErrRecognizedNegativeEvidence, name)
	}
	state.sessionID = sessionID
	if disabled, present := server["disabled"]; present {
		value, valid := disabled.(bool)
		if !valid {
			return fmt.Errorf("Kiro MCP server %s has malformed disabled state", name)
		}
		if value {
			return fmt.Errorf("%w: Kiro MCP server %s is disabled", shared.ErrRecognizedNegativeEvidence, name)
		}
	}
	status, ok := server["status"].(string)
	if !ok {
		return fmt.Errorf("Kiro MCP server %s has malformed status", name)
	}
	switch status {
	case "connecting":
		fingerprint, err := json.Marshal(server)
		if err != nil {
			return fmt.Errorf("encode Kiro MCP connecting state: %w", err)
		}
		// Once connected has been observed, every connecting record is a
		// regression. In particular, do not accept an exact duplicate of the
		// earlier connecting record when it was queued before settlement.
		if state.connected {
			return fmt.Errorf("%w: Kiro MCP server %s has regressive connecting status after connected", shared.ErrRecognizedNegativeEvidence, name)
		}
		if state.connecting && state.connectingRecord == string(fingerprint) {
			return nil
		}
		if state.connecting {
			return fmt.Errorf("%w: Kiro MCP server %s has duplicate or regressive status", shared.ErrRecognizedNegativeEvidence, name)
		}
		state.connecting = true
		state.connectingRecord = string(fingerprint)
		return nil
	case "connected":
		fingerprint, err := json.Marshal(server)
		if err != nil {
			return fmt.Errorf("encode Kiro MCP connected state: %w", err)
		}
		if state.connected {
			// The servers-array notification is a full registry snapshot. While
			// another server advances, Kiro repeats already-connected entries in
			// later snapshots. An identical repeat is idempotent evidence; a
			// changed connected record remains contradictory and fails closed.
			if state.connectedRecord == string(fingerprint) {
				return nil
			}
			return fmt.Errorf("%w: Kiro MCP server %s has duplicate connected identities", shared.ErrRecognizedNegativeEvidence, name)
		}
		tools, present := server["tools"].([]any)
		if !present || len(tools) == 0 {
			return fmt.Errorf("%w: Kiro MCP server %s has no usable tools", shared.ErrRecognizedNegativeEvidence, name)
		}
		seenTools := make(map[string]struct{}, len(tools))
		for _, rawTool := range tools {
			tool, valid := rawTool.(map[string]any)
			if !valid {
				return fmt.Errorf("Kiro MCP server %s has a malformed tool record", name)
			}
			toolName, valid := tool["name"].(string)
			if !valid || strings.TrimSpace(toolName) == "" {
				return fmt.Errorf("Kiro MCP server %s has an unnamed tool", name)
			}
			if _, duplicate := seenTools[toolName]; duplicate {
				return fmt.Errorf("Kiro MCP server %s has duplicate tool identity %s", name, toolName)
			}
			seenTools[toolName] = struct{}{}
			disabled, valid := tool["disabled"].(bool)
			if !valid {
				return fmt.Errorf("Kiro MCP server %s tool %s has malformed disabled state", name, toolName)
			}
			if disabled {
				return fmt.Errorf("%w: Kiro MCP server %s tool %s is disabled", shared.ErrRecognizedNegativeEvidence, name, toolName)
			}
		}
		state.connected = true
		state.connectedRecord = string(fingerprint)
		return nil
	case "pending", "disconnected", "disabled", "auth-required", "auth required", "authentication required", "error", "failed", "failure", "unhealthy":
		return fmt.Errorf("%w: Kiro MCP server %s reported %s", shared.ErrRecognizedNegativeEvidence, name, status)
	default:
		return fmt.Errorf("Kiro MCP server %s reported an unknown status %q", name, status)
	}
}

func allKiroServersConnected(expected map[string]*kiroACPServerState, sessionID string) bool {
	for _, state := range expected {
		if !state.connected || state.sessionID != sessionID {
			return false
		}
	}
	return true
}
