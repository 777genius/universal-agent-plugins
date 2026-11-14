// Package exampleclient is a test-only adapter used to prove that generic
// dispatchers accept a registry they did not assemble. It is not registered in
// clients/all: adding a production client still requires domain, clients/<id>,
// and clients/all. This package is the out-of-tree shape of the third step.
package exampleclient

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Adapter implements the mandatory contract plus host detection. It reuses an
// existing client id so domain stays the identity authority; the point is that
// planner and providers never import this package.
type Adapter struct{}

// New builds the example adapter.
func New() *Adapter { return &Adapter{} }

var (
	_ clients.Adapter      = (*Adapter)(nil)
	_ clients.HostDetector = (*Adapter)(nil)
)

// ID reports the client this adapter can stand in for in a subset registry.
// The string form keeps this package off the ClientID-selector budget: the
// identity still has to be one domain already defines, or NewRegistry rejects it.
func (*Adapter) ID() domain.ClientID { return domain.ClientID("cursor") }

// DetectSurfaces reports a single configuration directory the host provided.
func (*Adapter) DetectSurfaces(host clients.Host) clients.Detection {
	root := host.HomeDir()
	return clients.Detection{
		ConfigRoot: root,
		Surfaces:   []domain.ClientSurface{host.DirectorySurface("example_config", root)},
	}
}
