package transaction

// Pending physical owner scopes are gathered and validated before replay.
// Validation order is intentional: the first bad journal/receipt stops recovery.

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/profileauthority"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (kernel Kernel) pendingOwners(ctx context.Context, state domain.StateFileV2, open []dirswap.Receipt) ([]domain.PhysicalProfileOwner, error) {
	var owners []domain.PhysicalProfileOwner
	add := func(scope []domain.PhysicalProfileOwner) error {
		if err := kernel.validateOwners(ctx, scope); err != nil {
			return err
		}
		owners = append(owners, scope...)
		return nil
	}
	for _, journal := range open {
		if err := checkJournalStateAgreement(state, journal); err != nil {
			return nil, err
		}
		scope, err := physicalJournalOwners(journal)
		if err != nil {
			return nil, err
		}
		if err := add(scope); err != nil {
			return nil, err
		}
	}
	for _, receipt := range state.TransactionReceipts {
		if receipt.Phase == ReceiptPhaseStateCommitted {
			if err := kernel.verifyReceiptProof(receipt); err != nil {
				return nil, err
			}
			if err := add(receipt.ProfileOwners); err != nil {
				return nil, err
			}
		}
	}
	if err := kernel.collectPendingBindingOwners(ctx, state, open, add); err != nil {
		return nil, err
	}
	return uniquePendingPhysicalOwners(owners)
}

func checkJournalStateAgreement(state domain.StateFileV2, journal dirswap.Receipt) error {
	if journal.SchemaVersion != 5 && journal.SchemaVersion != 7 {
		return nil
	}
	_, _, _, matched := findReceipt(state, journal)
	switch journal.Phase {
	case dirswap.PhaseIntent, dirswap.PhaseBackupPending, dirswap.PhaseOldBackedUp, dirswap.PhaseActivationPending, dirswap.PhaseRollbackPending, dirswap.PhaseRolledBack:
		if matched {
			return fmt.Errorf("durable state references unrecoverable physical journal phase %q", journal.Phase)
		}
	case dirswap.PhaseCommitPending, dirswap.PhaseCommitted:
		if !matched {
			return fmt.Errorf("physical journal committed without durable state receipt")
		}
	case dirswap.PhaseActivated:
	default:
		return fmt.Errorf("unsupported physical journal phase %q", journal.Phase)
	}
	if !matched {
		var receipts []domain.MutationReceipt
		receipts = append(receipts, state.TransactionReceipts...)
		for _, installation := range state.Installations {
			for _, binding := range installation.Clients {
				receipts = append(receipts, binding.Receipts...)
			}
		}
		for _, receipt := range receipts {
			if receipt.OperationID == journal.OperationID && (receipt.Phase == ReceiptPhaseStateCommitted || receipt.Phase == ReceiptPhaseCommitted) {
				return fmt.Errorf("physical journal and durable state receipt disagree")
			}
		}
	}
	return nil
}

func physicalJournalOwners(journal dirswap.Receipt) ([]domain.PhysicalProfileOwner, error) {
	var owners []domain.PhysicalProfileOwner
	for _, owner := range journal.ProfileOwners {
		token, err := profileauthority.FromNeutral(*owner.Authority)
		if err != nil {
			return nil, err
		}
		owners = append(owners, domain.PhysicalProfileOwner{Namespace: owner.Namespace, InstallationID: owner.InstallationID, ClientID: owner.ClientID, ClientBindingID: owner.ClientBindingID, Authority: &token})
	}
	return owners, nil
}

func (kernel Kernel) collectPendingBindingOwners(ctx context.Context, state domain.StateFileV2, open []dirswap.Receipt, add func([]domain.PhysicalProfileOwner) error) error {
	for _, installation := range state.Installations {
		for key, binding := range installation.Clients {
			for _, receipt := range binding.Receipts {
				if receipt.Phase != ReceiptPhaseStateCommitted {
					continue
				}
				if err := kernel.checkPendingBindingReceipt(ctx, installation.InstallationID, key, binding, receipt, add); err != nil {
					return err
				}
			}
			for _, journal := range open {
				if journal.ClientBindingID == key {
					if err := kernel.checkPendingBindingJournal(ctx, installation.InstallationID, key, binding, journal); err != nil {
						return err
					}
				}
			}
			if binding.PendingNativeIntent != nil || binding.NativeActivationAttempt != "" {
				if err := kernel.checkPendingNativeBinding(ctx, installation.InstallationID, key, binding, add); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (kernel Kernel) checkPendingBindingReceipt(ctx context.Context, installationID, key string, binding domain.ClientBinding, receipt domain.MutationReceipt, add func([]domain.PhysicalProfileOwner) error) error {
	scope, err := kernel.bindingOwner(ctx, installationID, key, binding)
	if err != nil {
		return err
	}
	if len(scope) > 0 {
		found := false
		for _, owner := range receipt.ProfileOwners {
			if owner.Namespace == binding.ProfileNamespace && owner.InstallationID == installationID && owner.ClientID == binding.ClientID && owner.ClientBindingID == key && domain.SameProfileAuthority(owner.Authority, binding.ProfileAuthority) {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("pending receipt differs from recorded physical owner")
		}
	}
	if err := kernel.verifyReceiptProof(receipt); err != nil {
		return err
	}
	if binding.ProfileAuthority != nil && len(receipt.ProfileOwners) == 0 {
		return fmt.Errorf("pending opted receipt lacks physical owners")
	}
	return add(receipt.ProfileOwners)
}

func (kernel Kernel) checkPendingBindingJournal(ctx context.Context, installationID, key string, binding domain.ClientBinding, journal dirswap.Receipt) error {
	scope, err := kernel.bindingOwner(ctx, installationID, key, binding)
	if err != nil {
		return err
	}
	if len(scope) > 0 {
		found := false
		for _, owner := range journal.ProfileOwners {
			if owner.InstallationID == installationID && owner.ClientBindingID == key && owner.ClientID == binding.ClientID && owner.Namespace == binding.ProfileNamespace && owner.Authority != nil {
				token, err := profileauthority.FromNeutral(*owner.Authority)
				found = err == nil && token.Equal(*binding.ProfileAuthority)
			}
		}
		if !found {
			return fmt.Errorf("opted journal differs from recorded physical owner")
		}
	}
	return nil
}

func (kernel Kernel) checkPendingNativeBinding(ctx context.Context, installationID, key string, binding domain.ClientBinding, add func([]domain.PhysicalProfileOwner) error) error {
	if binding.PendingNativeIntent != nil {
		if err := binding.PendingNativeIntent.Validate(binding); err != nil {
			return err
		}
	}
	scope, err := kernel.bindingOwner(ctx, installationID, key, binding)
	if err != nil {
		return err
	}
	return add(scope)
}

func uniquePendingPhysicalOwners(owners []domain.PhysicalProfileOwner) ([]domain.PhysicalProfileOwner, error) {
	unique := map[[4]string]domain.PhysicalProfileOwner{}
	for _, owner := range owners {
		key := [4]string{owner.Namespace, owner.InstallationID, owner.ClientID, owner.ClientBindingID}
		if old, ok := unique[key]; ok && !domain.SameProfileAuthority(old.Authority, owner.Authority) {
			return nil, fmt.Errorf("pending owners disagree")
		}
		unique[key] = owner
	}
	var result []domain.PhysicalProfileOwner
	for _, owner := range unique {
		result = append(result, owner)
	}
	return result, nil
}
