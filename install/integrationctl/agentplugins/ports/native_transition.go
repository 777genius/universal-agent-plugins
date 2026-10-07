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
