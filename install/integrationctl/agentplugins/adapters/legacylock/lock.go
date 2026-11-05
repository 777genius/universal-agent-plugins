// Package legacylock adapts the historical integrationctl state lock to the
// agentplugins use-case port without replacing the disk-lock implementation.
package legacylock

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// StateLock retains the legacy lifecycle's original "state" lock namespace.
// Manager is the existing lock; this type introduces no new locking engine.
type StateLock struct {
	Manager legacyports.LockManager
}

var _ ports.MutationLock = StateLock{}

func (lock StateLock) Acquire(ctx context.Context) (ports.UnlockFunc, error) {
	if lock.Manager == nil {
		return nil, fmt.Errorf("legacy lock manager is required")
	}
	release, err := lock.Manager.Acquire(ctx, "state")
	if err != nil {
		return nil, err
	}
	if release == nil {
		return nil, fmt.Errorf("legacy lock manager returned no release function")
	}
	return ports.UnlockFunc(release), nil
}
