package statev2

import (
	"context"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/profileauthority"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func validateStandaloneReceipts(receipts []domain.MutationReceipt, operationIDs map[string]struct{}) error {
	for index, receipt := range receipts {
		prefix := fmt.Sprintf("transaction_receipts[%d]", index)
		if err := pathpolicy.ValidateLeafID(receipt.OperationID); err != nil {
			return fmt.Errorf("%s has invalid operation id: %w", prefix, err)
		}
		sharedData := receipt.ClientBindingID == "" && receipt.DataReceiptID != "" && receipt.MutationType == "directory_remove" && len(receipt.ProfileOwners) > 0 && len(receipt.DirectoryProof) > 0
		if (strings.TrimSpace(receipt.ClientBindingID) == "" && !sharedData) || receipt.Sequence < 1 || strings.TrimSpace(receipt.MutationType) == "" {
			return fmt.Errorf("%s is incomplete", prefix)
		}
		if receipt.DataReceiptID != "" {
			if err := pathpolicy.ValidateLeafID(receipt.DataReceiptID); err != nil {
				return fmt.Errorf("%s has invalid data receipt id: %w", prefix, err)
			}
		}
		if !validReceiptPhase(receipt.Phase) {
			return fmt.Errorf("%s has invalid phase %q", prefix, receipt.Phase)
		}
		if receipt.OperationGroupID != "" {
			if err := pathpolicy.ValidateLeafID(receipt.OperationGroupID); err != nil {
				return fmt.Errorf("%s has invalid operation group id: %w", prefix, err)
			}
		}
		if err := receipt.ValidatePublicationParent(true); err != nil {
			return err
		}
		if _, duplicate := operationIDs[receipt.OperationID]; duplicate {
			return fmt.Errorf("duplicate receipt operation_id %q", receipt.OperationID)
		}
		operationIDs[receipt.OperationID] = struct{}{}
	}
	return nil
}

// State loading and saving must retain the recorded publication ancestry.
func revalidatePublicationParents(ctx context.Context, state domain.StateFileV2) error {
	for _, receipt := range state.TransactionReceipts {
		if receipt.PublicationParentAuthority != nil {
			if err := profileauthority.Revalidate(ctx, *receipt.PublicationParentAuthority); err != nil {
				return err
			}
		}
	}
	return nil
}
