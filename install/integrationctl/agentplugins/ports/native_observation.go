package ports

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// NativeTransitionObservation is a transient observation of an existing
// client-owned operation. It grants no authority when reconstructed from JSON.
type NativeTransitionObservation struct {
	OperationID, InstallationID, BindingID, TargetPath, Phase, Digest, Root string
	// AuthorityDigest remains stable through the recorder's finite phase changes.
	AuthorityDigest, EnvironmentID string
}

type NativeTransitionObserver interface {
	Pending() ([]NativeTransitionObservation, error)
}

// ObservedNativeTransitionRecovery runs one existing recorder within the
// facade's observed state fence, under the already held mutation lock.
type ObservedNativeTransitionRecovery interface {
	NativeTransitionRecovery
	NativeTransitionObserver
	ReconcileObserved(context.Context, NativeTransitionObservation, NativeTransitionState) error
}

type NativePackageVerifier interface {
	Verify(context.Context, string, string) error
}

// NativeTransitionOwner joins observation ownership only to the current typed
// environment receipts. Native journal text cannot select an environment.
func NativeTransitionOwner(state domain.StateFileV2, item NativeTransitionObservation) (string, bool) {
	matched := false
	for _, installation := range state.Installations {
		if installation.InstallationID == item.InstallationID {
			binding, found := installation.Clients[item.BindingID]
			if matched || !found || binding.ClientBindingID != item.BindingID || binding.TargetLocator == "" || binding.TargetLocator != item.TargetPath {
				return "", false
			}
			matched = true
		}
	}
	if !matched {
		return "", false
	}
	owner, found := "", false
	for _, receipt := range state.EnvironmentReceipts {
		if receipt.InstallationID != item.InstallationID || receipt.ClientBindingID != item.BindingID {
			continue
		}
		if found || receipt.EnvironmentID == "" {
			return "", false
		}
		owner, found = receipt.EnvironmentID, true
	}
	return owner, true
}
