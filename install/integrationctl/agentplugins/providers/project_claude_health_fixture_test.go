package providers_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (r *claudeProjectRunner) selectedMCPHealth(selector string) (legacyports.CommandResult, error) {
	prefix := "plugin:" + r.fixture.request.DeclaredName + ":"
	if !strings.HasPrefix(selector, prefix) {
		return legacyports.CommandResult{}, fmt.Errorf("foreign fixture MCP selector %q", selector)
	}
	name := strings.TrimPrefix(selector, prefix)
	body, err := os.ReadFile(filepath.Join(r.fixture.request.Plan.ActivePath, ".mcp.json"))
	if err != nil {
		return legacyports.CommandResult{}, err
	}
	var servers map[string]map[string]any
	if err := json.Unmarshal(body, &servers); err != nil {
		return legacyports.CommandResult{}, err
	}
	server, ok := servers[name]
	if !ok {
		return legacyports.CommandResult{}, fmt.Errorf("unselected fixture MCP server %q", name)
	}
	kind, _ := server["type"].(string)
	if kind == "" {
		if command, _ := server["command"].(string); command != "" {
			kind = "stdio"
		}
	}
	if kind != "stdio" && kind != "http" && kind != "sse" {
		return legacyports.CommandResult{}, fmt.Errorf("unknown fixture MCP transport %q", kind)
	}
	return legacyports.CommandResult{Stdout: []byte(fmt.Sprintf("%s:\n  Scope: Dynamic config (from command line)\n  Status: ✔ Connected\n  Type: %s\n", selector, kind))}, nil
}
