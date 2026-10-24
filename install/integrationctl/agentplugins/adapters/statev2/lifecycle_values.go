package statev2

import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"

func validReceiptPhase(value string) bool {
	return value == receiptPhaseStateCommitted || value == receiptPhaseCommitted
}

func validMaterializationState(value domain.MaterializationState) bool {
	switch value {
	case domain.MaterializationAbsent,
		domain.MaterializationStaged,
		domain.MaterializationMaterialized,
		domain.MaterializationDegraded:
		return true
	default:
		return false
	}
}

func validActivationState(value domain.ActivationState) bool {
	switch value {
	case domain.ActivationNotRequired,
		domain.ActivationPrepared,
		domain.ActivationManual,
		domain.ActivationActive,
		domain.ActivationFailed:
		return true
	default:
		return false
	}
}

func validAuthenticationState(value domain.AuthenticationState) bool {
	switch value {
	case domain.AuthenticationNotRequired,
		domain.AuthenticationNotChecked,
		domain.AuthenticationPending,
		domain.AuthenticationComplete,
		domain.AuthenticationFailed:
		return true
	default:
		return false
	}
}

func validPolicyState(value domain.PolicyState) bool {
	switch value {
	case domain.PolicyAllowed,
		domain.PolicyBlocked,
		domain.PolicyApprovalRequired:
		return true
	default:
		return false
	}
}

func validVerificationState(value domain.VerificationState) bool {
	switch value {
	case domain.VerificationNotRun,
		domain.VerificationPackageValid,
		domain.VerificationInstalled,
		domain.VerificationRuntime,
		domain.VerificationFailed:
		return true
	default:
		return false
	}
}
