package providers

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

// ManagedMarketplaceName returns the deterministic native marketplace identity
// used by client adapters and read-only reconciliation output.
func ManagedMarketplaceName(physicalArtifactID string) string {
	return shared.ManagedMarketplaceName(physicalArtifactID)
}
