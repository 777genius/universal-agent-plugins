package usecase

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// This inert health answer is admitted only for one metadata-bound plugin and
// server in this fixture's selected profile. No native server is executed.
func (r *fakeClaudeLifecycleRunner) selectedMCPHealth(command legacyports.Command) (legacyports.CommandResult, error) {
	profile := ""
	for _, value := range command.Env {
		if strings.HasPrefix(value, "CLAUDE_CONFIG_DIR=") {
			profile = strings.TrimPrefix(value, "CLAUDE_CONFIG_DIR=")
		}
	}
	if profile != r.configRoot {
		return legacyports.CommandResult{}, fmt.Errorf("fixture MCP health lost selected profile")
	}
	selector := command.Argv[3]
	parts := strings.Split(selector, ":")
	if len(parts) != 3 || parts[0] != "plugin" || parts[1] == "" || parts[2] == "" {
		return legacyports.CommandResult{}, fmt.Errorf("unknown fixture MCP selector")
	}
	entries, err := os.ReadDir(filepath.Join(profile, "skills"))
	if err != nil {
		return legacyports.CommandResult{}, err
	}
	matches := 0
	var selected map[string]any
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		root := filepath.Join(profile, "skills", entry.Name())
		body, err := os.ReadFile(filepath.Join(root, ".claude-plugin", "plugin.json"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return legacyports.CommandResult{}, err
		}
		var manifest struct{ Name string }
		if err := json.Unmarshal(body, &manifest); err != nil {
			return legacyports.CommandResult{}, err
		}
		if manifest.Name != parts[1] {
			continue
		}
		body, err = os.ReadFile(filepath.Join(root, ".mcp.json"))
		if err != nil {
			return legacyports.CommandResult{}, err
		}
		var servers map[string]map[string]any
		if err := json.Unmarshal(body, &servers); err != nil {
			return legacyports.CommandResult{}, err
		}
		server, exists := servers[parts[2]]
		if !exists {
			return legacyports.CommandResult{}, fmt.Errorf("fixture MCP server is not selected")
		}
		selected = server
		matches++
	}
	if matches != 1 {
		return legacyports.CommandResult{}, fmt.Errorf("fixture MCP namespace is not unique")
	}
	kind, _ := selected["type"].(string)
	if kind == "" {
		if command, _ := selected["command"].(string); command != "" {
			kind = "stdio"
		}
	}
	if kind != "stdio" && kind != "http" && kind != "sse" {
		return legacyports.CommandResult{}, fmt.Errorf("fixture MCP transport is not recognized")
	}
	return legacyports.CommandResult{Stdout: []byte(fmt.Sprintf("%s:\n  Scope: Dynamic config (from command line)\n  Status: ✔ Connected\n  Type: %s\n", selector, kind))}, nil
}
