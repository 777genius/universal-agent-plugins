package environment

import (
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// LifecycleFacts reports convergence for automatic sync. Authentication alone
// requires explicit user attestation, rather than repeated package mutations.
func LifecycleFacts(binding installer.InspectedBinding) (retry bool, facts []string) {
	// Manual readiness is not observed native activation. Retain that fact and
	// offer attestation without automatically repeating an already prepared tree.
	if binding.Materialization == string(domain.MaterializationMaterialized) &&
		(binding.Verification == string(domain.VerificationPackageValid) || binding.Verification == string(domain.VerificationInstalled) || binding.Verification == string(domain.VerificationRuntime)) {
		switch binding.Activation {
		case string(domain.ActivationManual):
			return false, append([]string{"activation_manual"}, authenticationFacts(binding.Authentication)...)
		case string(domain.ActivationPrepared):
			return false, append([]string{"activation_prepared"}, authenticationFacts(binding.Authentication)...)
		}
	}
	if binding.LifecycleConverged {
		return false, nil
	}
	if binding.Materialization == string(domain.MaterializationMaterialized) && binding.Activation == string(domain.ActivationActive) && binding.Verification == string(domain.VerificationInstalled) {
		switch binding.Authentication {
		case string(domain.AuthenticationNotChecked):
			return false, []string{"auth_unverified"}
		case string(domain.AuthenticationPending):
			return false, []string{"auth_pending"}
		case string(domain.AuthenticationFailed):
			return false, []string{"auth_failed"}
		}
	}
	return true, []string{"partial"}
}

// Observe derives desired-state observations exclusively from live bindings and
// receipts. Unmanaged siblings remain visible so planning cannot adopt their
// shared installation through a different target.
func Observe(view installer.Inspection) ([]BindingObservation, error) {
	return ObserveForEnvironment(view, EnvironmentUserDefault)
}

// ObserveForEnvironment uses the selected namespace's explicit receipt owner.
// Other ownership stays visible as foreign and cannot be adopted by alias.
func ObserveForEnvironment(view installer.Inspection, environmentID string) ([]BindingObservation, error) {
	if environmentID == "" {
		environmentID = EnvironmentUserDefault
	}
	if view.Recovery.Required {
		return nil, installer.ErrRecoveryRequired
	}
	var out []BindingObservation
	live := map[string]bool{}
	for _, installation := range view.Installations {
		for _, binding := range installation.Bindings {
			current := BindingObservation{SourceKey: installation.SourceKey, InstallationID: installation.InstallationID, ClientID: binding.ClientID, BindingID: binding.BindingID, PackageDigest: binding.TreeDigest, Revision: binding.Revision}
			for _, receipt := range view.EnvironmentReceipts {
				if receipt.InstallationID != installation.InstallationID || receipt.ClientBindingID != binding.BindingID {
					continue
				}
				live[receipt.ReceiptID] = true
				if receipt.EnvironmentID != environmentID {
					continue
				}
				if receipt.SourceKey != installation.SourceKey || receipt.PackageDigest != binding.TreeDigest {
					return nil, errors.New("receipt/live binding identity mismatch")
				}
				cloned := receipt
				current.Alias, current.Managed, current.Receipt = receipt.Alias, true, &cloned
			}
			current.Incomplete, _ = LifecycleFacts(binding)
			current.Drift = binding.ProjectionStatus != ""
			current.Pending = binding.Materialization != string(domain.MaterializationMaterialized)
			out = append(out, current)
		}
	}
	for _, receipt := range view.EnvironmentReceipts {
		if receipt.EnvironmentID == environmentID && !live[receipt.ReceiptID] {
			return nil, errors.New("partial: environment receipt has no live binding")
		}
	}
	return out, nil
}

func authenticationFacts(authentication string) []string {
	switch authentication {
	case string(domain.AuthenticationNotChecked):
		return []string{"auth_unverified"}
	case string(domain.AuthenticationPending):
		return []string{"auth_pending"}
	case string(domain.AuthenticationFailed):
		return []string{"auth_failed"}
	}
	return nil
}
