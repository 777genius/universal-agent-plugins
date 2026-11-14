package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (session *groupSession) activateGroupTargets() (GroupResult, error) {
	if prepared, ok := session.independentGroupActivator(); ok {
		return session.activateIndependentGroupTargets(prepared)
	}
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
	outcome, delivery, nativeComplete, activationErr := session.activateGroupDelivery(target)
	return session.publishGroupActivation(plannedIndex+1, target, outcome, delivery, nativeComplete, activationErr)
}

func (session *groupSession) publishGroupActivation(remainingFrom int, target plannedGroupTarget, outcome domain.ActivationOutcome, delivery domain.StagedDelivery, nativeComplete bool, activationErr error) (publicationErr error) {
	previousObservation := target.plan.LocalEntryObservation.Clone()
	previousNativeObjects := []domain.NativeObjectOwnership(nil)
	if target.managed != nil {
		previousNativeObjects = target.managed.NativeObjects
	}
	lifecycleChanged, persistErr := session.service.updateActivationResultWithObservation(session.installationID, target.clientBindingID, outcome, activationErr, previousNativeObjects, previousObservation)
	if lifecycleChanged {
		session.result.Mutated = true
	}
	preparedUnchanged := target.noChange && target.managed != nil && activationErr == nil &&
		session.service.verifiedPreparedObservationUnchanged(target.input, target.plan, *target.managed, delivery, outcome, nativeComplete)
	if !nativeComplete && !preparedUnchanged && activationErr == nil {
		// An incomplete projection that re-activated successfully changed the
		// target unless fresh evidence proves the exact existing Prepared delivery.
		// This does not make registration complete. A failure before or during
		// activation claims no mutation; any recorded lifecycle change is the evidence.
		target.noChange = false
	}
	session.assignActivationOutcome(target, outcome, activationErr, persistErr, lifecycleChanged)
	defer func() {
		if err := session.reportGroupProgress(target, GroupProgressActivated); err != nil {
			publicationErr = errors.Join(publicationErr, err)
		}
	}()
	if persistErr != nil {
		session.externalFailed += len(target.resultIndexes)
		session.classifyActivationFailure()
		session.markRemainingNotAttempted(remainingFrom, "persist", "processing stopped because installation state could not be saved safely")
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
	if session.ctx.Err() != nil || domain.IsUnclosedClientProbe(activationErr) || errors.Is(activationErr, context.Canceled) ||
		(errors.Is(activationErr, context.DeadlineExceeded) && !domain.IsIsolatedProbeTimeout(activationErr)) {
		session.classifyActivationFailure()
		stage, message := groupActivationStopReason(session.ctx, activationErr)
		session.markRemainingNotAttempted(remainingFrom, stage, message)
		cause := activationErr
		if ctxErr := session.ctx.Err(); ctxErr != nil && !errors.Is(activationErr, ctxErr) {
			cause = errors.Join(activationErr, ctxErr)
		}
		return session.activationFailureError(cause)
	}
	return nil
}

func (session *groupSession) activateGroupDelivery(target plannedGroupTarget) (domain.ActivationOutcome, domain.StagedDelivery, bool, error) {
	request, outcome, delivery, nativeComplete, err := session.groupActivationRequest(target)
	if err != nil {
		return outcome, delivery, nativeComplete, err
	}
	outcome, activationErr := session.service.activateWithNativeAttempt(session.ctx, session.installationID, target.clientBindingID, request)
	return session.finishGroupActivation(target, outcome, delivery, nativeComplete, activationErr)
}

func (session *groupSession) groupActivationRequest(target plannedGroupTarget) (domain.ActivationRequest, domain.ActivationOutcome, domain.StagedDelivery, bool, error) {
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
			return domain.ActivationRequest{}, outcome, delivery, false, err
		}
	}
	previous := []domain.NativeObjectOwnership(nil)
	if target.managed != nil {
		previous = append([]domain.NativeObjectOwnership(nil), target.managed.NativeObjects...)
	}
	return domain.ActivationRequest{
		PackageUnchanged: target.noChange,
		Client:           target.input.Client, Plan: target.plan, Delivery: delivery,
		DeclaredName: target.input.Envelope.Manifest.Name, Replacing: session.replace, Interactive: target.input.Interactive,
		BackendExecutable: target.input.BackendExecutable, PreviousNativeObjects: previous,
		VerifyOnly: target.noChange && nativeComplete, ActivationComplete: target.input.ActivationComplete,
	}, domain.ActivationOutcome{}, delivery, nativeComplete, nil
}
