package shared

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// ProjectCopilotMarketplace writes the managed local marketplace document that
// GitHub Copilot CLI and VS Code share. It lives in shared because those two
// client packages cannot import each other.
func ProjectCopilotMarketplace(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) error {
	version := CopilotMarketplaceVersion(envelope.Manifest.Version)
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
