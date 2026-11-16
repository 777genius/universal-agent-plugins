package usecase

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// A local probe deadline must stop safely without counting the untouched client
// as failed or describing the caller as canceled. The committed sibling survives.
func TestGroupActivationStopPreservesAttemptFacts(t *testing.T) {
	for _, stage := range []string{"timeout", "canceled", "deadline"} {
		t.Run(stage, func(t *testing.T) {
			service, store, cursor := serviceFixture(t)
			kiro := domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
			codex := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: canonicalCodexProfile(t, ".codex")}
			targets := []AddInput{
				addInput(t, cursor, "https://example.com/stop-facts"),
				addInput(t, kiro, "https://example.com/stop-facts"),
				addInput(t, codex, "https://example.com/stop-facts"),
			}
			ctx, cancel := context.WithCancel(context.Background())
			if stage == "deadline" {
				cancel()
				// The deadline starts after fixture creation and leaves a bounded
				// reserve for staging/commit even on a slower CI filesystem.
				ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			}
			defer cancel()
			activator := &stopSecondGroupActivator{stop: func() error {
				if stage == "canceled" {
					cancel()
					return ctx.Err()
				}
				probe := ctx
				if stage == "timeout" {
					var cancelProbe context.CancelFunc
					probe, cancelProbe = context.WithTimeout(ctx, 10*time.Millisecond)
					defer cancelProbe()
				}
				<-probe.Done()
				return probe.Err()
			}}
			service.Activator = activator
			result, err := service.AddGroup(ctx, GroupInput{Targets: targets, OperationGroupID: "stop-facts-" + stage, Confirmed: true})
			wantCause := context.DeadlineExceeded
			if stage == "canceled" {
				wantCause = context.Canceled
			}
			if !errors.Is(err, wantCause) || !strings.Contains(err.Error(), "1 of 3 client activations failed; 1 not attempted") {
				t.Fatalf("aggregate stop error = %v", err)
			}
			if stage == "timeout" && ctx.Err() != nil {
				t.Fatalf("local deadline canceled parent: %v", ctx.Err())
			}
			if activator.calls != 2 || result.Phase != GroupPhaseExternalPartialFailure || len(result.Receipts) != 3 {
				t.Fatalf("stop lost committed partial progress: calls=%d result=%+v", activator.calls, result)
			}
			if result.Targets[0].GroupPhase != GroupTargetExternalCompleted || result.Targets[0].Failure != nil || result.Targets[0].Activation.Activation != domain.ActivationActive {
				t.Fatalf("completed target changed: %+v", result.Targets[0])
			}
			for index, phase := range []GroupTargetPhase{GroupTargetExternalFailed, GroupTargetExternalNotAttempted} {
				target := result.Targets[index+1]
				if target.GroupPhase != phase || target.Failure == nil || target.Failure.Stage != stage {
					t.Fatalf("stop classification target %d = %+v", index+1, target)
				}
			}
			state, loadErr := store.Load()
			if loadErr != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 3 {
				t.Fatalf("stop lost managed registration: %+v %v", state, loadErr)
			}
		})
	}
}

type stopSecondGroupActivator struct {
	calls int
	stop  func() error
}

func (a *stopSecondGroupActivator) Activate(context.Context, domain.ActivationRequest) (domain.ActivationOutcome, error) {
	a.calls++
	outcome := domain.ActivationOutcome{Activation: domain.ActivationActive, Authentication: domain.AuthenticationNotRequired, Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled}
	if a.calls == 2 {
		outcome.Activation, outcome.Verification = domain.ActivationFailed, domain.VerificationFailed
		return outcome, a.stop()
	}
	return outcome, nil
}

func (*stopSecondGroupActivator) Deactivate(context.Context, domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	return domain.DeactivationOutcome{}, nil
}

func TestGroupContinuesOnlyAfterIsolatedProbeClosure(t *testing.T) {
	for _, unsafe := range []bool{false, true} {
		name := "closed local probe"
		if unsafe {
			name = "joined cleanup failure"
		}
		t.Run(name, func(t *testing.T) {
			service, store, cursor := serviceFixture(t)
			codex := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: canonicalCodexProfile(t, ".codex")}
			kiro := domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
			activator := &stopSecondGroupActivator{stop: func() error {
				err := error(&domain.IsolatedProbeTimeout{Operation: "Claude Code plugin listing", Budget: 30 * time.Second, Elapsed: 30 * time.Second, Err: context.DeadlineExceeded})
				if unsafe {
					return errors.Join(err, errors.New("containment close failed"))
				}
				return err
			}}
			service.Activator = activator
			result, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{
				addInput(t, cursor, "https://example.com/local-probe"), addInput(t, kiro, "https://example.com/local-probe"), addInput(t, codex, "https://example.com/local-probe"),
			}, OperationGroupID: "isolated-probe", Confirmed: true})
			if !errors.Is(err, context.DeadlineExceeded) || result.Targets[1].Failure == nil || result.Targets[1].Failure.Stage != "timeout" {
				t.Fatalf("failure facts: %+v %v", result, err)
			}
			wantPhase := GroupTargetExternalCompleted
			wantCalls := 3
			if unsafe {
				wantPhase = GroupTargetExternalNotAttempted
				wantCalls = 2
			}
			if activator.calls != wantCalls || result.Targets[2].GroupPhase != wantPhase {
				t.Fatalf("calls=%d targets=%+v", activator.calls, result.Targets)
			}
			state, loadErr := store.Load()
			if loadErr != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 3 {
				t.Fatalf("registration lost: %+v %v", state, loadErr)
			}
		})
	}
}

