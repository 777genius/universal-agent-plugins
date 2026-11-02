package transaction

import (
	"context"
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/profileauthority"
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
	changed := false
	openOperations := make(map[string]struct{}, len(open))
	for _, directoryReceipt := range open {
		openOperations[directoryReceipt.OperationID] = struct{}{}
		installationIndex, clientKey, receiptIndex, stateCommitted := findReceipt(state, directoryReceipt)
		if stateCommitted {
			// A prior atomic rename may have become visible while its parent fsync
			// returned an error. Re-saving successfully makes the state commit
			// decision durable before native backup data can be deleted.
			if err := kernel.StateStore.Save(state); err != nil {
				return fmt.Errorf("make recovered state commit durable for %s: %w", directoryReceipt.OperationID, err)
			}
		}
		if err := kernel.Directory.Recover(ctx, directoryReceipt.OperationID, stateCommitted); err != nil {
			return fmt.Errorf("recover directory mutation %s: %w", directoryReceipt.OperationID, err)
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
	// A crash can occur after the directory journal is durably removed but
	// before the state receipt is advanced from state_committed to committed.
	// At that point the state receipt is the durable commit decision and there
	// is no native operation left to recover, so finalize only that receipt.
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

func (kernel Kernel) pendingOwners(ctx context.Context, state domain.StateFileV2, open []dirswap.Receipt) ([]domain.PhysicalProfileOwner, error) {
	var owners []domain.PhysicalProfileOwner
	add := func(scope []domain.PhysicalProfileOwner) error {
		if err := kernel.validateOwners(ctx, scope); err != nil {
			return err
		}
		owners = append(owners, scope...)
		return nil
	}
	for _, j := range open {
		if j.SchemaVersion == 5 || j.SchemaVersion == 7 {
			_, _, _, matched := findReceipt(state, j)
			switch j.Phase {
			case dirswap.PhaseIntent, dirswap.PhaseBackupPending, dirswap.PhaseOldBackedUp, dirswap.PhaseActivationPending, dirswap.PhaseRollbackPending, dirswap.PhaseRolledBack:
				if matched {
					return nil, fmt.Errorf("durable state references unrecoverable physical journal phase %q", j.Phase)
				}
			case dirswap.PhaseCommitPending, dirswap.PhaseCommitted:
				if !matched {
					return nil, fmt.Errorf("physical journal committed without durable state receipt")
				}
			case dirswap.PhaseActivated:
			default:
				return nil, fmt.Errorf("unsupported physical journal phase %q", j.Phase)
			}
			if !matched {
				var receipts []domain.MutationReceipt
				receipts = append(receipts, state.TransactionReceipts...)
				for _, i := range state.Installations {
					for _, b := range i.Clients {
						receipts = append(receipts, b.Receipts...)
					}
				}
				for _, r := range receipts {
					if r.OperationID == j.OperationID && (r.Phase == ReceiptPhaseStateCommitted || r.Phase == ReceiptPhaseCommitted) {
						return nil, fmt.Errorf("physical journal and durable state receipt disagree")
					}
				}
			}
		}
		var scope []domain.PhysicalProfileOwner
		for _, o := range j.ProfileOwners {
			token, err := profileauthority.FromNeutral(*o.Authority)
			if err != nil {
				return nil, err
			}
			scope = append(scope, domain.PhysicalProfileOwner{Namespace: o.Namespace, InstallationID: o.InstallationID, ClientID: o.ClientID, ClientBindingID: o.ClientBindingID, Authority: &token})
		}
		if err := add(scope); err != nil {
			return nil, err
		}
	}
	for _, r := range state.TransactionReceipts {
		if r.Phase == ReceiptPhaseStateCommitted {
			if err := kernel.verifyReceiptProof(r); err != nil {
				return nil, err
			}
			if err := add(r.ProfileOwners); err != nil {
				return nil, err
			}
		}
	}
	for _, installation := range state.Installations {
		for key, b := range installation.Clients {
			for _, r := range b.Receipts {
				if r.Phase != ReceiptPhaseStateCommitted {
					continue
				}
				{
					scope, err := kernel.bindingOwner(ctx, installation.InstallationID, key, b)
					if err != nil {
						return nil, err
					}
					if len(scope) > 0 {
						found := false
						for _, o := range r.ProfileOwners {
							if o.Namespace == b.ProfileNamespace && o.InstallationID == installation.InstallationID && o.ClientID == b.ClientID && o.ClientBindingID == key && domain.SameProfileAuthority(o.Authority, b.ProfileAuthority) {
								found = true
							}
						}
						if !found {
							return nil, fmt.Errorf("pending receipt differs from recorded physical owner")
						}
					}
					if err := kernel.verifyReceiptProof(r); err != nil {
						return nil, err
					}
					if b.ProfileAuthority != nil && len(r.ProfileOwners) == 0 {
						return nil, fmt.Errorf("pending opted receipt lacks physical owners")
					}
					if err := add(r.ProfileOwners); err != nil {
						return nil, err
					}
				}
			}
			for _, j := range open {
				if j.ClientBindingID == key {
					scope, err := kernel.bindingOwner(ctx, installation.InstallationID, key, b)
					if err != nil {
						return nil, err
					}
					if len(scope) > 0 {
						found := false
						for _, o := range j.ProfileOwners {
							if o.InstallationID == installation.InstallationID && o.ClientBindingID == key && o.ClientID == b.ClientID && o.Namespace == b.ProfileNamespace && o.Authority != nil {
								token, err := profileauthority.FromNeutral(*o.Authority)
								found = err == nil && token.Equal(*b.ProfileAuthority)
							}
						}
						if !found {
							return nil, fmt.Errorf("opted journal differs from recorded physical owner")
						}
					}
				}
			}
			if b.PendingNativeIntent != nil || b.NativeActivationAttempt != "" {
				if b.PendingNativeIntent != nil {
					if err := b.PendingNativeIntent.Validate(b); err != nil {
						return nil, err
					}
				}
				scope, err := kernel.bindingOwner(ctx, installation.InstallationID, key, b)
				if err != nil {
					return nil, err
				}
				if err := add(scope); err != nil {
					return nil, err
				}
			}
		}
	}
	// The same owner may appear in several pending decisions; retain one immutable scope.
	unique := map[[4]string]domain.PhysicalProfileOwner{}
	for _, o := range owners {
		key := [4]string{o.Namespace, o.InstallationID, o.ClientID, o.ClientBindingID}
		if old, ok := unique[key]; ok && !domain.SameProfileAuthority(old.Authority, o.Authority) {
			return nil, fmt.Errorf("pending owners disagree")
		}
		unique[key] = o
	}
	owners = nil
	for _, o := range unique {
		owners = append(owners, o)
	}
	return owners, nil
}
