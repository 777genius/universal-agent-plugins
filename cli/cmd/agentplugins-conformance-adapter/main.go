// Command agentplugins-conformance-adapter exposes the standard loader through
// the process protocol used by the Agent Plugins conformance corpus. It only
// parses package files; it never installs a package or starts an MCP server.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type namedEntry struct {
	What   string `json:"what,omitempty"`
	Field  string `json:"field,omitempty"`
	RuleID string `json:"ruleId,omitempty"`
}

type loadReport struct {
	Rejected *string `json:"rejected"`
	Loaded   struct {
		Skills     []string `json:"skills"`
		MCPServers []string `json:"mcpServers"`
	} `json:"loaded"`
	Skipped  []namedEntry `json:"skipped"`
	Reported []namedEntry `json:"reported"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: agentplugins-conformance-adapter PLUGIN_DIR")
		return 2
	}
	report, err := inspect(args[0])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err := json.NewEncoder(stdout).Encode(report); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}

func inspect(root string) (loadReport, error) {
	result := emptyReport()
	registry, err := specregistry.New()
	if err != nil {
		return result, fmt.Errorf("create schema registry: %w", err)
	}
	envelope, loadErr := (loader.Loader{Registry: registry}).Load(context.Background(), domain.LoadInput{SnapshotRoot: root})
	if loadErr != nil {
		reason := loadErr.Error()
		result.Rejected = &reason
		return result, nil
	}
	for name := range envelope.Skills {
		result.Loaded.Skills = append(result.Loaded.Skills, name)
	}
	for name := range envelope.MCP.Servers {
		result.Loaded.MCPServers = append(result.Loaded.MCPServers, name)
	}
	sort.Strings(result.Loaded.Skills)
	sort.Strings(result.Loaded.MCPServers)
	if envelope.Inventory.InvalidSkillsRoot {
		result.Skipped = append(result.Skipped, namedEntry{What: "skills"})
	}
	for _, name := range envelope.Inventory.InvalidSkills {
		result.Skipped = append(result.Skipped, namedEntry{What: "skills/" + name})
	}
	if envelope.MCP.Present && !envelope.MCP.Enabled {
		result.Skipped = append(result.Skipped, namedEntry{What: "mcp.json"})
	}
	for name := range envelope.MCP.InvalidServer {
		result.Skipped = append(result.Skipped, namedEntry{What: "mcp.json#" + name})
	}
	for _, diagnostic := range envelope.Diagnostics {
		switch diagnostic.Code {
		case "plugin_unknown_field", "plugin_extensions_ignored":
			result.Reported = append(result.Reported, namedEntry{Field: diagnostic.Item})
		}
	}
	sort.Slice(result.Skipped, func(i, j int) bool { return result.Skipped[i].What < result.Skipped[j].What })
	sort.Slice(result.Reported, func(i, j int) bool { return result.Reported[i].Field < result.Reported[j].Field })
	return result, nil
}

func emptyReport() loadReport {
	result := loadReport{Skipped: []namedEntry{}, Reported: []namedEntry{}}
	result.Loaded.Skills = []string{}
	result.Loaded.MCPServers = []string{}
	return result
}
