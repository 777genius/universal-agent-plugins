package claude

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.ArtifactSelectionReader = (*Adapter)(nil)

// ManagedMCPSelectionForArtifact preserves the exact selected server set across
// legacy skills-directory and current explicit marketplace project projections.
func (adapter *Adapter) ManagedMCPSelectionForArtifact(scope domain.InstallScope, root string) (clients.SelectionLayout, error) {
	if scope == domain.ScopeUser {
		return adapter.ManagedMCPSelection(), nil
	}
	if scope != domain.ScopeProject {
		return clients.SelectionLayout{}, fmt.Errorf("the Claude managed selection requires an explicit scope")
	}
	_, err := readClaudeJSONObject(root, filepath.Join(".claude-plugin", "marketplace.json"))
	if os.IsNotExist(err) {
		manifest, err := readClaudeJSONObject(root, filepath.Join(".claude-plugin", "plugin.json"))
		if err != nil {
			return clients.SelectionLayout{}, err
		}
		if name, ok := manifest["name"].(string); !ok || name == "" {
			return clients.SelectionLayout{}, fmt.Errorf("legacy Claude project manifest has no exact plugin name")
		}
		return adapter.ManagedMCPSelection(), nil
	}
	if err != nil {
		return clients.SelectionLayout{}, err
	}
	manifest, err := readClaudeJSONObject(root, filepath.Join(claudeProjectBundleDirectory, ".claude-plugin", "plugin.json"))
	if err != nil {
		return clients.SelectionLayout{}, err
	}
	name, ok := manifest["name"].(string)
	if !ok || name == "" {
		return clients.SelectionLayout{}, fmt.Errorf("the Claude project bundle has no plugin name")
	}
	if current, err := hasClaudeProjectMarketplace(root, name, filepath.Base(root)); err != nil || !current {
		return clients.SelectionLayout{}, fmt.Errorf("the Claude project marketplace selection marker is not exact")
	}
	return clients.SelectionLayout{File: filepath.Join(claudeProjectBundleDirectory, ".mcp.json")}, nil
}
