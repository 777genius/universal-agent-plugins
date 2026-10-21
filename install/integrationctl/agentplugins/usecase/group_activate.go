package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (session *groupSession) activateGroupTargets() (GroupResult, error) {
	session.externalCompleted = 0
	session.externalFailed = 0
	session.firstActivationErr = nil
	logicalTotal := len(session.result.Targets)
	for plannedIndex, target := range session.planned {
		if err := session.ctx.Err(); err != nil {
			session.classifyActivationFailure()
			stage, message := groupActivationStopReason(session.ctx, err)
			session.markRemainingNotAttempted(plannedIndex, stage, message)
			return session.result, session.activationFailureError(err)
		}
		if err := session.activateOneGroupTarget(plannedIndex, target); err != nil {
			return session.result, err
		}
	}
	if session.externalFailed > 0 {
		session.classifyActivationFailure()
		if session.firstActivationErr != nil {
			return session.result, fmt.Errorf("%d of %d client activations failed: %w", session.externalFailed, logicalTotal, session.firstActivationErr)
		}
		return session.result, fmt.Errorf("%d of %d client activations failed", session.externalFailed, logicalTotal)
	}
	session.result.Phase = GroupPhaseCompleted
	return session.result, nil
}

func (session *groupSession) activateOneGroupTarget(plannedIndex int, target plannedGroupTarget) error {
	previousObservation := target.plan.LocalEntryObservation.Clone()
	outcome, nativeComplete, activationErr := session.activateGroupDelivery(target)
	previousNativeObjects := []domain.NativeObjectOwnership(nil)
	if target.managed != nil {
		previousNativeObjects = target.managed.NativeObjects
	}
	lifecycleChanged, persistErr := session.service.updateActivationResultWithObservation(session.installationID, target.clientBindingID, outcome, activationErr, previousNativeObjects, previousObservation)
	if lifecycleChanged {
		session.result.Mutated = true
	}
	if !nativeComplete && activationErr == nil {
		// An incomplete projection that re-activated successfully changed the
		// target. A failure before or during activation claims no mutation; the
		// recorded lifecycle change, if any, is the evidence.
		target.noChange = false
	}
	session.assignActivationOutcome(target, outcome, activationErr, persistErr, lifecycleChanged)
	if err := session.reportGroupProgress(target, GroupProgressActivated); err != nil {
		return err
	}
	if persistErr != nil {
		session.externalFailed += len(target.resultIndexes)
		session.classifyActivationFailure()
		session.markRemainingNotAttempted(plannedIndex+1, "persist", "processing stopped because installation state could not be saved safely")
		return session.activationFailureError(persistErr)
	}
	if activationErr == nil {
		session.externalCompleted += len(target.resultIndexes)
		return nil
	}
	session.externalFailed += len(target.resultIndexes)
	if session.firstActivationErr == nil {
		session.firstActivationErr = activationErr
	}
	if errors.Is(activationErr, context.Canceled) || errors.Is(activationErr, context.DeadlineExceeded) || session.ctx.Err() != nil {
		session.classifyActivationFailure()
		stage, message := groupActivationStopReason(session.ctx, activationErr)
		session.markRemainingNotAttempted(plannedIndex+1, stage, message)
		cause := activationErr
		if ctxErr := session.ctx.Err(); ctxErr != nil && !errors.Is(activationErr, ctxErr) {
			cause = errors.Join(activationErr, ctxErr)
		}
		return session.activationFailureError(cause)
	}
	return nil
}

func (session *groupSession) activateGroupDelivery(target plannedGroupTarget) (domain.ActivationOutcome, bool, error) {
	delivery := target.delivery
	nativeComplete := true
	if target.noChange && target.managed != nil {
		var err error
		delivery, nativeComplete, _, err = session.service.activeNativeDelivery(session.ctx, target.input, target.plan, session.state.Installations[session.installationIndex], *target.managed)
		if err != nil {
			// A projection failure happens before the adapter can run. Keep the
			// recorded lifecycle and acknowledged objects instead of persisting
			// an empty outcome, so the failure reports the projection error
			// rather than a state refusal.
			outcome := lifecycleOutcome(*target.managed)
			outcome.NativeEffect = domain.NativeEffectUnchanged
			return outcome, false, err
		}
	}
	previous := []domain.NativeObjectOwnership(nil)
	if target.managed != nil {
		previous = append([]domain.NativeObjectOwnership(nil), target.managed.NativeObjects...)
	}
	outcome, activationErr := session.service.activateWithNativeAttempt(session.ctx, session.installationID, target.clientBindingID, domain.ActivationRequest{
		PackageUnchanged: target.noChange,
		Client:           target.input.Client, Plan: target.plan, Delivery: delivery,
		DeclaredName: target.input.Envelope.Manifest.Name, Replacing: session.replace, Interactive: target.input.Interactive,
		BackendExecutable: target.input.BackendExecutable, PreviousNativeObjects: previous,
		VerifyOnly: target.noChange && nativeComplete, ActivationComplete: target.input.ActivationComplete,
	})
	if session.input.Repair && target.managed != nil {
		outcome = preserveManagedAuthentication(outcome, target.managed.Authentication)
	}
	activationErr = session.normalizeActivationError(target, &outcome, activationErr)
	session.preserveNoChangeActivation(target, &outcome, activationErr)
	return outcome, nativeComplete, activationErr
}

