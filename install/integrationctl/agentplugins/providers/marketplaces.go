package providers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func managedMarketplaceName(physicalArtifactID string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(physicalArtifactID)))
	return "agentplugins-" + hex.EncodeToString(sum[:6])
}

// ManagedMarketplaceName returns the deterministic native marketplace identity
// used by client adapters and read-only reconciliation output.
func ManagedMarketplaceName(physicalArtifactID string) string {
	return managedMarketplaceName(physicalArtifactID)
}

func copilotMarketplaceVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "0.0.0"
	}
	return version
}

func projectCopilotMarketplace(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) error {
	version := copilotMarketplaceVersion(envelope.Manifest.Version)
	description := strings.TrimSpace(envelope.Manifest.Description)
	if description == "" {
		description = "Managed Agent Plugin " + envelope.Manifest.Name
	}
	document := map[string]any{
		"name":  managedMarketplaceName(plan.PhysicalArtifactID),
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
	if err := writeJSON(filepath.Join(root, ".github", "plugin", "marketplace.json"), document); err != nil {
		return fmt.Errorf("write Copilot managed marketplace: %w", err)
	}
	return nil
}

func projectCodexMarketplace(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) error {
	document := map[string]any{
		"name": managedMarketplaceName(plan.PhysicalArtifactID),
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
	if err := writeJSON(filepath.Join(root, ".agents", "plugins", "marketplace.json"), document); err != nil {
		return fmt.Errorf("write Codex managed marketplace: %w", err)
	}
	return nil
}
