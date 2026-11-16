package codex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func verifyCodexNativeProjectConfig(reader *bufio.Scanner, writer *json.Encoder, project string, plan domain.DeliveryPlan) error {
	if err := writer.Encode(map[string]any{"id": 2, "method": "config/read", "params": map[string]any{"cwd": project, "includeLayers": false}}); err != nil {
		return err
	}
	result, err := codexDiscoveryResponse(reader, "2")
	if err != nil {
		return err
	}
	config, _ := result["config"].(map[string]any)
	projects, _ := config["projects"].(map[string]any)
	selected, _ := projects[project].(map[string]any)
	if selected["trust_level"] != "trusted" {
		return fmt.Errorf("codex does not report the exact selected project as trusted; native policy was preserved")
	}
	return verifyCodexEffectiveProjectMCP(config, plan)
}

// Older app-server versions ignore serverName and can initialize every enabled
// MCP server. Only the same process's qualified stable version may receive this
// selected-server request; a detected executable version is not that proof.
func codexNativeVersionSupportsSelectedMCP(agent string) bool {
	fields := strings.Fields(agent)
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "agentplugins/") {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(fields[0], "agentplugins/"), ".")
	if len(parts) != 3 {
		return false
	}
	var version [3]uint64
	for i, part := range parts {
		if part == "" || len(part) > 1 && part[0] == '0' {
			return false
		}
		for _, digit := range part {
			if digit < '0' || digit > '9' {
				return false
			}
		}
		number, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return false
		}
		version[i] = number
	}
	return version[0] > 0 || version[1] >= 162
}

func verifyCodexNativeMCP(reader *bufio.Scanner, writer *json.Encoder, plan domain.DeliveryPlan, pluginID, nativeAgent string) error {
	names := shared.SupportedMCPNames(plan)
	if len(names) > 0 && !codexNativeVersionSupportsSelectedMCP(nativeAgent) {
		return fmt.Errorf("automatic selected-server MCP discovery requires Codex 0.162.0 or newer; native server policy was preserved")
	}
	for index, name := range names {
		id := index + 4
		if err := writer.Encode(map[string]any{"id": id, "method": "mcpServerStatus/list", "params": map[string]any{"serverName": name, "detail": "toolsAndAuthOnly"}}); err != nil {
			return err
		}
		result, err := codexDiscoveryResponse(reader, fmt.Sprint(id))
		if err != nil {
			return err
		}
		if err := verifyCodexMCPStatus(result, name, pluginID); err != nil {
			return err
		}
	}
	return nil
}

func verifyCodexMCPStatus(result map[string]any, name, pluginID string) error {
	entries, ok := result["data"].([]any)
	if !ok || len(entries) != 1 || result["nextCursor"] != nil {
		return fmt.Errorf("codex did not return one exact MCP server")
	}
	entry, ok := entries[0].(map[string]any)
	if !ok || entry["name"] != name {
		return fmt.Errorf("codex returned another MCP server")
	}
	actualPlugin := entry["pluginId"]
	if !codexPluginMatches(actualPlugin, pluginID) {
		return fmt.Errorf("codex MCP server belongs to another plugin")
	}
	if entry["authStatus"] != "unsupported" || entry["toolsError"] != nil {
		return fmt.Errorf("codex MCP authentication or tool discovery is incomplete")
	}
	if status := entry["runtimeStatus"]; status != nil && status != "connected" {
		return fmt.Errorf("codex MCP runtime is not connected")
	}
	info, ok := entry["serverInfo"].(map[string]any)
	if !ok {
		return fmt.Errorf("codex MCP server did not initialize")
	}
	for _, key := range []string{"name", "version"} {
		if text, ok := info[key].(string); !ok || text == "" {
			return fmt.Errorf("codex MCP server identity is incomplete")
		}
	}
	if _, ok := entry["tools"].(map[string]any); !ok {
		return fmt.Errorf("codex MCP tool inventory is unavailable")
	}
	return nil
}

// A successful no-auth stdio handshake establishes installation authentication
// only. It says nothing about credentials later required by a tool or model.
func codexNativeAuthentication(plan domain.DeliveryPlan) domain.AuthenticationState {
	if (plan.Authentication == "" || plan.Authentication == domain.AuthenticationNotChecked) && len(shared.SupportedMCPNames(plan)) > 0 {
		return domain.AuthenticationNotRequired
	}
	return shared.ObservedSkillInstallationAuthentication(plan)
}

// Bind the native status to the selected project's effective command, not just
// a same-name server supplied by another native configuration layer.
func verifyCodexEffectiveProjectMCP(config map[string]any, plan domain.DeliveryPlan) error {
	servers, _ := config["mcp_servers"].(map[string]any)
	for _, name := range shared.SupportedMCPNames(plan) {
		expected, err := readCodexProjectServer(plan.ActivePath, name)
		if err != nil {
			return err
		}
		entry, ok := servers[name].(map[string]any)
		if !ok || entry["enabled"] == false {
			return fmt.Errorf("codex effective selected MCP server is absent or disabled")
		}
		body, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		var actual nativeconfig.Server
		if err := json.Unmarshal(body, &actual); err != nil {
			return err
		}
		if !codexDiscoveryPathMatches(actual.Command, expected.Command) || !slices.Equal(actual.Args, expected.Args) || !maps.Equal(actual.Env, expected.Env) || actual.CWD != expected.CWD && !codexDiscoveryPathMatches(actual.CWD, expected.CWD) {
			return fmt.Errorf("codex effective selected MCP command differs from the installed project component")
		}
	}
	return nil
}
