package transaction

// Commit and ownership proof bookkeeping.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// A directory commit confirms only the managed package bytes. External native
// receipts remain the last confirmed effect until the client adapter reports
// its own result after this transaction.
func committedDirectoryObjects(before domain.StateFileV2, installationID, bindingID string, projected []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	var objects []domain.NativeObjectOwnership
	for _, object := range projected {
		if object.Kind == "managed_package_directory" {
			objects = append(objects, object)
		}
	}
	for _, installation := range before.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		for _, object := range installation.Clients[bindingID].NativeObjects {
			if object.Kind != "managed_package_directory" {
				objects = append(objects, object)
			}
		}
		break
	}
	return objects
}

func groupReceipts(items []appliedGroupMutation) []domain.MutationReceipt {
	result := make([]domain.MutationReceipt, 0, len(items))
	for _, item := range items {
		if item.state.OperationID != "" {
			result = append(result, item.state)
		}
	}
	return result
}

func dataBindingID(r DirectoryRemoval, owners []domain.PhysicalProfileOwner) string {
	if r.DataReceiptID != "" && len(owners) > 0 {
		return ""
	}
	return r.ClientBindingID
}

func directoryProof(r dirswap.Receipt) json.RawMessage {
	if r.SchemaVersion != 5 && r.SchemaVersion != 7 {
		return nil
	}
	raw, _ := json.Marshal(r)
	return raw
}
func sameDirectoryProof(raw json.RawMessage, r dirswap.Receipt) bool {
	if r.SchemaVersion != 5 && r.SchemaVersion != 7 {
		return len(raw) == 0
	}
	var recorded dirswap.Receipt
	if json.Unmarshal(raw, &recorded) != nil {
		return false
	}
	recorded.Phase = r.Phase
	a, _ := json.Marshal(recorded)
	b, _ := json.Marshal(r)
	return bytes.Equal(a, b)
}
func (kernel Kernel) verifyReceiptProof(r domain.MutationReceipt) error {
	if len(r.ProfileOwners) == 0 {
		if len(r.DirectoryProof) != 0 {
			return fmt.Errorf("physical proof lost owner scope")
		}
		return nil
	}
	var proof dirswap.Receipt
	if len(r.DirectoryProof) == 0 || json.Unmarshal(r.DirectoryProof, &proof) != nil || (proof.SchemaVersion != 5 && proof.SchemaVersion != 7) || proof.OperationID != r.OperationID || proof.Sequence != r.Sequence || proof.ClientBindingID != r.ClientBindingID || proof.DataReceiptID != r.DataReceiptID || proof.ActivePath != r.ActivePath || proof.StagingPath != r.StagingPath || proof.BackupPath != r.BackupPath || !sameReceiptOwners(r.ProfileOwners, proof.ProfileOwners) {
		return fmt.Errorf("pending receipt physical role proof is incomplete")
	}
	manager := kernel.Directory
	manager.Namespace = kernel.Namespace
	current, err := manager.Load(r.OperationID)
	if err == nil {
		if !sameDirectoryProof(r.DirectoryProof, current) {
			return fmt.Errorf("pending receipt and current physical journal disagree")
		}
		proof = current
	} else if os.IsNotExist(err) {
		// The state commit can outlive journal removal. Verify final roles.
		proof.Phase = dirswap.PhaseCommitted
	} else {
		return err
	}
	return manager.VerifyRecorded(proof)
}
