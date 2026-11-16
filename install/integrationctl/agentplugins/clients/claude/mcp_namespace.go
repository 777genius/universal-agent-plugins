package claude

import (
	"context"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// MCP selectors omit the marketplace and scope. The exact package observer
// proves ownership; this additional observation excludes a co-enabled plugin
// that could otherwise supply a same-name server to `mcp get`.
func claudeMCPNamespaceUnique(body []byte, name string) (bool, error) {
	entries, ok := decodeProjectList(body)
	if !ok || len(body) > 1<<20 {
		return false, fmt.Errorf("claude plugin namespace listing is not recognized")
	}
	enabled := 0
	for _, value := range entries {
		entry, ok := value.(map[string]any)
		if !ok {
			return false, fmt.Errorf("claude plugin namespace row is invalid")
		}
		id, validID := entry["id"].(string)
		active, validActive := entry["enabled"].(bool)
		if !validID || id == "" || !validActive {
			return false, fmt.Errorf("claude plugin namespace identity is invalid")
		}
		declared, _, qualified := strings.Cut(id, "@")
		if !qualified {
			return false, fmt.Errorf("claude plugin namespace is unqualified")
		}
		if declared == name && active {
			enabled++
		}
	}
	return enabled == 1, nil
}

func inspectClaudeMCPNamespace(ctx context.Context, env clients.Env, request domain.ActivationRequest) (bool, error) {
	probe, err := PrepareClaudeActivationProbe(request)
	if err != nil {
		return false, err
	}
	listed, err := runPreparedClaudeList(ctx, env, probe.Command)
	if err != nil {
		return false, err
	}
	return claudeMCPNamespaceUnique(listed.Stdout, request.DeclaredName)
}
