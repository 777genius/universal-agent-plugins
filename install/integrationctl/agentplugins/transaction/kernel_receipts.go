package transaction

// Commit and ownership proof bookkeeping.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/profileauthority"
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

// Assemble durable facts from the actual directory result, preserving its origin.
func recordedDirectoryMutation(mutation DirectoryMutation, raw dirswap.Receipt, owners []domain.PhysicalProfileOwner) (domain.MutationReceipt, error) {
	parent, err := recordedPublicationParent(raw.PublicationParentAuthority)
	if err != nil {
		return domain.MutationReceipt{}, err
	}
	return domain.MutationReceipt{PublicationParentAuthority: parent, DirectoryProof: directoryProof(raw), ProfileOwners: copyOwners(owners), DataReceiptID: raw.DataReceiptID,
		OperationID:          mutation.OperationID,
		OperationGroupID:     mutation.OperationID,
		Sequence:             mutation.Sequence,
		MutationType:         "directory_swap",
		ClientBindingID:      mutation.ClientBindingID,
		ActivePath:           raw.ActivePath,
		StagingPath:          raw.StagingPath,
		BackupPath:           raw.BackupPath,
		BeforeDigest:         mutation.BeforeDigest,
		AfterDigest:          mutation.AfterDigest,
		EnvironmentAuthority: mutation.EnvironmentAuthority,
		Phase:                ReceiptPhaseStateCommitted,
	}, nil
}

func recordDirectoryMutation(state *domain.StateFileV2, before domain.StateFileV2, index int, client domain.ClientBinding, mutation DirectoryMutation, receipt domain.MutationReceipt) {
	if mutation.Standalone {
		receipt.PublicationParentAuthority = domain.CloneProfileAuthority(receipt.PublicationParentAuthority)
		state.TransactionReceipts = append(state.TransactionReceipts, receipt)
		return
	}
	client.Receipts = append(client.Receipts, receipt)
	client.NativeObjects = committedDirectoryObjects(before, mutation.InstallationID, mutation.ClientBindingID, mutation.NativeObjects)
	client.Materialization = domain.MaterializationMaterialized
	client.Activation = mutation.Activation
	client.Authentication = mutation.Authentication
	client.Policy = mutation.Policy
	client.Verification = mutation.Verification
	installation := state.Installations[index]
	installation.OperationGroupID = mutation.OperationID
	installation.Clients[mutation.ClientBindingID] = client
	state.Installations[index] = installation
}

func validateStandalonePublication(mutation DirectoryMutation, before []byte) error {
	if !mutation.Standalone {
		if mutation.PublicationParentAuthority != nil {
			return fmt.Errorf("publication parent is only valid for standalone publication")
		}
		return nil
	}
	if !mutation.RequireAbsent || mutation.Verify == nil || mutation.BeforeDigest != "" ||
		len(mutation.NativeObjects) != 0 || mutation.Activation != "" || mutation.Authentication != "" ||
		mutation.Policy != "" || mutation.Verification != "" {
		return fmt.Errorf("standalone publication requires verified fresh contents without binding changes")
	}
	if mutation.PublicationParentAuthority == nil || mutation.PublicationParentAuthority.IsZero() || mutation.PublicationParentAuthority.Facts().CanonicalRoot != mutation.OwnedBase {
		return fmt.Errorf("standalone publication original parent is required")
	}
	desired, err := marshalComparableState(mutation.DesiredState)
	if err != nil {
		return err
	}
	if !bytes.Equal(before, desired) {
		return fmt.Errorf("standalone publication cannot change installation state")
	}
	return nil
}

// Injected state stores receive the same early ancestry check as concrete stores.
func (Kernel) validatePublicationParents(ctx context.Context, state domain.StateFileV2) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, installation := range state.Installations {
		for _, binding := range installation.Clients {
			for _, receipt := range binding.Receipts {
				if err := receipt.ValidatePublicationParent(false); err != nil {
					return err
				}
			}
		}
	}
	for _, receipt := range state.TransactionReceipts {
		if err := receipt.ValidatePublicationParent(true); err != nil {
			return err
		}
		if receipt.PublicationParentAuthority == nil {
			continue
		}
		if err := profileauthority.Revalidate(ctx, *receipt.PublicationParentAuthority); err != nil {
			return err
		}
	}
	return nil
}

func neutralPublicationParent(parent *domain.ProfileAuthority) (*directoryidentity.Authority, error) {
	if parent == nil {
		return nil, nil
	}
	authority, err := profileauthority.ToNeutral(*parent)
	return &authority, err
}

func recordedPublicationParent(parent *directoryidentity.Authority) (*domain.ProfileAuthority, error) {
	if parent == nil {
		return nil, nil
	}
	authority, err := profileauthority.FromNeutral(*parent)
	return &authority, err
}

func samePublicationParent(recorded *domain.ProfileAuthority, journal *directoryidentity.Authority) bool {
	if recorded == nil || journal == nil {
		return recorded == nil && journal == nil
	}
	converted, err := profileauthority.FromNeutral(*journal)
	return err == nil && recorded.Equal(converted)
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

func hasPublicationReceipt(state domain.StateFileV2, operationID string) bool {
	for _, receipt := range state.TransactionReceipts {
		if receipt.OperationID == operationID && receipt.PublicationParentAuthority != nil {
			return true
		}
	}
	return false
}
