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
			session.markRemainingNotAttempted(plannedIndex, "canceled", "processing stopped because the operation was canceled before remaining clients could be activated safely")
			return session.result, fmt.Errorf("%d of %d client activations failed: %w", session.externalFailed+countNotAttempted(session.result.Targets), logicalTotal, err)
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
	outcome, activationErr := session.activateGroupDelivery(target)
	previousNativeObjects := []domain.NativeObjectOwnership(nil)
	if target.managed != nil {
		previousNativeObjects = target.managed.NativeObjects
	}
	lifecycleChanged, persistErr := session.service.updateActivationResult(session.installationID, target.clientBindingID, outcome, activationErr, previousNativeObjects)
	if lifecycleChanged {
		session.result.Mutated = true
	}
	session.assignActivationOutcome(target, outcome, activationErr, persistErr, lifecycleChanged)
	logicalTotal := len(session.result.Targets)
	if persistErr != nil {
		session.externalFailed += len(target.resultIndexes)
		session.classifyActivationFailure()
		session.markRemainingNotAttempted(plannedIndex+1, "persist", "processing stopped because installation state could not be saved safely")
		return fmt.Errorf("%d of %d client activations failed: %w", session.externalFailed+countNotAttempted(session.result.Targets), logicalTotal, persistErr)
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
		session.markRemainingNotAttempted(plannedIndex+1, "canceled", "processing stopped because the operation was canceled before remaining clients could be activated safely")
		cause := activationErr
		if ctxErr := session.ctx.Err(); ctxErr != nil && !errors.Is(activationErr, context.Canceled) && !errors.Is(activationErr, context.DeadlineExceeded) {
			cause = ctxErr
		}
		return fmt.Errorf("%d of %d client activations failed: %w", session.externalFailed+countNotAttempted(session.result.Targets), logicalTotal, cause)
	}
	return nil
}

func (session *groupSession) activateGroupDelivery(target plannedGroupTarget) (domain.ActivationOutcome, error) {
	delivery := target.delivery
	if target.noChange && target.managed != nil {
		delivery = domain.StagedDelivery{
			ClientID: target.input.Client.ClientID, OwnedBase: target.plan.TargetRoot,
			ActivePath: target.managed.TargetLocator, ArtifactDigest: managedDigest(*target.managed),
			NativeObjects: append([]domain.NativeObjectOwnership(nil), target.managed.NativeObjects...),
		}
	}
	previous := []domain.NativeObjectOwnership(nil)
	if target.managed != nil {
		previous = append([]domain.NativeObjectOwnership(nil), target.managed.NativeObjects...)
	}
	outcome, activationErr := session.service.Activator.Activate(session.ctx, domain.ActivationRequest{
		Client: target.input.Client, Plan: target.plan, Delivery: delivery,
		DeclaredName: target.input.Envelope.Manifest.Name, Replacing: session.replace, Interactive: target.input.Interactive,
		BackendExecutable: target.input.BackendExecutable, PreviousNativeObjects: previous,
		VerifyOnly: target.noChange, ActivationComplete: target.input.ActivationComplete,
	})
	if session.input.Repair && target.managed != nil {
		outcome = preserveManagedAuthentication(outcome, target.managed.Authentication)
	}
	activationErr = session.normalizeActivationError(target, &outcome, activationErr)
	session.preserveNoChangeActivation(target, &outcome, activationErr)
	return outcome, activationErr
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
		*outcome = domain.ActivationOutcome{
			Activation: domain.ActivationFailed, Authentication: target.plan.Authentication,
			Policy: domain.PolicyAllowed, Verification: domain.VerificationFailed,
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
			session.result.Targets[resultIndex].Failure = groupTargetFailureFromActivation(activationErr, outcome)
		default:
			session.result.Targets[resultIndex].GroupPhase = GroupTargetExternalCompleted
		}
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
