package providers

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/all"
)

// testStager completes a stager literal with the production path policy and
// the full client registry.
func testStager(base Stager) Stager {
	if base.Paths == nil {
		base.Paths = pathpolicy.Policy{}
	}
	if base.Registry == nil {
		base.Registry = all.Default()
	}
	return base
}

// testActivator completes an activator literal with the full client registry.
func testActivator(base Activator) Activator {
	if base.Registry == nil {
		base.Registry = all.Default()
	}
	return base
}
