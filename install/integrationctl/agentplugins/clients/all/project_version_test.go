package all

import (
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestDefaultRegistryKeepsProjectVersionAndHostEnvironmentCapabilitiesFinite(t *testing.T) {
	registry := Default()
	for _, adapter := range registry.All() {
		id := adapter.ID()
		requirement, requires := clients.As[clients.ProjectVersionRequirement](registry, id)
		if requires != (id == domain.ClientKimi || id == domain.ClientWindsurf) {
			t.Fatalf("unexpected project version capability for %s: %v", id, requires)
		}
		if requires {
			client := domain.DetectedClient{ClientID: id, ExecutablePath: filepath.Join(t.TempDir(), "devin")}
			if !requirement.RequiresProjectVersion(client) {
				t.Fatalf("reviewed project host no longer requests its version: %s", id)
			}
		}
		_, hostEnvironment := clients.As[clients.HostVersionProbeEnvironment](registry, id)
		if hostEnvironment != (id == domain.ClientWindsurf) {
			t.Fatalf("unexpected original-host version environment for %s: %v", id, hostEnvironment)
		}
	}
}
