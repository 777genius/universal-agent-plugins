package usecase

import (
	"context"
	"errors"
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (session *groupSession) finishGroupActivation(target plannedGroupTarget, outcome domain.ActivationOutcome, delivery domain.StagedDelivery, nativeComplete bool, activationErr error) (domain.ActivationOutcome, domain.StagedDelivery, bool, error) {

	if session.input.Repair && target.managed != nil {
		outcome = preserveManagedAuthentication(outcome, target.managed.Authentication)
	}
	activationErr = session.normalizeActivationError(target, &outcome, activationErr)
	session.preserveNoChangeActivation(target, &outcome, activationErr)
	return outcome, delivery, nativeComplete, activationErr
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
	mutated := !target.noChange || lifecycleChanged || outcome.NativeEffect == domain.NativeEffectCommitted
	for _, resultIndex := range target.resultIndexes {
		session.result.Targets[resultIndex].Activation = outcome
		session.result.Targets[resultIndex].Mutated = mutated
		if mutated {
			session.result.Targets[resultIndex].NoChange = false
		}
		session.result.Mutated = session.result.Mutated || session.result.Targets[resultIndex].Mutated
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
	case domain.IsUnclosedClientProbe(cause) && !errors.Is(cause, context.DeadlineExceeded):
		return "activation", "processing stopped because client process cleanup could not be confirmed before remaining clients could be activated safely"
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
