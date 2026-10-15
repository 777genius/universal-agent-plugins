package transaction

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var ErrEnvironmentAuthority = errors.New("environment owner authority is invalid or missing")

// AuthorizeEnvironmentMutation is the common ownership guard used by SDK and
// legacy CLI lifecycle paths before any managed effect. It never adopts an
// unmanaged binding and requires authority whenever the selected binding is environment-owned.
func AuthorizeEnvironmentMutation(state domain.StateFileV2, authority *domain.EnvironmentAuthority, action, installationID, bindingID, sourceKey string, allowCreate bool) error {
	if authority == nil {
		for _, receipt := range state.EnvironmentReceipts {
			if receipt.InstallationID == installationID && receipt.ClientBindingID == bindingID {
				return fmt.Errorf("%w: %s requires typed environment authority", ErrEnvironmentAuthority, action)
			}
		}
		return nil
	}
	if err := validateEnvironmentMutationIdentity(authority, action, installationID, bindingID, allowCreate); err != nil {
		return err
	}
	if authority.SourceKey != "" {
		sourceKey = authority.SourceKey
	}
	var receipt *domain.EnvironmentReceipt
	for index := range state.EnvironmentReceipts {
		item := &state.EnvironmentReceipts[index]
		if item.EnvironmentID == authority.EnvironmentID && item.Alias == authority.Alias && item.ClientID == authority.ClientID {
			receipt = item
			break
		}
	}
	if receipt == nil {
		return authorizeEnvironmentCreation(state, authority, installationID, bindingID, allowCreate)
	}
	if authority.ReceiptID != "" && authority.ReceiptID != receipt.ReceiptID {
		return fmt.Errorf("%w: expected receipt changed", ErrEnvironmentAuthority)
	}
	if receipt.InstallationID != installationID || receipt.ClientBindingID != bindingID {
		return fmt.Errorf("%w: receipt points at a different installation or binding", ErrEnvironmentAuthority)
	}
	if sourceKey != "" && receipt.SourceKey != sourceKey {
		return fmt.Errorf("%w: receipt source key changed", ErrEnvironmentAuthority)
	}
	return nil
}

func UpsertEnvironmentReceipt(state *domain.StateFileV2, authority *domain.EnvironmentAuthority, installationID, bindingID, sourceKey, packageDigest string, profile *domain.ProfileAuthority, phase string) error {
	if state == nil || authority == nil {
		return ErrEnvironmentAuthority
	}
	item := domain.EnvironmentReceipt{
		EnvironmentID: authority.EnvironmentID, Alias: authority.Alias, ClientID: authority.ClientID,
		InstallationID: installationID, ClientBindingID: bindingID,
		PackageDigest: packageDigest, ProfileAuthority: domain.CloneProfileAuthority(profile),
		SourceKey: firstNonEmpty(authority.SourceKey, sourceKey), DesiredAction: authority.ExpectedAction, Phase: phase,
	}
	for index := range state.EnvironmentReceipts {
		existing := &state.EnvironmentReceipts[index]
		if existing.EnvironmentID == item.EnvironmentID && existing.Alias == item.Alias && existing.ClientID == item.ClientID {
			item.ReceiptID = existing.ReceiptID
			if item.ReceiptID == "" {
				item.ReceiptID = ComputeEnvironmentReceiptID(item.EnvironmentID, item.Alias, item.ClientID)
			}
			state.EnvironmentReceipts[index] = item
			return nil
		}
	}
	item.ReceiptID = ComputeEnvironmentReceiptID(item.EnvironmentID, item.Alias, item.ClientID)
	state.EnvironmentReceipts = append(state.EnvironmentReceipts, item)
	return nil
}

func DropEnvironmentReceipt(state *domain.StateFileV2, authority *domain.EnvironmentAuthority) {
	if state == nil || authority == nil {
		return
	}
	kept := state.EnvironmentReceipts[:0]
	for _, receipt := range state.EnvironmentReceipts {
		if receipt.EnvironmentID == authority.EnvironmentID && receipt.Alias == authority.Alias && receipt.ClientID == authority.ClientID {
			continue
		}
		kept = append(kept, receipt)
	}
	state.EnvironmentReceipts = kept
}

