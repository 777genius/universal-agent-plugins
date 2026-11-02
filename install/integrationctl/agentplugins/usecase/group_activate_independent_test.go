package usecase

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type independentFixtureActivator struct {
	release chan struct{}
	started chan domain.ClientID
	mode    string
	joined  atomic.Int32
}

func (a *independentFixtureActivator) Activate(context.Context, domain.ActivationRequest) (domain.ActivationOutcome, error) {
	return domain.ActivationOutcome{}, fmt.Errorf("serial effect path used")
}
func (a *independentFixtureActivator) Deactivate(context.Context, domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	return domain.DeactivationOutcome{}, nil
}
func (a *independentFixtureActivator) SupportsIndependentActivation(request domain.ActivationRequest) bool {
	return request.Client.ClientID == domain.ClientClaude || request.Client.ClientID == domain.ClientCodex
}
func (a *independentFixtureActivator) PrepareIndependentActivation(_ context.Context, request domain.ActivationRequest) (func(context.Context) (domain.ActivationOutcome, error), error) {
	return func(ctx context.Context) (domain.ActivationOutcome, error) {
		defer a.joined.Add(1)
		a.started <- request.Client.ClientID
		if request.Client.ClientID == domain.ClientClaude {
			select {
			case <-a.release:
			case <-ctx.Done():
				return domain.ActivationOutcome{}, ctx.Err()
			}
			if a.mode == "local timeout" {
				return domain.ActivationOutcome{}, &domain.IsolatedProbeTimeout{Operation: "Claude listing", Budget: 30 * time.Second, Elapsed: 30 * time.Second, Err: context.DeadlineExceeded}
			}
			if a.mode == "generic cleanup" {
				return domain.ActivationOutcome{}, &domain.UnclosedClientProbe{Operation: "Claude listing", Err: errors.New("containment close failed")}
			}
			if a.mode == "unknown cleanup" {
				return domain.ActivationOutcome{}, context.DeadlineExceeded
			}
		} else if a.mode == "unknown cleanup" || a.mode == "generic cleanup" || a.mode == "parent cancel" {
			<-ctx.Done()
			return domain.ActivationOutcome{}, ctx.Err()
		}
		return domain.ActivationOutcome{Activation: domain.ActivationActive, Authentication: request.Plan.Authentication, Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled}, nil
	}, nil
}

func TestIndependentClientActivationPublishesCodexBeforeClaudeReturns(t *testing.T) {
	for _, mode := range []string{"success", "local timeout", "unknown cleanup", "generic cleanup", "parent cancel"} {
		t.Run(mode, func(t *testing.T) {
			service, store, _ := serviceFixture(t)
			a := &independentFixtureActivator{release: make(chan struct{}), started: make(chan domain.ClientID, 2), mode: mode}
			service.Activator = a
			claude := domain.DetectedClient{ClientID: domain.ClientClaude, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".claude"), ExecutablePath: "/TEST/claude"}
			codex := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: canonicalCodexProfile(t, ".codex")}
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			progress := make(chan GroupProgressEvent, 8)
			type completion struct {
				result GroupResult
				err    error
			}
			done := make(chan completion, 1)
			finished := make(chan struct{})
			inputs := []AddInput{addInput(t, claude, "https://example.com/independent"), addInput(t, codex, "https://example.com/independent")}
			go func() {
				defer close(finished)
				result, err := service.AddGroup(ctx, GroupInput{Targets: inputs, OperationGroupID: "independent-activation", Confirmed: true, Progress: func(event GroupProgressEvent) {
					if event.Phase == GroupProgressActivated {
						progress <- event
					}
				}})
				done <- completion{result, err}
			}()
			defer func() {
				cancel()
				select {
				case <-finished:
				case <-time.After(30 * time.Second):
					t.Error("fixture did not stop after cancellation")
				}
			}()
			for range 2 {
				select {
				case <-a.started:
				case got := <-done:
					t.Fatalf("group returned before both clients started: %+v %v", got.result, got.err)
				case <-time.After(30 * time.Second):
					t.Fatal("independent effects did not start")
				}
			}
			if mode == "success" || mode == "local timeout" {
				select {
				case event := <-progress:
					if event.ClientID != domain.ClientCodex || event.Result.GroupPhase != GroupTargetExternalCompleted {
						t.Fatalf("first completed client = %+v", event)
					}
				case <-time.After(15 * time.Second):
					t.Fatal("Codex was blocked by Claude")
				}
			}
			if mode == "parent cancel" {
				cancel()
			} else {
				close(a.release)
			}
			var got completion
			select {
			case got = <-done:
			case <-time.After(30 * time.Second):
				t.Fatal("group did not join all effects")
			}
			if a.joined.Load() != 2 {
				t.Fatalf("returned with unjoined effects: %d", a.joined.Load())
			}
			if mode == "success" {
				if got.err != nil || got.result.Phase != GroupPhaseCompleted {
					t.Fatalf("result=%+v err=%v", got.result, got.err)
				}
			} else if got.err == nil {
				t.Fatal("failed activation reported success")
			}
			if mode == "local timeout" && (got.result.Targets[0].GroupPhase != GroupTargetExternalFailed || got.result.Targets[1].GroupPhase != GroupTargetExternalCompleted) {
				t.Fatalf("timeout lost independent success: %+v", got.result)
			}
			state, err := store.Load()
			if err != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 2 {
				t.Fatalf("lost committed registration: %+v %v", state, err)
			}
			if mode == "unknown cleanup" || mode == "generic cleanup" || mode == "parent cancel" {
				if got.result.Targets[0].GroupPhase != GroupTargetExternalFailed || got.result.Targets[1].GroupPhase != GroupTargetExternalFailed {
					t.Fatalf("canceled effects fabricated completion: %+v", got.result)
				}
			}
			if mode == "parent cancel" && !errors.Is(got.err, context.Canceled) {
				t.Fatalf("parent cause lost: %v", got.err)
			}
		})
	}
}
