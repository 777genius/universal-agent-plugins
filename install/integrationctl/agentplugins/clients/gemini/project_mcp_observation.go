package gemini

import (
	"context"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tailscale/hujson"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func observeGeminiProjectMCP(ctx context.Context, env clients.Env, request domain.ActivationRequest) (bool, error) {
	expected := map[string]string{}
	for _, component := range request.Plan.Components {
		if component.Kind != domain.ComponentMCPServer {
			continue
		}
		command, args, ok := geminiOwnedMCPCommand(env, request, component.Name)
		if !ok {
			return false, nil
		}
		row := "✓ " + component.Name + ": " + command + " " + strings.Join(args, " ") + " (stdio) - Connected"
		if strings.ContainsAny(row, "\x00\r\n\x1b") {
			return false, nil
		}
		expected[component.Name] = row
	}
	if len(expected) == 0 {
		return true, nil
	}
	result, probeErr := runGeminiInventory(ctx, env, request, "mcp", "list")
	if probeErr != nil {
		return false, probeErr
	}
	// Gemini 0.65.0's debugLogger emits this inventory on stderr.
	return geminiMCPConnected(result.Stderr, expected) && len(result.Stdout) == 0, nil
}

func geminiMCPConnected(body []byte, expected map[string]string) bool {
	text := string(body)
	if !strings.HasPrefix(text, "Configured MCP servers:\n\n") || len(body) > 1<<20 {
		return false
	}
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for name, row := range expected {
		if strings.Count(text, " "+name+": ") != 1 {
			return false
		}
		found := false
		for _, line := range lines {
			found = found || line == row
		}
		if !found {
			return false
		}
	}
	return len(expected) > 0
}

func geminiOwnedMCPCommand(env clients.Env, request domain.ActivationRequest, name string) (string, []string, bool) {
	file, err := env.NativeConfig.ReadExactFile(filepath.Join(request.Plan.NativeRegistryRoot, "settings.json"))
	if err != nil || !file.Exists || len(file.Body) > 1<<20 {
		return "", nil, false
	}
	body, err := hujson.Standardize(file.Body)
	if err != nil {
		return "", nil, false
	}
	document, err := shared.DecodeStrictJSONObject(body)
	if err != nil {
		return "", nil, false
	}
	servers, ok := document["mcpServers"].(map[string]any)
	if !ok {
		return "", nil, false
	}
	entry, ok := servers[name].(map[string]any)
	if !ok {
		return "", nil, false
	}
	command, ok := entry["command"].(string)
	if !ok || command == "" || entry["url"] != nil || entry["httpUrl"] != nil {
		return "", nil, false
	}
	args := []string{}
	if raw, exists := entry["args"]; exists {
		values, ok := raw.([]any)
		if !ok {
			return "", nil, false
		}
		for _, value := range values {
			text, ok := value.(string)
			if !ok {
				return "", nil, false
			}
			args = append(args, text)
		}
	}
	return command, args, true
}

func geminiObservedLocalMCPAuthentication(plan domain.DeliveryPlan) bool {
	if plan.Authentication != domain.AuthenticationNotChecked || !slices.Contains(plan.Warnings, "authentication_not_catalog_verified") {
		return false
	}
	for _, component := range plan.Components {
		if component.Kind == domain.ComponentMCPServer {
			return true
		}
	}
	return false
}
