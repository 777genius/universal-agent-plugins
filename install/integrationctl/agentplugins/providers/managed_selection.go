package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// ManagedMCPNames reads delivery selection only from a digest-verified artifact.
// A second verification detects artifact drift during the read. It does not
// promise protection from same-principal ABA races.
func (stager Stager) ManagedMCPNames(ctx context.Context, client domain.ClientID, root, digest string) ([]string, error) {
	if err := stager.Verify(ctx, root, digest); err != nil {
		return nil, err
	}
	filename := "mcp.json"
	if client == domain.ClientClaude || client == domain.ClientCodex || client == domain.ClientChatGPT {
		filename = ".mcp.json"
	}
	candidate := filepath.Join(root, filename)
	info, statErr := os.Lstat(candidate)
	if statErr != nil && !os.IsNotExist(statErr) {
		return nil, statErr
	}
	if statErr == nil && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("managed MCP selection is not a regular file")
	}
	body, err := os.ReadFile(candidate)
	names := []string{}
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err == nil {
		var document map[string]json.RawMessage
		if err := json.Unmarshal(body, &document); err != nil || document == nil {
			return nil, fmt.Errorf("managed MCP selection is invalid: %v", err)
		}
		servers := document
		if client != domain.ClientClaude {
			raw, ok := document["mcpServers"]
			if !ok {
				return nil, fmt.Errorf("managed MCP selection lacks mcpServers")
			}
			servers = nil
			if err := json.Unmarshal(raw, &servers); err != nil || servers == nil {
				return nil, fmt.Errorf("managed MCP server selection is invalid: %v", err)
			}
		}
		for name := range servers {
			names = append(names, name)
		}
	}
	if err := stager.Verify(ctx, root, digest); err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}
