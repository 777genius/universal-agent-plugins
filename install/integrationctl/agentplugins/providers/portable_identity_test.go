package providers

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"testing"
)

func TestPortableIdentityPreservesMarketplaceGolden(t *testing.T) {
	physical := domain.ComputePhysicalArtifactID("demo", "00000000-0000-4000-8000-000000000001")
	if physical != "demo-11e594f48195" {
		t.Fatalf("historical physical id changed: %s", physical)
	}
	if got := ManagedMarketplaceName(physical); got != "agentplugins-4293497808d3" {
		t.Fatalf("historical marketplace id changed: %s", got)
	}
}