// ValidateEnvironmentRecoveryScope refuses a mixed owned/foreign or
// indeterminate recovery before Kernel.Recover can change any journal, receipt
// or native intent. One typed owner must cover the complete pending scope.
func ValidateEnvironmentRecoveryScope(state domain.StateFileV2, authority *domain.EnvironmentAuthority) error {
	wanted := ""
	if authority != nil {
		wanted = authority.EnvironmentID
	}
	check := func(owner *domain.EnvironmentAuthority) error {
		got := ""
		if owner != nil {
			got = owner.EnvironmentID
		}
		if got != wanted {
			return fmt.Errorf("%w: foreign or indeterminate pending recovery blocks complete scope", ErrEnvironmentAuthority)
		}
		return nil
	}
	for _, receipt := range state.TransactionReceipts {
		if receipt.Phase != "" && receipt.Phase != "committed" {
			if err := check(receipt.EnvironmentAuthority); err != nil {
				return err
			}
		}
	}
	for _, installation := range state.Installations {
		for _, binding := range installation.Clients {
			if err := validateBindingRecoveryScope(state, installation.InstallationID, binding, check); err != nil {
				return err
			}
		}
	}
	return nil
}

func ComputeEnvironmentReceiptID(environmentID, alias, clientID string) string {
	body, _ := json.Marshal([]string{environmentID, alias, clientID})
	sum := sha256.Sum256(body)
	return "envrec_" + hex.EncodeToString(sum[:12])
}

func environmentID(authority *domain.EnvironmentAuthority) string {
	if authority == nil {
		return ""
	}
	return authority.EnvironmentID
}

func authorizeEnvironmentCreation(state domain.StateFileV2, authority *domain.EnvironmentAuthority, installationID, bindingID string, allowCreate bool) error {
	if !allowCreate {
		return fmt.Errorf("%w: no environment receipt owns this binding", ErrEnvironmentAuthority)
	}
	if installationID == "" && bindingID == "" {
		return nil
	}
	if bindingID == "" && authority.ClientBindingID == "" && authority.ReceiptID == "" {
		for _, installation := range state.Installations {
			if installation.InstallationID == installationID && len(installation.Clients) == 0 {
				return nil
			}
		}
	}
	for _, existing := range state.EnvironmentReceipts {
		if existing.EnvironmentID == authority.EnvironmentID && existing.Alias == authority.Alias &&
			existing.InstallationID == installationID {
			return nil
		}
	}
	return fmt.Errorf("%w: unmanaged installation cannot be adopted", ErrEnvironmentAuthority)

}

func validateBindingRecoveryScope(state domain.StateFileV2, installationID string, binding domain.ClientBinding, check func(*domain.EnvironmentAuthority) error) error {
	for _, receipt := range binding.Receipts {
		if receipt.Phase != "" && receipt.Phase != "committed" {
			if err := check(receipt.EnvironmentAuthority); err != nil {
				return err
			}
		}
	}
	if binding.PendingNativeIntent == nil {
		return nil
	}
	var owner *domain.EnvironmentAuthority
	for _, receipt := range state.EnvironmentReceipts {
		if receipt.InstallationID == installationID && receipt.ClientBindingID == binding.ClientBindingID {
			owner = &domain.EnvironmentAuthority{EnvironmentID: receipt.EnvironmentID}
		}
	}
	return check(owner)
}

func validateEnvironmentMutationIdentity(authority *domain.EnvironmentAuthority, action, installationID, bindingID string, allowCreate bool) error {
	if authority.EnvironmentID == "" || authority.Alias == "" || authority.ClientID == "" || authority.ExpectedAction != action {
		return fmt.Errorf("%w: incomplete or wrong expected action", ErrEnvironmentAuthority)
	}
	if !allowCreate && (authority.InstallationID == "" || authority.ClientBindingID == "") {
		return fmt.Errorf("%w: existing binding identity is required", ErrEnvironmentAuthority)
	}
	if authority.InstallationID != "" && authority.InstallationID != installationID {
		return fmt.Errorf("%w: installation identity changed", ErrEnvironmentAuthority)
	}
	if authority.ClientBindingID != "" && authority.ClientBindingID != bindingID {
		return fmt.Errorf("%w: binding identity changed", ErrEnvironmentAuthority)
	}
	return nil
}
