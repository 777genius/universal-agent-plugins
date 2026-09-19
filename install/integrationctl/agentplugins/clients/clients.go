// Package clients is the extension point of the install core: one adapter per
// supported client, reached through a Registry that the composition root
// injects. Generic packages (providers, planner, adapters/clientdetect) depend
// on this contract and never on a concrete client package.
//
// Capabilities are segregated: an adapter implements only the interfaces its
// client actually has, and a dispatcher asks for one with As[T]. Because that
// lookup is a runtime type assertion, every adapter is expected to carry
// compile-time assertions (`var _ clients.Lifecycle = (*Adapter)(nil)`) for the
// capabilities it declares, and contracttest verifies the declaration against
// the implementation.
//
// The package is close to self-contained but not a leaf: Env carries a
// nativeconfig.Kernel, which pulls github.com/tailscale/hujson and
// adapters/atomicfile. That is a known, accepted exception - see
// docs/ARCHITECTURE.md.
package clients

import (
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// Adapter is the only mandatory part of the contract. Identity metadata is not
// duplicated here: Definition and traits stay in domain, so Directory
// validation and the CLI can read them without building a registry.
type Adapter interface {
	ID() domain.ClientID
}

// Env carries the infrastructure an adapter is allowed to use. It is passed in
// by the dispatcher rather than owned by the adapter, so a client package never
// reaches for a process, a clock or the filesystem policy on its own.
type Env struct {
	Runner       ports.CommandRunner
	NativeConfig nativeconfig.Kernel
	Paths        ports.PathPolicy
	Launcher     StdioLauncherDeliverer
	Now          func() time.Time
}

// StdioLauncherDeliverer copies the managed stdio launcher into a staged tree.
// It is an interface rather than *managedstdio.Source so that the contract does
// not depend on the launcher implementation.
type StdioLauncherDeliverer interface {
	Deliver(root string) error
}
