package transaction

// Durable state decisions and receipt identity helpers.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// persistCommitDecision handles the ambiguous failure window where an atomic
// state rename is visible but syncing its parent directory reports an error.
// The caller may roll back only when a reload proves the exact old state is
// still authoritative. A visible desired state is re-saved successfully before
// the native directory transaction is allowed to commit. All other outcomes
// leave the directory journal and backup intact for recovery.
func (kernel Kernel) persistCommitDecision(desired domain.StateFileV2, beforeJSON []byte) (oldState bool, err error) {
	initialErr := kernel.StateStore.Save(desired)
	if initialErr == nil {
		return false, nil
	}
		observed, loadErr := kernel.StateStore.Load()
		if loadErr != nil {
			return false, fmt.Errorf("state save failed (%v) and commit visibility is unknown: %w", fmt.Sprint(initialErr), loadErr)
		}
		observedJSON, marshalErr := marshalComparableState(observed)
		if marshalErr != nil {
			return false, fmt.Errorf("state save failed (%v) and observed state cannot be compared: %w", fmt.Sprint(initialErr), marshalErr)
		}
		desiredJSON, marshalErr := marshalComparableState(desired)
		if marshalErr != nil {
			return false, fmt.Errorf("state save failed (%v) and desired state cannot be compared: %w", fmt.Sprint(initialErr), marshalErr)
		}
		if bytes.Equal(observedJSON, desiredJSON) {
			if retryErr := kernel.StateStore.Save(desired); retryErr != nil {
				return false, fmt.Errorf("state became visible after save error (%v), but durability retry failed: %w", fmt.Sprint(initialErr), retryErr)
			}
			return false, nil
		}
		if bytes.Equal(observedJSON, beforeJSON) {
			return true, initialErr
		}
		return false, fmt.Errorf("state save failed and reload matched neither exact old nor desired state: %w", initialErr)
}

// PersistStateDecision gives lifecycle state writes the same visibility and
// durability handling as directory commit decisions.
func (kernel Kernel) PersistStateDecision(before, desired domain.StateFileV2) error {
	_, err := kernel.PersistStateDecisionWithDisposition(before, desired)
	return err
}

// Load implements the small state authority consumed by native recovery.
func (kernel Kernel) Load() (domain.StateFileV2, error) { return kernel.StateStore.Load() }

// Only exact old visibility authorizes native restoration. Desired visibility
// with unresolved durability deliberately remains Unknown.
func (kernel Kernel) PersistStateDecisionWithDisposition(before, desired domain.StateFileV2) (domain.StateDecisionDisposition, error) {
	beforeJSON, err := marshalComparableState(before)
	if err != nil {
		return domain.StateDecisionUnknown, err
	}
	old, err := kernel.persistCommitDecision(desired, beforeJSON)
	if old {
		return domain.StateDecisionOld, err
	}
	if err != nil {
		return domain.StateDecisionUnknown, err
	}
	return domain.StateDecisionDesired, nil
}

func marshalComparableState(state domain.StateFileV2) ([]byte, error) {
	state.Installations = append([]domain.Installation(nil), state.Installations...)
	sort.Slice(state.Installations, func(i, j int) bool {
		return state.Installations[i].InstallationID < state.Installations[j].InstallationID
	})
	return json.Marshal(state)
}

func loadClientFromState(state domain.StateFileV2, installationID, clientBindingID string) (int, domain.ClientBinding, error) {
	if state.SchemaVersion != domain.StateSchemaVersion {
		return -1, domain.ClientBinding{}, fmt.Errorf("desired transaction state schema_version must be %d", domain.StateSchemaVersion)
	}
	for index, installation := range state.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		client, ok := installation.Clients[clientBindingID]
		if !ok {
			return -1, domain.ClientBinding{}, fmt.Errorf("client binding %q not found in desired transaction state", clientBindingID)
		}
		return index, client, nil
	}
	return -1, domain.ClientBinding{}, fmt.Errorf("installation %q not found in desired transaction state", installationID)
}

