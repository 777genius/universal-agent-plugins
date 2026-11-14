package clients

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Lifecycle activates and deactivates one client. The generic dispatcher keeps
// the invariants that hold for every client (plan and delivery agree on the
// client, the active path is a contained real directory) and delegates the rest
// here. A client without Lifecycle is activated manually by the user.
type Lifecycle interface {
	Activate(ctx context.Context, env Env, req domain.ActivationRequest) (domain.ActivationOutcome, error)
	Deactivate(ctx context.Context, env Env, req domain.DeactivationRequest) (domain.DeactivationOutcome, error)
}

// ActivationPreflighter rejects a request before anything is written, so a
// missing client CLI or an unusable native config fails the operation instead
// of leaving a half-activated install behind.
type ActivationPreflighter interface {
	PreflightActivation(env Env, req domain.ActivationRequest) error
}

// PluginEnablement declares a separate explicit consent supported by an
// adapter. It never promotes the ordinary confirmation or activation state.
type PluginEnablement interface {
	SupportsPluginEnablement(domain.InstallScope) bool
}

func SupportsPluginEnablement(registry *Registry, id domain.ClientID, scope domain.InstallScope) bool {
	adapter, ok := As[PluginEnablement](registry, id)
	return ok && adapter.SupportsPluginEnablement(scope)
}

// RemovalAttestationPolicy reports whether an owned binding requires an explicit
// host confirmation that its native plugin was removed or never activated. The
// answer is read-only policy, independent of Confirmed or ExternalUninstalled.
type RemovalAttestationPolicy interface {
	RequiresExternalUninstalled(env Env, req domain.DeactivationRequest) bool
}

// AutomaticActivator reports whether Activate will drive a managed client CLI
// for this exact request. Runtime preflight reads the same predicate so the two
// cannot drift.
type AutomaticActivator interface {
	AutomaticallyActivates(env Env, req domain.ActivationRequest) bool
}

// ReadOnlyVerifier reports whether this client can be verified without writing,
// which is what lets a read-only reconciliation trust its own observation.
type ReadOnlyVerifier interface {
	VerifierAvailable(client domain.DetectedClient, plan domain.DeliveryPlan, backendExecutable string) bool
}

// IndependentActivator attests that an adapter's activation effects do not
// write shared installation state or invoke host handoff callbacks.
type IndependentActivator interface {
	SupportsIndependentActivation(domain.ActivationRequest) bool
}
