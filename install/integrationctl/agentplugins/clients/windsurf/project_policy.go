package windsurf

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// This observes only selected project files. It never initiates Devin's native
// migration, edits a local override, or interprets unknown imported policy.
func checkDevinProjectPolicy(root string, components []domain.ComponentDecision, kernel nativeconfig.Kernel) error {
	for _, name := range []string{"config.json", "config.local.json"} {
		document, err := readDevinProjectDocument(filepath.Join(root, name), kernel)
		if err != nil {
			return err
		}
		if raw, exists := document["mcpServers"]; exists {
			entries, ok := raw.(map[string]any)
			if !ok || len(entries) > 0 {
				return fmt.Errorf("%w: Devin project main config has legacy mcpServers; migrate separately with the selected native client before automatic installation", nativeconfig.ErrNativeMigrationRequired)
			}
		}
	}
	document, err := readDevinProjectDocument(filepath.Join(root, "mcp_config.local.json"), kernel)
	if err != nil {
		return err
	}
	raw, exists := document["mcpServers"]
	if !exists {
		return nil
	}
	entries, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("devin local MCP override must contain an object mcpServers")
	}
	for _, component := range components {
		if component.Kind != domain.ComponentMCPServer || component.Support == domain.SupportUnsupported {
			continue
		}
		if _, shadowed := entries[component.Name]; shadowed {
			return fmt.Errorf("devin local MCP override shadows selected server %q; resolve policy separately", component.Name)
		}
	}
	return nil
}

func readDevinProjectDocument(path string, kernel nativeconfig.Kernel) (map[string]any, error) {
	file, err := kernel.ReadExactFile(path)
	if err != nil {
		return nil, err
	}
	if !file.Exists {
		return map[string]any{}, nil
	}
	standard, err := nativeconfig.StandardizeCommentedJSON(file.Body)
	if err != nil {
		return nil, err
	}
	return shared.DecodeStrictJSONObject(standard)
}