func (kernel Kernel) loadClient(installationID, clientBindingID string) (domain.StateFileV2, int, domain.ClientBinding, error) {
	state, err := kernel.StateStore.Load()
	if err != nil {
		return domain.StateFileV2{}, -1, domain.ClientBinding{}, err
	}
	for index, installation := range state.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		client, ok := installation.Clients[clientBindingID]
		if !ok {
			return domain.StateFileV2{}, -1, domain.ClientBinding{}, fmt.Errorf("client binding %q not found", clientBindingID)
		}
		return state, index, client, nil
	}
	return domain.StateFileV2{}, -1, domain.ClientBinding{}, fmt.Errorf("installation %q not found", installationID)
}

func replaceReceipt(client domain.ClientBinding, receipt domain.MutationReceipt) domain.ClientBinding {
	for index := range client.Receipts {
		if client.Receipts[index].OperationID == receipt.OperationID {
			client.Receipts[index] = receipt
			return client
		}
	}
	client.Receipts = append(client.Receipts, receipt)
	return client
}

func matchesDirectoryReceipt(receipt domain.MutationReceipt, directory dirswap.Receipt, mutationType string) bool {
	return sameDirectoryProof(receipt.DirectoryProof, directory) &&
		sameReceiptOwners(receipt.ProfileOwners, directory.ProfileOwners) &&
		receipt.DataReceiptID == directory.DataReceiptID &&
		receipt.OperationID == directory.OperationID &&
		receipt.ClientBindingID == directory.ClientBindingID &&
		receipt.Sequence == directory.Sequence &&
		receipt.MutationType == mutationType &&
		receipt.ActivePath == directory.ActivePath &&
		receipt.StagingPath == directory.StagingPath &&
		receipt.BackupPath == directory.BackupPath &&
		(receipt.Phase == ReceiptPhaseStateCommitted || receipt.Phase == ReceiptPhaseCommitted)
}

func findReceipt(state domain.StateFileV2, directory dirswap.Receipt) (int, string, int, bool) {
	mutationType := "directory_swap"
	if directory.Operation == dirswap.OperationRemove {
		mutationType = "directory_remove"
	}
	for receiptIndex, receipt := range state.TransactionReceipts {
		if matchesDirectoryReceipt(receipt, directory, mutationType) {
			return -1, "", receiptIndex, true
		}
	}
	for installationIndex, installation := range state.Installations {
		for clientKey, client := range installation.Clients {
			for receiptIndex, receipt := range client.Receipts {
				if matchesDirectoryReceipt(receipt, directory, mutationType) && clientKey == directory.ClientBindingID {
					return installationIndex, clientKey, receiptIndex, true
				}
			}
		}
	}
	return -1, "", -1, false
}

func operationIDExists(state domain.StateFileV2, operationID string) bool {
	for _, receipt := range state.TransactionReceipts {
		if receipt.OperationID == operationID {
			return true
		}
	}
	for _, installation := range state.Installations {
		for _, client := range installation.Clients {
			for _, receipt := range client.Receipts {
				if receipt.OperationID == operationID {
					return true
				}
			}
		}
	}
	return false
}

func requireMutationReady(store StateStore) error {
	if guard, ok := store.(interface{ RequireMutationReady() error }); ok {
		return guard.RequireMutationReady()
	}
	return nil
}

func operationGroupIDExists(state domain.StateFileV2, operationGroupID string) bool {
	for _, receipt := range state.TransactionReceipts {
		if receipt.OperationGroupID == operationGroupID {
			return true
		}
	}
	for _, installation := range state.Installations {
		if installation.OperationGroupID == operationGroupID {
			return true
		}
		for _, client := range installation.Clients {
			for _, receipt := range client.Receipts {
				if receipt.OperationGroupID == operationGroupID {
					return true
				}
			}
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
