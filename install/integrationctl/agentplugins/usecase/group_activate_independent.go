package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// Independent effects are opt-in. Other adapters may mutate the shared kernel
// during activation, so they retain the original serial coordinator.
func (session *groupSession) independentGroupActivator() (ports.IndependentClientActivator, bool) {
	activator, ok := session.service.Activator.(ports.IndependentClientActivator)
	if !ok || len(session.planned) < 2 {
		return nil, false
	}
	seen := map[domain.ClientID]bool{}
	for _, target := range session.planned {
		request := domain.ActivationRequest{Client: target.input.Client, Plan: target.plan}
		if seen[target.input.Client.ClientID] || !target.plan.SelectedDelivery.IsZero() || !activator.SupportsIndependentActivation(request) {
			return nil, false
		}
		seen[target.input.Client.ClientID] = true
	}
	return activator, true
}

type independentActivationResult struct {
	index          int
	outcome        domain.ActivationOutcome
	delivery       domain.StagedDelivery
	nativeComplete bool
	err            error
}

// Only client effects run in goroutines. Request preparation, journal changes,
// host callbacks, publication, and progress callbacks stay on this coordinator.
func (session *groupSession) activateIndependentGroupTargets(activator ports.IndependentClientActivator) (GroupResult, error) {
	session.externalCompleted, session.externalFailed = 0, 0
	session.firstActivationErr = nil
	effectContext, cancel := context.WithCancelCause(session.ctx)
	defer cancel(nil)
	results := make(chan independentActivationResult, len(session.planned))
	dispatched := 0
	var stopErr error
	for index, target := range session.planned {
		if err := effectContext.Err(); err != nil {
			stopErr = context.Cause(effectContext)
			break
		}
		request, outcome, delivery, nativeComplete, err := session.groupActivationRequest(target)
		if err == nil {
			request, outcome, err = session.service.prepareNativeActivation(effectContext, session.installationID, target.clientBindingID, request)
		}
		if err == nil && session.service.ClientPreparation != nil && !request.VerifyOnly {
			err = session.service.ClientPreparation.RevalidateClient(effectContext, request.Client, request.Plan)
		}
		var operation func(context.Context) (domain.ActivationOutcome, error)
		if err == nil {
			operation, err = activator.PrepareIndependentActivation(effectContext, request)
		}
		if err == nil {
			err = effectContext.Err()
		}
		if err == nil && operation == nil {
			err = fmt.Errorf("independent activator returned no operation")
		}
		if err == nil {
			err = session.reportGroupProgress(target, GroupProgressActivating)
		}
		dispatched++
		if err != nil {
			results <- independentActivationResult{index: index, outcome: outcome, delivery: delivery, nativeComplete: nativeComplete, err: err}
			stopErr = err
			cancel(err)
			break
		}
		go func(index int, delivery domain.StagedDelivery, nativeComplete bool, operation func(context.Context) (domain.ActivationOutcome, error)) {
			outcome, err := operation(effectContext)
			if domain.IsUnclosedClientProbe(err) {
				outcome.NativeEffect = domain.NativeEffectUncertain
			}
			if outcome.NativeEffect == domain.NativeEffectUncertain && err == nil {
				err = fmt.Errorf("independent native activation outcome is uncertain")
			}
			// A closed local probe does not cancel a sibling. Unknown cleanup or an
			// uncertain native effect cancels all in-flight commands before joining.
			if outcome.NativeEffect == domain.NativeEffectUncertain ||
				((errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) && !domain.IsIsolatedProbeTimeout(err)) {
				cancel(err)
			}
			results <- independentActivationResult{index: index, outcome: outcome, delivery: delivery, nativeComplete: nativeComplete, err: err}
		}(index, delivery, nativeComplete, operation)
	}
	// Every launched operation must return before the group lock/reservation is
	// released. Publish completion in observed order, so a quick Codex activation
	// becomes visible while Claude is still running.
	for range dispatched {
		got := <-results
		target := session.planned[got.index]
		outcome, delivery, nativeComplete, err := session.finishGroupActivation(target, got.outcome, got.delivery, got.nativeComplete, got.err)
		if publishErr := session.publishGroupActivation(len(session.planned), target, outcome, delivery, nativeComplete, err); publishErr != nil {
			if stopErr == nil {
				stopErr = publishErr
			}
			cancel(publishErr)
		}
	}
	if dispatched < len(session.planned) {
		stage, message := groupActivationStopReason(session.ctx, stopErr)
		session.markRemainingNotAttempted(dispatched, stage, message)
	}
	if session.externalFailed > 0 || stopErr != nil || session.ctx.Err() != nil {
		session.classifyActivationFailure()
		cause := session.firstActivationErr
		if cause == nil {
			cause = stopErr
		}
		if ctxErr := session.ctx.Err(); ctxErr != nil && !errors.Is(cause, ctxErr) {
			cause = errors.Join(cause, ctxErr)
		}
		return session.result, session.activationFailureError(cause)
	}
	session.result.Phase = GroupPhaseCompleted
	return session.result, nil
}
