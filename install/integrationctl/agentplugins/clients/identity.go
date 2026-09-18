package clients

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// RegistryFinding is what an identity inspection concluded about a native
// client registry. Indeterminate is not a failure: it means the observation
// could not be trusted, and the caller must not turn it into evidence of
// absence.
type RegistryFinding uint8

const (
	// RegistryClear means no competing package claims this identity.
	RegistryClear RegistryFinding = iota
	// RegistryExpected means the identity is claimed by the managed package.
	RegistryExpected
	// RegistryCollision means a foreign package already claims the identity.
	RegistryCollision
	// RegistryIndeterminate means the registry could not be observed.
	RegistryIndeterminate
)

// RegistryInspector reads the client's own registry to decide whether the
// managed package identity is free, already ours, or taken.
type RegistryInspector interface {
	InspectNativeRegistry(ctx context.Context, env Env, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (RegistryFinding, error)
	// UsesNativeRegistryExecutable reports whether the inspection runs the
	// client executable. Only those clients count as having attempted a native
	// observation when the executable is missing.
	UsesNativeRegistryExecutable() bool
}

// PreparedRegistryInspector overrides how the prepared package directory is
// inspected. Without it the generic default shared.InspectUnqualifiedPluginRoot
// applies.
type PreparedRegistryInspector interface {
	InspectPreparedRegistry(plan domain.DeliveryPlan, name string, owned bool) (RegistryFinding, error)
}

// SelectionLayout says where the managed MCP selection document lives inside a
// delivered package and whether its servers sit under an "mcpServers" member.
type SelectionLayout struct {
	File   string
	Nested bool
}

// DefaultSelectionLayout is the managed MCP selection document every client
// uses unless it implements SelectionReader.
var DefaultSelectionLayout = SelectionLayout{File: "mcp.json", Nested: true}

// SelectionReader overrides the managed MCP selection layout. Without it the
// generic default is DefaultSelectionLayout.
type SelectionReader interface {
	ManagedMCPSelection() SelectionLayout
}