func (session *groupSession) normalizeActivationError(target plannedGroupTarget, outcome *domain.ActivationOutcome, activationErr error) error {
	if activationErr == nil && outcome.Activation == "" {
		if err := session.ctx.Err(); err != nil {
			activationErr = err
		} else {
			activationErr = fmt.Errorf("activator returned an empty activation outcome")
		}
	}
	if activationErr == nil && (outcome.Activation == domain.ActivationFailed || outcome.Verification == domain.VerificationFailed || outcome.Authentication == domain.AuthenticationFailed) {
		activationErr = fmt.Errorf("activator reported a failed activation outcome without an error")
	}
	if activationErr != nil && outcome.Activation == "" {
		effect, objects := outcome.NativeEffect, outcome.NativeObjects
		*outcome = domain.ActivationOutcome{
			Activation: domain.ActivationFailed, Authentication: target.plan.Authentication,
			Policy: domain.PolicyAllowed, Verification: domain.VerificationFailed,
			NativeEffect: effect, NativeObjects: objects,
		}
	}
	return activationErr
}

func (session *groupSession) preserveNoChangeActivation(target plannedGroupTarget, outcome *domain.ActivationOutcome, activationErr error) {
	if !target.noChange || target.managed == nil {
		return
	}
	if activationErr == nil && !session.service.clientVerifierAvailable(target.input, target.plan) && target.managed.Activation == domain.ActivationActive && target.managed.Verification == domain.VerificationInstalled {
		outcome.Activation = target.managed.Activation
		outcome.Verification = target.managed.Verification
	}
	if (target.managed.Authentication == domain.AuthenticationPending || target.managed.Authentication == domain.AuthenticationNotChecked) && target.input.AuthComplete {
		outcome.Authentication = domain.AuthenticationComplete
		outcome.AuthenticationAttested = true
		return
	}
	if target.managed.Authentication != "" {
		outcome.Authentication = target.managed.Authentication
	}
}

func (session *groupSession) assignActivationOutcome(target plannedGroupTarget, outcome domain.ActivationOutcome, activationErr, persistErr error, lifecycleChanged bool) {
	for _, resultIndex := range target.resultIndexes {
		session.result.Targets[resultIndex].Activation = outcome
		session.result.Targets[resultIndex].Mutated = !target.noChange || lifecycleChanged
		if lifecycleChanged {
			session.result.Targets[resultIndex].NoChange = false
		}
		switch {
		case persistErr != nil:
			session.result.Targets[resultIndex].GroupPhase = GroupTargetExternalFailed
			session.result.Targets[resultIndex].Failure = &GroupTargetFailure{Stage: "persist", Message: persistErr.Error()}
		case activationErr != nil:
			session.result.Targets[resultIndex].GroupPhase = GroupTargetExternalFailed
			session.result.Targets[resultIndex].Failure = groupTargetFailureFromActivation(session.ctx, activationErr, outcome)
		default:
			session.result.Targets[resultIndex].GroupPhase = GroupTargetExternalCompleted
		}
	}
}

func (session *groupSession) activationFailureError(cause error) error {
	if skipped := countNotAttempted(session.result.Targets); skipped > 0 {
		return fmt.Errorf("%d of %d client activations failed; %d not attempted: %w", session.externalFailed, len(session.result.Targets), skipped, cause)
	}
	return fmt.Errorf("%d of %d client activations failed: %w", session.externalFailed, len(session.result.Targets), cause)
}

func groupActivationStopReason(ctx context.Context, cause error) (string, string) {
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return "deadline", "processing stopped because the operation deadline was exceeded before remaining clients could be activated safely"
	case errors.Is(ctx.Err(), context.Canceled), errors.Is(cause, context.Canceled):
		return "canceled", "processing stopped because the operation was canceled before remaining clients could be activated safely"
	default:
		return "timeout", "processing stopped because a client activation timed out before remaining clients could be activated safely"
	}
}

func (session *groupSession) markRemainingNotAttempted(fromIndex int, stage, message string) {
	for index := fromIndex; index < len(session.planned); index++ {
		target := session.planned[index]
		for _, resultIndex := range target.resultIndexes {
			session.result.Targets[resultIndex].GroupPhase = GroupTargetExternalNotAttempted
			session.result.Targets[resultIndex].Failure = &GroupTargetFailure{Stage: stage, Message: message}
		}
	}
}

func (session *groupSession) classifyActivationFailure() {
	if session.externalCompleted > 0 {
		session.result.Phase = GroupPhaseExternalPartialFailure
		return
	}
	session.result.Phase = GroupPhaseManagedActivationFailed
}
