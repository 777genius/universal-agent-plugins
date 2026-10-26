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

type independentClientOperation func(context.Context) (domain.ActivationOutcome, error)

type independentActivationRun struct {
	session    *groupSession
	activator  ports.IndependentClientActivator
	ctx        context.Context
	cancel     context.CancelCauseFunc
	results    chan independentActivationResult
	dispatched int
	stopErr    error
}

// Only client effects run in goroutines. Request preparation, journal changes,
// host callbacks, publication, and progress callbacks stay on this coordinator.
func (session *groupSession) activateIndependentGroupTargets(activator ports.IndependentClientActivator) (GroupResult, error) {
	session.externalCompleted, session.externalFailed = 0, 0
	session.firstActivationErr = nil
	effectContext, cancel := context.WithCancelCause(session.ctx)
	defer cancel(nil)
	run := independentActivationRun{session: session, activator: activator, ctx: effectContext, cancel: cancel,
		results: make(chan independentActivationResult, len(session.planned))}
	run.dispatch()
	run.collect()
	return run.finish()
}

func (run *independentActivationRun) dispatch() {
	for index, target := range run.session.planned {
		if run.ctx.Err() != nil {
			run.stopErr = context.Cause(run.ctx)
			break
		}
		operation, result := run.prepare(index, target)
		run.dispatched++
		if result.err != nil {
			run.results <- result
			run.stopErr = result.err
			run.cancel(result.err)
			break
		}
		go run.execute(operation, result)
	}
}

// Preparation stays serial: shared journals, client revalidation, and host
// callbacks finish before exposing this target's independent effect operation.
func (run *independentActivationRun) prepare(index int, target plannedGroupTarget) (independentClientOperation, independentActivationResult) {
	request, outcome, delivery, nativeComplete, err := run.session.groupActivationRequest(target)
	result := independentActivationResult{index: index, outcome: outcome, delivery: delivery, nativeComplete: nativeComplete, err: err}
	if err != nil {
		return nil, result
	}
	request, result.outcome, result.err = run.session.service.prepareNativeActivation(run.ctx, run.session.installationID, target.clientBindingID, request)
	if result.err != nil {
		return nil, result
	}
	if preparation := run.session.service.ClientPreparation; preparation != nil && !request.VerifyOnly {
		result.err = preparation.RevalidateClient(run.ctx, request.Client, request.Plan)
		if result.err != nil {
			return nil, result
		}
	}
	operation, err := run.activator.PrepareIndependentActivation(run.ctx, request)
	if err == nil {
		err = run.ctx.Err()
	}
	if err == nil && operation == nil {
		err = fmt.Errorf("independent activator returned no operation")
	}
	if err == nil {
		err = run.session.reportGroupProgress(target, GroupProgressActivating)
	}
	result.err = err
	return operation, result
}

// execute reads only the effect context and cancellation/channel handles. It
// does not touch shared installation state or invoke coordinator observers.
func (run *independentActivationRun) execute(operation independentClientOperation, result independentActivationResult) {
	result.outcome, result.err = operation(run.ctx)
	if domain.IsUnclosedClientProbe(result.err) {
		result.outcome.NativeEffect = domain.NativeEffectUncertain
	}
	if result.outcome.NativeEffect == domain.NativeEffectUncertain && result.err == nil {
		result.err = fmt.Errorf("independent native activation outcome is uncertain")
	}
	// A closed local probe does not cancel a sibling. Unknown cleanup or an
	// uncertain native effect cancels all in-flight commands before joining.
	if result.outcome.NativeEffect == domain.NativeEffectUncertain ||
		((errors.Is(result.err, context.Canceled) || errors.Is(result.err, context.DeadlineExceeded)) && !domain.IsIsolatedProbeTimeout(result.err)) {
		run.cancel(result.err)
	}
	run.results <- result
}

// Every launched operation returns before the group lock/reservation is
// released. Publish in observed completion order while other effects continue.
func (run *independentActivationRun) collect() {
	for range run.dispatched {
		got := <-run.results
		target := run.session.planned[got.index]
		outcome, delivery, nativeComplete, err := run.session.finishGroupActivation(target, got.outcome, got.delivery, got.nativeComplete, got.err)
		if publishErr := run.session.publishGroupActivation(len(run.session.planned), target, outcome, delivery, nativeComplete, err); publishErr != nil {
			if run.stopErr == nil {
				run.stopErr = publishErr
			}
			run.cancel(publishErr)
		}
	}
}

func (run *independentActivationRun) finish() (GroupResult, error) {
	session := run.session
	if run.dispatched < len(session.planned) {
		stage, message := groupActivationStopReason(session.ctx, run.stopErr)
		session.markRemainingNotAttempted(run.dispatched, stage, message)
	}
	if session.externalFailed > 0 || run.stopErr != nil || session.ctx.Err() != nil {
		session.classifyActivationFailure()
		cause := session.firstActivationErr
		if cause == nil {
			cause = run.stopErr
		}
		if ctxErr := session.ctx.Err(); ctxErr != nil && !errors.Is(cause, ctxErr) {
			cause = errors.Join(cause, ctxErr)
		}
		return session.result, session.activationFailureError(cause)
	}
	session.result.Phase = GroupPhaseCompleted
	return session.result, nil
}
