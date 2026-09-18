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
