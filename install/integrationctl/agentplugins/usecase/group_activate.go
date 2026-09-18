package usecase

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (service Service) activatePlannedGroupTargets(ctx context.Context, input GroupInput, replace bool, installationID string, planned []plannedGroupTarget, result *GroupResult) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	run := &groupActivationRun{result: result, planned: planned, attempted: make([]bool, len(planned))}
	var wait sync.WaitGroup
	for index := range planned {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			service.activatePlannedIndex(ctx, cancel, input, replace, installationID, index, run)
		}(index)
	}
	wait.Wait()
	run.mu.Lock()
	defer run.mu.Unlock()
	if run.persistErr != nil {
		run.markUnattempted("persist", "processing stopped because installation state could not be saved safely")
		run.classifyFailure()
		return fmt.Errorf("%d of %d client activations failed: %w", run.externalFailed+countNotAttempted(result.Targets), len(result.Targets), run.persistErr)
	}
	if err := ctx.Err(); err != nil {
		run.markUnattempted("canceled", "processing stopped because the operation was canceled before remaining clients could be activated safely")
		skipped := countNotAttempted(result.Targets)
		if run.externalFailed > 0 || skipped > 0 {
			run.classifyFailure()
			cause := run.firstActivationErr
			if cause == nil {
				cause = err
			}
			return fmt.Errorf("%d of %d client activations failed: %w", run.externalFailed+skipped, len(result.Targets), cause)
		}
	}
	return run.finish()
}

type groupActivationRun struct {
	mu                 sync.Mutex
	result             *GroupResult
	planned            []plannedGroupTarget
	attempted          []bool
	externalCompleted  int
	externalFailed     int
	firstActivationErr error
	persistErr         error
}

func (service Service) activatePlannedIndex(ctx context.Context, cancel context.CancelFunc, input GroupInput, replace bool, installationID string, index int, run *groupActivationRun) {
	if ctx.Err() != nil {
		return
	}
	target := run.planned[index]
	outcome, activationErr := service.runPlannedActivation(ctx, input, replace, target)
	run.mu.Lock()
	defer run.mu.Unlock()
	if run.persistErr != nil {
		return
	}
	run.attempted[index] = true
	persistErr := service.recordPlannedActivation(installationID, target, outcome, activationErr, run)
	if persistErr != nil {
		run.persistErr = persistErr
		cancel()
		return
	}
	if errors.Is(activationErr, context.Canceled) || errors.Is(activationErr, context.DeadlineExceeded) || ctx.Err() != nil {
		cancel()
	}
}

func (service Service) runPlannedActivation(ctx context.Context, input GroupInput, replace bool, target plannedGroupTarget) (domain.ActivationOutcome, error) {
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
		previous = append(previous, target.managed.NativeObjects...)
	}
	outcome, err := service.Activator.Activate(ctx, domain.ActivationRequest{
		Client: target.input.Client, Plan: target.plan, Delivery: delivery,
		DeclaredName: target.input.Envelope.Manifest.Name, Replacing: replace, Interactive: target.input.Interactive,
		BackendExecutable: target.input.BackendExecutable, PreviousNativeObjects: previous,
		VerifyOnly: target.noChange, ActivationComplete: target.input.ActivationComplete,
	})
	if input.Repair && target.managed != nil {
		outcome = preserveManagedAuthentication(outcome, target.managed.Authentication)
	}
	outcome, err = normalizePlannedActivation(ctx, target, outcome, err)
	if target.noChange && target.managed != nil && err == nil {
		outcome = service.restoreNoChangeActivation(target, outcome)
	}
	return outcome, err
}

func normalizePlannedActivation(ctx context.Context, target plannedGroupTarget, outcome domain.ActivationOutcome, err error) (domain.ActivationOutcome, error) {
	if err == nil && outcome.Activation == "" {
		if ctxErr := ctx.Err(); ctxErr != nil {
			err = ctxErr
		} else {
			err = fmt.Errorf("activator returned an empty activation outcome")
		}
	}
	if err == nil && (outcome.Activation == domain.ActivationFailed || outcome.Verification == domain.VerificationFailed || outcome.Authentication == domain.AuthenticationFailed) {
		err = fmt.Errorf("activator reported a failed activation outcome without an error")
	}
	if err != nil && outcome.Activation == "" {
		outcome = domain.ActivationOutcome{
			Activation: domain.ActivationFailed, Authentication: target.plan.Authentication,
			Policy: domain.PolicyAllowed, Verification: domain.VerificationFailed,
		}
	}
	return outcome, err
}

