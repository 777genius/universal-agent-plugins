package usecase

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// Managed-package repair may restore known absent or drifted bytes. It must
// first prove that the separate project component files remain exclusively
// owned. A package-only fallback cannot supply this prepared evidence.
func (session *repairSession) verifyProjectRepairOwnership() error {
	if session.input.Scope != domain.ScopeProject || session.plan.SelectedDelivery.EffectiveTraitsForScope(session.input.Client.ClientID, session.input.Scope).LifecycleKind != domain.LifecycleNativeConfig {
		return nil
	}
	if _, ok := session.service.NativeObserver.(PreparedIdentityObserver); !ok {
		return fmt.Errorf("project component repair requires a prepared ownership observer")
	}
	observation, err := session.service.preparedIdentityObservation(session.ctx, session.input.Client, session.plan, &session.client)
	if err != nil {
		return fmt.Errorf("observe project files before repair: %w", err)
	}
	// Prepared providers explicitly report no native observation. A wrapper's
	// legacy package-only fallback has no such finding and cannot qualify.
	if observation.NativeDiscoveryAttempted || observation.NativeDiscoveryReconciled || observation.NativeDiscoveryState != domain.NativeIdentityAbsent {
		return fmt.Errorf("project repair requires filesystem-only ownership evidence")
	}
	if session.verifyErr == nil {
		if observation.State != domain.NativeIdentityManaged || !observation.ReceiptReconciled {
			return fmt.Errorf("project repair ownership receipt is not reconciled")
		}
		return validateNativeIdentityObservation(observation, &session.client)
	}
	verification, known := repairMismatchKind(session.verifyErr)
	if !known {
		return fmt.Errorf("managed package integrity could not be determined before project repair: %w", session.verifyErr)
	}
	if observation.ReceiptReconciled {
		return fmt.Errorf("project repair observation contradicts managed package integrity")
	}
	switch verification.Kind {
	case ports.VerificationAbsent:
		if observation.State == domain.NativeIdentityAbsent && observation.Digest == "" {
			return nil
		}
	case ports.VerificationDigestMismatch:
		if verification.ActualDigest != "" && observation.State == domain.NativeIdentityIndeterminate && observation.Digest == verification.ActualDigest {
			return nil
		}
	}
	return fmt.Errorf("project component ownership or managed package changed before repair")
}
