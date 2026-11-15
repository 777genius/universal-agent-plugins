package usecase

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Explicit attestations keep their existing resume path. Otherwise an opted-in
// verifier must observe the current client instead of retaining cached Active.
func (session *applySession) freshExistingVerification() bool {
	return !session.input.AuthComplete && !session.input.ActivationComplete &&
		session.service.preparedSetupVerifierAvailable(session.input, session.result.Plan)
}

// Reuse intact-package repair inside the already-held apply mutation lock.
// The fresh read-only result alone is not authority to act: the adapter must
// admit this exact setup and repair rechecks revision, path, digest and ownership.
func (session *applySession) completeExistingPreparedSetup(current domain.ClientBinding, verified domain.ActivationOutcome) (bool, AddResult, error) {
	if err := session.ctx.Err(); err != nil {
		return true, session.result, err
	}
	if verified.Activation != domain.ActivationPrepared {
		return false, AddResult{}, nil
	}
	if current.NativeActivationAttempt != "" || current.PendingNativeIntent != nil {
		return true, session.result, fmt.Errorf("native activation attempt %s is unresolved; inspect the owned client state before retry", unresolvedNativeAttemptID(current))
	}
	repair := &repairSession{
		service: session.service, ctx: session.ctx, input: session.input, result: session.result,
		state: session.state, index: session.installationIndex, installation: session.state.Installations[session.installationIndex],
		plan: session.result.Plan, clientKey: session.clientBindingID, client: current,
	}
	delivery, _, _, err := session.service.activeNativeDelivery(session.ctx, session.input, session.result.Plan, repair.installation, current)
	if err != nil {
		return true, session.result, err
	}
	if !repair.canCompletePreparedSetup(delivery, verified) {
		return false, AddResult{}, nil
	}
	repair.result.Activation = verified
	repair.result.NoChange = false
	if err := repair.validateRepairRevision(); err != nil {
		return true, repair.result, err
	}
	if err := repair.verifyRepairPreconditions(); err != nil {
		return true, repair.result, err
	}
	if repair.verifyErr != nil {
		return true, repair.result, fmt.Errorf("managed package changed before completing native setup: %w", repair.verifyErr)
	}
	if session.input.DryRun || !session.input.Confirmed {
		repair.result.RequiresConfirmation = true
		return true, repair.result, nil
	}
	result, err := repair.reapplyIntactNative(delivery)
	return true, result, err
}