func (service Service) restoreNoChangeActivation(target plannedGroupTarget, outcome domain.ActivationOutcome) domain.ActivationOutcome {
	if !service.clientVerifierAvailable(target.input, target.plan) && target.managed.Activation == domain.ActivationActive && target.managed.Verification == domain.VerificationInstalled {
		outcome.Activation = target.managed.Activation
		outcome.Verification = target.managed.Verification
	}
	if (target.managed.Authentication == domain.AuthenticationPending || target.managed.Authentication == domain.AuthenticationNotChecked) && target.input.AuthComplete {
		outcome.Authentication = domain.AuthenticationComplete
		outcome.AuthenticationAttested = true
	} else if target.managed.Authentication != "" {
		outcome.Authentication = target.managed.Authentication
	}
	return outcome
}

func (service Service) recordPlannedActivation(installationID string, target plannedGroupTarget, outcome domain.ActivationOutcome, activationErr error, run *groupActivationRun) error {
	previous := []domain.NativeObjectOwnership(nil)
	if target.managed != nil {
		previous = target.managed.NativeObjects
	}
	lifecycleChanged, persistErr := service.updateActivationResult(installationID, target.clientBindingID, outcome, activationErr, previous)
	if lifecycleChanged {
		run.result.Mutated = true
	}
	for _, resultIndex := range target.resultIndexes {
		run.result.Targets[resultIndex].Activation = outcome
		run.result.Targets[resultIndex].Mutated = !target.noChange || lifecycleChanged
		if lifecycleChanged {
			run.result.Targets[resultIndex].NoChange = false
		}
		switch {
		case persistErr != nil:
			run.result.Targets[resultIndex].GroupPhase = GroupTargetExternalFailed
			run.result.Targets[resultIndex].Failure = &GroupTargetFailure{Stage: "persist", Message: persistErr.Error()}
		case activationErr != nil:
			run.result.Targets[resultIndex].GroupPhase = GroupTargetExternalFailed
			run.result.Targets[resultIndex].Failure = groupTargetFailureFromActivation(activationErr, outcome)
		default:
			run.result.Targets[resultIndex].GroupPhase = GroupTargetExternalCompleted
		}
	}
	if persistErr != nil {
		run.externalFailed += len(target.resultIndexes)
		return persistErr
	}
	if activationErr != nil {
		run.externalFailed += len(target.resultIndexes)
		if run.firstActivationErr == nil {
			run.firstActivationErr = activationErr
		}
		return nil
	}
	run.externalCompleted += len(target.resultIndexes)
	return nil
}

func (run *groupActivationRun) markUnattempted(stage, message string) {
	for index, target := range run.planned {
		if run.attempted[index] {
			continue
		}
		for _, resultIndex := range target.resultIndexes {
			run.result.Targets[resultIndex].GroupPhase = GroupTargetExternalNotAttempted
			run.result.Targets[resultIndex].Failure = &GroupTargetFailure{Stage: stage, Message: message}
		}
	}
}

func (run *groupActivationRun) classifyFailure() {
	if run.externalCompleted > 0 {
		run.result.Phase = GroupPhaseExternalPartialFailure
		return
	}
	run.result.Phase = GroupPhaseManagedActivationFailed
}

func (run *groupActivationRun) finish() error {
	if run.externalFailed == 0 {
		run.result.Phase = GroupPhaseCompleted
		return nil
	}
	run.classifyFailure()
	if run.firstActivationErr != nil {
		return fmt.Errorf("%d of %d client activations failed: %w", run.externalFailed, len(run.result.Targets), run.firstActivationErr)
	}
	return fmt.Errorf("%d of %d client activations failed", run.externalFailed, len(run.result.Targets))
}
