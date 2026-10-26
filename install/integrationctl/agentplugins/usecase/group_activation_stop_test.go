package usecase

import (
	"context"
	"errors"
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
