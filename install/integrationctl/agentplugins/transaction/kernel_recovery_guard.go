package transaction

// Read-only recovery authority preflight.

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// PrevalidateRecovery is inert and covers the entire pending durability decision.
func (kernel Kernel) PrevalidateRecovery(ctx context.Context) error {
	if kernel.StateStore == nil {
		return fmt.Errorf("transaction state store is required")
	}
	state, err := kernel.StateStore.Load()
	if err != nil {
		return err
	}
	if err := kernel.validatePublicationRecovery(ctx, state); err != nil {
		return err
	}
	if kernel.Directory.JournalDir == "" {
		owners, err := kernel.pendingOwners(ctx, state, nil)
		if err != nil {
			return err
		}
		if len(owners) != 0 || len(kernel.Directory.RequiredOwners) != 0 {
			return fmt.Errorf("physical recovery journal dir is required")
		}
		for _, installation := range state.Installations {
			for _, binding := range installation.Clients {
				if binding.ProfileAuthority != nil {
					return fmt.Errorf("physical recovery journal dir is required")
				}
			}
		}
		return nil
	}
	kernel.Directory.Namespace = kernel.Namespace
	open, err := kernel.Directory.ListOpen()
	if err != nil {
		return err
	}
	_, err = kernel.pendingOwners(ctx, state, open)
	return err
}

// State-only decisions need original ancestry, while replay also needs its manager.
func (kernel Kernel) validatePublicationRecovery(ctx context.Context, state domain.StateFileV2) error {
	if kernel.Directory.JournalDir == "" {
		for _, receipt := range state.TransactionReceipts {
			if receipt.PublicationParentAuthority != nil {
				if err := ctx.Err(); err != nil {
					return err
				}
				return fmt.Errorf("publication recovery journal dir is required")
			}
		}
	}
	return kernel.validatePublicationParents(ctx, state)
}
