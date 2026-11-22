package opencode

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

var (
	_ clients.VersionProbeEnvironment       = (*Adapter)(nil)
	_ clients.DisposableVersionProbeProfile = (*Adapter)(nil)
	_ clients.AliasedVersionProbeProfile    = (*Adapter)(nil)
)

// OpenCode initializes logs and caches before processing --version.
func (*Adapter) RequiresDisposableVersionProbeProfile() bool { return true }

// The version probe uses only its private profile, so a symlink in the
// selected configuration path does not change the executable it observes.
func (*Adapter) AllowsAliasedVersionProbeProfile() bool { return true }

func (*Adapter) VersionProbeEnvironment(root string) ([]string, error) {
	return shared.PrepareOpenCodeVersionProbeEnvironment(root)
}
