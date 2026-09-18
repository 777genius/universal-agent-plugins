package kiro

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

func consumeKiroMCPServer(name, sessionID string, server map[string]any, expected map[string]*kiroACPServerState) error {
	if err := validateKiroMCPServerShape(name, server, expected); err != nil {
		return err
	}
	state, planned := expected[name]
	if !planned {
		return nil
	}
	if state.sessionID != "" && state.sessionID != sessionID {
		return fmt.Errorf("%w: Kiro MCP server %s has conflicting session identities", shared.ErrRecognizedNegativeEvidence, name)
	}
	state.sessionID = sessionID
	if err := rejectDisabledKiroMCPServer(name, server); err != nil {
		return err
	}
	status, ok := server["status"].(string)
	if !ok {
		return fmt.Errorf("the Kiro MCP server %s has malformed status", name)
	}
	switch status {
	case "connecting":
		return consumeKiroMCPConnecting(name, server, state)
	case "connected":
		return consumeKiroMCPConnected(name, server, state)
	case "pending", "disconnected", "disabled", "auth-required", "auth required", "authentication required", "error", "failed", "failure", "unhealthy":
		return fmt.Errorf("%w: Kiro MCP server %s reported %s", shared.ErrRecognizedNegativeEvidence, name, status)
	default:
		return fmt.Errorf("the Kiro MCP server %s reported an unknown status %q", name, status)
	}
}

func validateKiroMCPServerShape(name string, server map[string]any, expected map[string]*kiroACPServerState) error {
	if authType, present := server["authType"]; present {
		value, valid := authType.(string)
		if !valid || strings.TrimSpace(value) == "" {
			return fmt.Errorf("the Kiro MCP server %s has malformed auth type", name)
		}
	}
	if disabled, present := server["disabled"]; present {
		if _, valid := disabled.(bool); !valid {
			return fmt.Errorf("the Kiro MCP server %s has malformed disabled state", name)
		}
	}
	if status, valid := server["status"].(string); !valid || strings.TrimSpace(status) == "" {
		return fmt.Errorf("the Kiro MCP server %s has malformed status", name)
	}
	return validateKiroMCPServerTools(name, server, expected)
}

func validateKiroMCPServerTools(name string, server map[string]any, expected map[string]*kiroACPServerState) error {
	rawTools, present := server["tools"]
	if !present || rawTools == nil {
		return nil
	}
	tools, valid := rawTools.([]any)
	if !valid {
		return fmt.Errorf("the Kiro MCP server %s has malformed tools", name)
	}
	seenToolNames := make(map[string]struct{}, len(tools))
	for _, rawTool := range tools {
		tool, valid := rawTool.(map[string]any)
		if !valid {
			return fmt.Errorf("the Kiro MCP server %s has a malformed tool record", name)
		}
		toolName, valid := tool["name"].(string)
		if !valid || strings.TrimSpace(toolName) == "" {
			return fmt.Errorf("the Kiro MCP server %s has an unnamed tool", name)
		}
		if _, duplicate := seenToolNames[toolName]; duplicate {
			return fmt.Errorf("the Kiro MCP server %s has duplicate tool identity %s", name, toolName)
		}
		seenToolNames[toolName] = struct{}{}
		if disabled, valid := tool["disabled"].(bool); !valid {
			return fmt.Errorf("the Kiro MCP server %s has a tool with malformed disabled state", name)
		} else if disabled {
			if _, planned := expected[name]; planned {
				return fmt.Errorf("%w: Kiro MCP server %s has a disabled tool", shared.ErrRecognizedNegativeEvidence, name)
			}
		}
	}
	return nil
}

func rejectDisabledKiroMCPServer(name string, server map[string]any) error {
	disabled, present := server["disabled"]
	if !present {
		return nil
	}
	value, valid := disabled.(bool)
	if !valid {
		return fmt.Errorf("the Kiro MCP server %s has malformed disabled state", name)
	}
	if value {
		return fmt.Errorf("%w: Kiro MCP server %s is disabled", shared.ErrRecognizedNegativeEvidence, name)
	}
	return nil
}

func consumeKiroMCPConnecting(name string, server map[string]any, state *kiroACPServerState) error {
	fingerprint, err := json.Marshal(server)
	if err != nil {
		return fmt.Errorf("encode Kiro MCP connecting state: %w", err)
	}
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
}

func consumeKiroMCPConnected(name string, server map[string]any, state *kiroACPServerState) error {
	fingerprint, err := json.Marshal(server)
	if err != nil {
		return fmt.Errorf("encode Kiro MCP connected state: %w", err)
	}
	if state.connected {
		if state.connectedRecord == string(fingerprint) {
			return nil
		}
		return fmt.Errorf("%w: Kiro MCP server %s has duplicate connected identities", shared.ErrRecognizedNegativeEvidence, name)
	}
	if err := requireUsableKiroMCPTools(name, server); err != nil {
		return err
	}
	state.connected = true
	state.connectedRecord = string(fingerprint)
	return nil
}

func requireUsableKiroMCPTools(name string, server map[string]any) error {
	tools, present := server["tools"].([]any)
	if !present || len(tools) == 0 {
		return fmt.Errorf("%w: Kiro MCP server %s has no usable tools", shared.ErrRecognizedNegativeEvidence, name)
	}
	seenTools := make(map[string]struct{}, len(tools))
	for _, rawTool := range tools {
		tool, valid := rawTool.(map[string]any)
		if !valid {
			return fmt.Errorf("the Kiro MCP server %s has a malformed tool record", name)
		}
		toolName, valid := tool["name"].(string)
		if !valid || strings.TrimSpace(toolName) == "" {
			return fmt.Errorf("the Kiro MCP server %s has an unnamed tool", name)
		}
		if _, duplicate := seenTools[toolName]; duplicate {
			return fmt.Errorf("the Kiro MCP server %s has duplicate tool identity %s", name, toolName)
		}
		seenTools[toolName] = struct{}{}
		disabled, valid := tool["disabled"].(bool)
		if !valid {
			return fmt.Errorf("the Kiro MCP server %s tool %s has malformed disabled state", name, toolName)
		}
		if disabled {
			return fmt.Errorf("%w: Kiro MCP server %s tool %s is disabled", shared.ErrRecognizedNegativeEvidence, name, toolName)
		}
	}
	return nil
}
