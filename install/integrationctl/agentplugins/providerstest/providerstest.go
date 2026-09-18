// Package providerstest constructs stagers for tests outside the providers
// package. It exists so that a new required dependency is wired in one
// place instead of in every test that builds a stager.
package providerstest

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/all"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

// NewStager completes a stager literal with the production path policy and
// the full client registry. It deliberately offers no way to substitute a
// different path policy: a permissive one in a test would make the test prove
// nothing about containment.
func NewStager(base providers.Stager) providers.Stager {
	if base.Paths == nil {
		base.Paths = pathpolicy.Policy{}
	}
	if base.Registry == nil {
		base.Registry = all.Default()
	}
	return base
}

// NewActivator completes an activator literal with the full client registry.
func NewActivator(base providers.Activator) providers.Activator {
	if base.Registry == nil {
		base.Registry = all.Default()
	}
	return base
}
