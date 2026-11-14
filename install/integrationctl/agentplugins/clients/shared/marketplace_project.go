package shared

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// MarketplaceVersion is the version string Copilot and VS Code write into a
// managed marketplace entry. An empty authored version would be rejected by
// those clients, so it is projected as 0.0.0 instead of omitted.
func MarketplaceVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "0.0.0"
	}
	return version
}

// ProjectCopilotMarketplace writes the GitHub Copilot / VS Code marketplace
// document that both adapters of that backend family deliver. It lives here
// because clients/copilot and clients/vscode may not import each other.
func ProjectCopilotMarketplace(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) error {
	version := MarketplaceVersion(envelope.Manifest.Version)
	description := strings.TrimSpace(envelope.Manifest.Description)
	if description == "" {
		description = "Managed Agent Plugin " + envelope.Manifest.Name
	}
	document := map[string]any{
		"name":  ManagedMarketplaceName(plan.PhysicalArtifactID),
		"owner": map[string]any{"name": "Agent Plugins CLI"},
		"metadata": map[string]any{
			"description": "Managed local marketplace for " + envelope.Manifest.Name,
			"version":     version,
		},
		"plugins": []map[string]any{{
			"name":        envelope.Manifest.Name,
			"description": description,
			"version":     version,
			"source":      ".",
		}},
	}
	if err := WriteJSON(filepath.Join(root, ".github", "plugin", "marketplace.json"), document); err != nil {
		return fmt.Errorf("write Copilot managed marketplace: %w", err)
	}
	return nil
}

// ProjectCodexMarketplace writes the Codex / ChatGPT managed marketplace
// document. Both clients share the OpenAI marketplace layout, so the write
// belongs to neither adapter package.
func ProjectCodexMarketplace(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) error {
	document := map[string]any{
		"name": ManagedMarketplaceName(plan.PhysicalArtifactID),
		"plugins": []map[string]any{{
			"name": envelope.Manifest.Name,
			"source": map[string]any{
				"source": "local",
				"path":   "./",
			},
			"policy": map[string]any{
				"installation":   "AVAILABLE",
				"authentication": "ON_INSTALL",
			},
			"category": "Productivity",
		}},
	}
	if err := WriteJSON(filepath.Join(root, ".agents", "plugins", "marketplace.json"), document); err != nil {
		return fmt.Errorf("write Codex managed marketplace: %w", err)
	}
	return nil
}
