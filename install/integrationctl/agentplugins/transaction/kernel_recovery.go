package transaction

// Directory journal replay and state receipt finalization belong to recovery,
// not to the mutation entrypoints.

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (kernel Kernel) Recover(ctx context.Context) error {
	if kernel.StateStore == nil {
		return fmt.Errorf("transaction state store is required")
	}
	state, err := kernel.StateStore.Load()
	if err != nil {
		return err
	}
	kernel.Directory.Namespace = kernel.Namespace
	open, err := kernel.Directory.ListOpen()
	if err != nil {
		return err
	}
	owners, err := kernel.pendingOwners(ctx, state, open)
	if err != nil {
		return err
	}
	kernel = kernel.guardOwners(ctx, owners)
	changed, openOperations, err := kernel.recoverDirectoryJournals(ctx, &state, open)
	if err != nil {
		return err
	}
	if finalizeReceiptsWithoutJournals(&state, openOperations) {
		changed = true
	}
	if changed {
		if err := kernel.StateStore.Save(state); err != nil {
			return err
		}
	}
	if kernel.NativeRecovery != nil {
		return kernel.NativeRecovery.Recover(ctx)
	}
	return nil
}

func (kernel Kernel) recoverDirectoryJournals(ctx context.Context, state *domain.StateFileV2, open []dirswap.Receipt) (bool, map[string]struct{}, error) {
	changed := false
	openOperations := make(map[string]struct{}, len(open))
	for _, directoryReceipt := range open {
		openOperations[directoryReceipt.OperationID] = struct{}{}
		installationIndex, clientKey, receiptIndex, stateCommitted := findReceipt(*state, directoryReceipt)
		if stateCommitted {
			// A prior atomic rename may have become visible while its parent fsync
			// returned an error. Re-saving makes the decision durable before backup deletion.
			if err := kernel.StateStore.Save(*state); err != nil {
				return false, nil, fmt.Errorf("make recovered state commit durable for %s: %w", directoryReceipt.OperationID, err)
			}
		}
		if err := kernel.Directory.Recover(ctx, directoryReceipt.OperationID, stateCommitted); err != nil {
			return false, nil, fmt.Errorf("recover directory mutation %s: %w", directoryReceipt.OperationID, err)
		}
		if stateCommitted {
			if installationIndex < 0 {
				state.TransactionReceipts[receiptIndex].Phase = ReceiptPhaseCommitted
			} else {
				installation := state.Installations[installationIndex]
				client := installation.Clients[clientKey]
				client.Receipts[receiptIndex].Phase = ReceiptPhaseCommitted
				installation.Clients[clientKey] = client
				state.Installations[installationIndex] = installation
			}
			changed = true
		}
	}
	return changed, openOperations, nil
}

// A crash can follow journal removal but precede the state receipt advance.
// With no native operation remaining, only finalize the durable state receipt.
func finalizeReceiptsWithoutJournals(state *domain.StateFileV2, openOperations map[string]struct{}) bool {
	changed := false
	for index := range state.TransactionReceipts {
		receipt := &state.TransactionReceipts[index]
		if receipt.Phase == ReceiptPhaseStateCommitted {
			if _, stillOpen := openOperations[receipt.OperationID]; !stillOpen {
				receipt.Phase = ReceiptPhaseCommitted
				changed = true
			}
		}
	}
	for installationIndex, installation := range state.Installations {
		for clientKey, client := range installation.Clients {
			clientChanged := false
			for receiptIndex := range client.Receipts {
				receipt := &client.Receipts[receiptIndex]
				if receipt.Phase != ReceiptPhaseStateCommitted {
					continue
				}
				if _, stillOpen := openOperations[receipt.OperationID]; stillOpen {
					continue
				}
				receipt.Phase = ReceiptPhaseCommitted
				clientChanged = true
			}
			if clientChanged {
				installation.Clients[clientKey] = client
				changed = true
			}
		}
		state.Installations[installationIndex] = installation
	}
	return changed
}
