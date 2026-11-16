package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// Independent effects are opt-in per client. Adapters that may mutate the shared
// kernel during activation keep the serial coordinator. When at least two
// distinct clients can run independently they are moved to the front of the plan
// and run concurrently with the serial remainder; one client alone gains nothing.
// The planned order never decides result order: results are addressed by index.
//
// The reorder is temporary: operation identifiers and staging cleanup address
// planned targets by their original position, so restore puts that order back.
func (session *groupSession) independentGroupActivator() (activator ports.IndependentClientActivator, count int, restore func(), ok bool) {
	activator, ok = session.service.Activator.(ports.IndependentClientActivator)
	if !ok || len(session.planned) < 2 {
		return nil, 0, nil, false
	}
	seen := map[domain.ClientID]bool{}
	var independentAt, serialAt []int
	for position, target := range session.planned {
		request := domain.ActivationRequest{Client: target.input.Client, Plan: target.plan}
		if !seen[target.input.Client.ClientID] && target.plan.SelectedDelivery.IsZero() && activator.SupportsIndependentActivation(request) {
			seen[target.input.Client.ClientID] = true
			independentAt = append(independentAt, position)
			continue
		}
		serialAt = append(serialAt, position)
	}
	if len(independentAt) < 2 {
		return nil, 0, nil, false
	}
	order := make([]int, 0, len(session.planned))
	order = append(order, independentAt...)
	order = append(order, serialAt...)
	original := session.planned
	reordered := make([]plannedGroupTarget, len(original))
	for position, from := range order {
		reordered[position] = original[from]
	}
	session.planned = reordered
	restore = func() {
		put := make([]plannedGroupTarget, len(original))
		for position, from := range order {
			put[from] = session.planned[position]
		}
		session.planned = put
	}
	return activator, len(independentAt), restore, true
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
	// count is the planned prefix that runs independently.
	count      int
	session    *groupSession
	activator  ports.IndependentClientActivator
	ctx        context.Context
	cancel     context.CancelCauseFunc
	results    chan independentActivationResult
	dispatched int
	collected  int
	stopErr    error
}

// Only client effects run in goroutines. Request preparation, journal changes,
// host callbacks, publication, and progress callbacks stay on this coordinator.
// Serial targets run on the coordinator while the independent effects are in
// flight; finished independent results are published between serial targets and
// the rest once the serial remainder returns.
func (session *groupSession) activateIndependentGroupTargets(activator ports.IndependentClientActivator, count int) (GroupResult, error) {
	session.externalCompleted, session.externalFailed = 0, 0
	session.firstActivationErr = nil
	effectContext, cancel := context.WithCancelCause(session.ctx)
	defer cancel(nil)
	run := independentActivationRun{session: session, activator: activator, ctx: effectContext, cancel: cancel, count: count,
		results: make(chan independentActivationResult, count)}
	run.dispatch()
	serialCause := run.activateSerialRemainder()
	run.collect()
	if serialCause != nil {
		// The serial stop happened before the independent results were
		// published; classify and count again with every outcome in.
		session.classifyActivationFailure()
		return session.result, session.activationFailureError(errors.Join(serialCause, run.stopErr))
	}
	return run.finish()
}

// activateSerialRemainder runs the planned targets that cannot be independent
// and returns the cause that stopped the group, nil when every one was reached.
// A stop here cancels in-flight independent effects so they are joined quickly;
// a stop raised by an independent effect (uncertain or unclosed client process,
// cancellation) stops the remaining serial targets too, as it would serially.
func (run *independentActivationRun) activateSerialRemainder() error {
	session := run.session
	if run.dispatched < run.count {
		return nil
	}
	for index := run.count; index < len(session.planned); index++ {
		run.drainReady()
		if run.ctx.Err() != nil {
			cause := context.Cause(run.ctx)
			stage, message := groupActivationStopReason(session.ctx, cause)
			session.markRemainingNotAttempted(index, stage, message)
			return cause
		}
		if err := session.activateOneGroupTarget(index, session.planned[index]); err != nil {
			cause := errors.Unwrap(err)
			if cause == nil {
				cause = err
			}
			run.cancel(cause)
			return cause
		}
	}
	return nil
}

func (run *independentActivationRun) dispatch() {
	for index, target := range run.session.planned[:run.count] {
		if run.ctx.Err() != nil {
			run.stopErr = context.Cause(run.ctx)
			break
		}
		operation, result := run.prepare(index, target)
		run.dispatched++
		if result.err != nil {
			// The failure belongs to this target and is published as such. Only a
			// failure that makes later activation unsafe stops the others.
			run.results <- result
			if stopsGroupActivation(run.session.ctx, result.err) {
				run.stopErr = result.err
				run.cancel(result.err)
				break
			}
			continue
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
	for run.collected < run.dispatched {
		run.publish(<-run.results)
	}
}

// drainReady publishes independent results that already finished, without
// waiting, so a long serial client does not delay their progress or durability.
func (run *independentActivationRun) drainReady() {
	for run.collected < run.dispatched {
		select {
		case got := <-run.results:
			run.publish(got)
		default:
			return
		}
	}
}

func (run *independentActivationRun) publish(got independentActivationResult) {
	run.collected++
	target := run.session.planned[got.index]
	outcome, delivery, nativeComplete, err := run.session.finishGroupActivation(target, got.outcome, got.delivery, got.nativeComplete, got.err)
	publishErr := run.session.publishGroupActivation(len(run.session.planned), target, outcome, delivery, nativeComplete, err)
	// A persistence failure can replace the activation error in publication.
	// Retain unconfirmed process cleanup before returning either failure.
	if domain.IsUnclosedClientProbe(err) && !errors.Is(publishErr, err) {
		publishErr = errors.Join(publishErr, err)
	}
	if publishErr != nil {
		run.stopErr = errors.Join(run.stopErr, publishErr)
		run.cancel(publishErr)
	}
}

func (run *independentActivationRun) finish() (GroupResult, error) {
	session := run.session
	if run.dispatched < run.count {
		// The independent prefix stopped early; the serial remainder never started.
		stage, message := groupActivationStopReason(session.ctx, run.stopErr)
		session.markRemainingNotAttempted(run.dispatched, stage, message)
	}
	if session.externalFailed > 0 || run.stopErr != nil || session.ctx.Err() != nil {
		session.classifyActivationFailure()
		cause := session.firstActivationErr
		if cause == nil {
			cause = run.stopErr
		} else if run.stopErr != nil {
			cause = errors.Join(cause, run.stopErr)
		}
		if ctxErr := session.ctx.Err(); ctxErr != nil && !errors.Is(cause, ctxErr) {
			cause = errors.Join(cause, ctxErr)
		}
		return session.result, session.activationFailureError(cause)
	}
	session.result.Phase = GroupPhaseCompleted
	return session.result, nil
}
