package providers

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

// ManagedMarketplaceName returns the deterministic native marketplace identity
// used by client adapters and read-only reconciliation output.
func ManagedMarketplaceName(physicalArtifactID string) string {
	return shared.ManagedMarketplaceName(physicalArtifactID)
}

// copilotMarketplaceVersion stays in providers until Part 8 moves identity
// inspection. Copilot and VS Code share it, so the implementation lives in
// clients/shared rather than either client package.
func copilotMarketplaceVersion(version string) string {
	return shared.MarketplaceVersion(version)
}
