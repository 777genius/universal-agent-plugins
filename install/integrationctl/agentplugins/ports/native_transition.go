package ports

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type NativeTransitionRecovery interface{ Recover(context.Context) error }
type NativeTransitionState interface {
	Load() (domain.StateFileV2, error)
	PersistStateDecisionWithDisposition(domain.StateFileV2, domain.StateFileV2) (domain.StateDecisionDisposition, error)
}
type OpenCodeTransitionRecorder interface {
	PersistPrepared(context.Context, domain.NativeAttemptIdentity, string, domain.OpenCodeTransitionPrepared, []domain.NativeObjectOwnership, []domain.NativeObjectOwnership) error
	Reconcile(context.Context, string) (domain.NativeEffectState, error)
}

// NativeTransitionActivationFence keeps generic lifecycle publication from
// changing the exact snapshot governed by a retained native record.
type NativeTransitionActivationFence interface {
	HoldsActivation(installationID, bindingID string) (bool, error)
}

// NativeProjectRoot describes one adapter-selected private runtime directory.
// Its marker is source ownership; the publisher also requires its retained
// directory transaction receipt before reusing a root.
type NativeProjectRoot struct {
	Path       string
	Marker     domain.NativeObjectOwnership
	MarkerBody []byte
}

// NativeProjectRootPublisher publishes an absent selected root using the same
// directory kernel and state store as the enclosing installation.
type NativeProjectRootPublisher interface {
	PublishProjectRoot(context.Context, domain.ActivationRequest, NativeProjectRoot) error
	VerifyProjectRoot(context.Context, string, domain.NativeObjectOwnership) error
}

// NativeRemovalDecisionRecorder records the exact ordinary removal decision
// before its state save. A handled decision must settle through Recover before
// the core can delete the managed package. The callback never commits state.
type NativeRemovalDecisionRecorder interface {
	PrepareNativeRemovalDecision(context.Context, string, string, domain.StateFileV2, domain.StateFileV2) (bool, error)
}