func TestSerialGroupRetainsCleanupCauseWhenActivationPersistenceFails(t *testing.T) {
	closeErr := errors.New("native registry process cleanup failed")
	marker := &domain.UnclosedClientProbe{Operation: "native client listing", Err: closeErr}
	firstErr := errors.New("earlier closed activation failed")
	for _, test := range []struct {
		name              string
		activationErr     error
		firstErr          error
		persist, unclosed bool
	}{
		{"bare cleanup", marker, nil, true, true},
		{"wrapped cleanup", fmt.Errorf("native observation: %w", marker), nil, true, true},
		{"joined cleanup", errors.Join(context.DeadlineExceeded, marker), nil, true, true},
		{"closed failure", errors.New("closed native activation failed"), nil, true, false},
		{"earlier failure then cleanup and persist", marker, firstErr, true, true},
		{"earlier failure then cleanup", marker, firstErr, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, store, cursor := serviceFixture(t)
			claude, codex, _, _ := mixedClients(t)
			persistErr := errors.New("activation state save failed")
			failingStore := &activationPublicationFailureStore{Store: store, cause: persistErr}
			service.StateStore = failingStore
			activator := &serialCleanupFailureActivator{first: test.firstErr}
			activator.stop = func() error {
				if test.persist {
					failingStore.armed.Store(true)
				}
				return test.activationErr
			}
			service.Activator = activator // This fixture has no independent-activation capability.
			result, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{
				addInput(t, cursor, "https://example.com/serial-cleanup"),
				addInput(t, claude, "https://example.com/serial-cleanup"),
				addInput(t, codex, "https://example.com/serial-cleanup"),
			}, OperationGroupID: "serial-cleanup", Confirmed: true})
			wantFailures := int32(0)
			stage := "activation"
			if test.persist {
				wantFailures, stage = 1, "persist"
			}
			if activator.calls != 2 || failingStore.failures.Load() != wantFailures || errors.Is(err, persistErr) != test.persist {
				t.Fatalf("persistence refusal or remaining-client stop changed: calls=%d saves=%d err=%v", activator.calls, failingStore.failures.Load(), err)
			}
			wantPhase, firstPhase := GroupPhaseExternalPartialFailure, GroupTargetExternalCompleted
			if test.firstErr != nil {
				wantPhase, firstPhase = GroupPhaseManagedActivationFailed, GroupTargetExternalFailed
			}
			if result.Phase != wantPhase || len(result.Targets) != 3 || len(result.Receipts) != 3 {
				t.Fatalf("committed group facts changed: %+v", result)
			}
			want := []GroupTargetPhase{firstPhase, GroupTargetExternalFailed, GroupTargetExternalNotAttempted}
			for index, phase := range want {
				if result.Targets[index].GroupPhase != phase {
					t.Fatalf("target %d phase=%s, want %s", index, result.Targets[index].GroupPhase, phase)
				}
			}
			if (result.Targets[0].Failure != nil) != (test.firstErr != nil) || result.Targets[1].Failure == nil || result.Targets[1].Failure.Stage != stage || result.Targets[2].Failure == nil || result.Targets[2].Failure.Stage != stage {
				t.Fatalf("original success/failure or stop classification changed: %+v", result.Targets)
			}
			if !test.unclosed {
				if domain.IsUnclosedClientProbe(err) || errors.Is(err, test.activationErr) {
					t.Fatalf("ordinary activation failure must keep the existing persistence-only cause: %v", err)
				}
				return
			}
			var retained *domain.UnclosedClientProbe
			if !errors.As(err, &retained) || retained != marker || !errors.Is(err, closeErr) || !errors.Is(err, test.activationErr) {
				t.Fatalf("serial persistence hid the exact cleanup marker or cause: %v", err)
			}
		})
	}
}

type serialCleanupFailureActivator struct {
	stopSecondGroupActivator
	first error
}

func (activator *serialCleanupFailureActivator) Activate(ctx context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	outcome, err := activator.stopSecondGroupActivator.Activate(ctx, request)
	if activator.calls == 1 && activator.first != nil {
		outcome.Activation, outcome.Verification = domain.ActivationFailed, domain.VerificationFailed
		err = activator.first
	}
	return outcome, err
}
