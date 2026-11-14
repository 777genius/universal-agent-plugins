package shared

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// ManagedMarketplaceName is the deterministic native marketplace identity of a
// managed package. It is derived from the physical artifact id so that the same
// artifact always claims the same namespace, and two different artifacts never
// collide in a client registry.
func ManagedMarketplaceName(physicalArtifactID string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(physicalArtifactID)))
	return "agentplugins-" + hex.EncodeToString(sum[:6])
}
