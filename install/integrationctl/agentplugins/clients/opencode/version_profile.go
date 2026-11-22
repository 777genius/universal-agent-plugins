package opencode

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

var (
	_ clients.VersionProbeEnvironment       = (*Adapter)(nil)
	_ clients.DisposableVersionProbeProfile = (*Adapter)(nil)
)

// OpenCode initializes logs and caches before processing --version.
func (*Adapter) RequiresDisposableVersionProbeProfile() bool { return true }

func (*Adapter) VersionProbeEnvironment(root string) ([]string, error) {
	return shared.PrepareOpenCodeVersionProbeEnvironment(root)
}
