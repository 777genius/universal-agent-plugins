package usecase

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
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

// serialAwareActivator completes serial clients immediately and holds Claude's
// independent effect until released, so a test can prove they overlap.
type serialAwareActivator struct {
	independentFixtureActivator
	serialRan chan domain.ClientID
}

func (a *serialAwareActivator) Activate(_ context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	a.serialRan <- request.Client.ClientID
	return domain.ActivationOutcome{Activation: domain.ActivationActive, Authentication: request.Plan.Authentication, Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled}, nil
}

func TestIndependentActivationRunsSerialClientsWhileEffectsAreInFlight(t *testing.T) {
	service, store, _ := serviceFixture(t)
	a := &serialAwareActivator{
		independentFixtureActivator: independentFixtureActivator{release: make(chan struct{}), started: make(chan domain.ClientID, 2), mode: "success"},
		serialRan:                   make(chan domain.ClientID, 1),
	}
	service.Activator = a
	claude := domain.DetectedClient{ClientID: domain.ClientClaude, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".claude"), ExecutablePath: "/TEST/claude"}
	codex := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: canonicalCodexProfile(t, ".codex")}
	cursor := domain.DetectedClient{ClientID: domain.ClientCursor, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".cursor")}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	// A serial client is listed first: the plan must still overlap it with the
	// independent pair, and results must keep the caller's order.
	inputs := []AddInput{addInput(t, cursor, "https://example.com/mixed"), addInput(t, claude, "https://example.com/mixed"), addInput(t, codex, "https://example.com/mixed")}
	type completion struct {
		result GroupResult
		err    error
	}
	done := make(chan completion, 1)
	go func() {
		result, err := service.AddGroup(ctx, GroupInput{Targets: inputs, OperationGroupID: "mixed-activation", Confirmed: true})
		done <- completion{result, err}
	}()
	select {
	case got := <-a.serialRan:
		if got != domain.ClientCursor {
			t.Fatalf("serial client = %s", got)
		}
	case got := <-done:
		t.Fatalf("group returned before the serial client ran: %+v %v", got.result, got.err)
	case <-time.After(30 * time.Second):
		t.Fatal("the serial client never ran while Claude was still pending")
	}
	select {
	case got := <-done:
		t.Fatalf("group must wait for the pending Claude effect: %+v %v", got.result, got.err)
	default:
	}
	close(a.release)
	var got completion
	select {
	case got = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("group did not finish after Claude was released")
	}
	if got.err != nil || got.result.Phase != GroupPhaseCompleted {
		t.Fatalf("result=%+v err=%v", got.result, got.err)
	}
	for index, want := range []domain.ClientID{domain.ClientCursor, domain.ClientClaude, domain.ClientCodex} {
		if got.result.Targets[index].Plan.ClientID != want || got.result.Targets[index].GroupPhase != GroupTargetExternalCompleted {
			t.Fatalf("target %d = %+v, want %s completed", index, got.result.Targets[index], want)
		}
	}
	if a.joined.Load() != 2 {
		t.Fatalf("independent effects not joined: %d", a.joined.Load())
	}
	if state, err := store.Load(); err != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 3 {
		t.Fatalf("lost committed registration: %+v %v", state, err)
	}
}

// A refusal while preparing one independent client belongs to that client. It
// must not cancel a healthy sibling or skip the serial remainder.
type prepareRefusingActivator struct{ serialAwareActivator }

func (a *prepareRefusingActivator) PrepareIndependentActivation(ctx context.Context, request domain.ActivationRequest) (func(context.Context) (domain.ActivationOutcome, error), error) {
	if request.Client.ClientID == domain.ClientCodex {
		return nil, errors.New("projection could not be prepared for this client")
	}
	return a.serialAwareActivator.PrepareIndependentActivation(ctx, request)
}

func mixedClients(t *testing.T) (claude, codex, cursor, gemini domain.DetectedClient) {
	t.Helper()
	claude = domain.DetectedClient{ClientID: domain.ClientClaude, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".claude"), ExecutablePath: "/TEST/claude"}
	codex = domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: canonicalCodexProfile(t, ".codex")}
	cursor = domain.DetectedClient{ClientID: domain.ClientCursor, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".cursor")}
	canonicalHome, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gemini = domain.DetectedClient{ClientID: domain.ClientGemini, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(canonicalHome, ".gemini")}
	return
}

func TestIndependentPrepareRefusalStaysWithItsOwnClient(t *testing.T) {
	service, _, _ := serviceFixture(t)
	a := &prepareRefusingActivator{serialAwareActivator{
		independentFixtureActivator: independentFixtureActivator{release: make(chan struct{}), started: make(chan domain.ClientID, 4), mode: "success"},
		serialRan:                   make(chan domain.ClientID, 4),
	}}
	close(a.release)
	service.Activator = a
	claude, codex, cursor, _ := mixedClients(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	// The refusing client is placed between the healthy independent one and a serial one.
	inputs := []AddInput{addInput(t, claude, "https://example.com/refusal"), addInput(t, codex, "https://example.com/refusal"), addInput(t, cursor, "https://example.com/refusal")}
	result, err := service.AddGroup(ctx, GroupInput{Targets: inputs, OperationGroupID: "prepare-refusal", Confirmed: true})
	if err == nil {
		t.Fatal("a refused client must be reported")
	}
	want := []GroupTargetPhase{GroupTargetExternalCompleted, GroupTargetExternalFailed, GroupTargetExternalCompleted}
	for index, phase := range want {
		if result.Targets[index].GroupPhase != phase {
			t.Fatalf("target %d (%s) = %s, want %s: %+v", index, result.Targets[index].Plan.ClientID, result.Targets[index].GroupPhase, phase, result.Targets[index].Failure)
		}
	}
	if result.Phase != GroupPhaseExternalPartialFailure {
		t.Fatalf("phase %s", result.Phase)
	}
}

// An independent effect that ends with an unconfirmed client process makes
// activating anything else unsafe, exactly as it does in the serial path.
type unclosedProbeActivator struct {
	serialAwareActivator
}

func (a *unclosedProbeActivator) PrepareIndependentActivation(_ context.Context, request domain.ActivationRequest) (func(context.Context) (domain.ActivationOutcome, error), error) {
	return func(ctx context.Context) (domain.ActivationOutcome, error) {
		defer a.joined.Add(1)
		if request.Client.ClientID == domain.ClientClaude {
			return domain.ActivationOutcome{}, &domain.UnclosedClientProbe{Operation: "Claude listing", Err: errors.New("containment close failed")}
		}
		<-ctx.Done()
		return domain.ActivationOutcome{}, ctx.Err()
	}, nil
}

func (a *unclosedProbeActivator) Activate(_ context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	a.serialRan <- request.Client.ClientID
	if request.Client.ClientID == domain.ClientCursor {
		// Hold the coordinator until Claude's unclosed probe was observed.
		deadline := time.Now().Add(10 * time.Second)
		for a.joined.Load() < 1 && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		time.Sleep(150 * time.Millisecond)
	}
	return domain.ActivationOutcome{Activation: domain.ActivationActive, Authentication: request.Plan.Authentication, Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled}, nil
}

func TestIndependentUnclosedProbeStopsTheRemainingSerialClients(t *testing.T) {
	service, _, _ := serviceFixture(t)
	a := &unclosedProbeActivator{serialAwareActivator: serialAwareActivator{
		independentFixtureActivator: independentFixtureActivator{release: make(chan struct{}), started: make(chan domain.ClientID, 4), mode: "success"},
		serialRan:                   make(chan domain.ClientID, 4),
	}}
	service.Activator = a
	claude, codex, cursor, gemini := mixedClients(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	inputs := []AddInput{addInput(t, claude, "https://example.com/unclosed"), addInput(t, codex, "https://example.com/unclosed"), addInput(t, cursor, "https://example.com/unclosed"), addInput(t, gemini, "https://example.com/unclosed")}
	result, err := service.AddGroup(ctx, GroupInput{Targets: inputs, OperationGroupID: "unclosed-probe", Confirmed: true})
	if err == nil {
		t.Fatal("an unconfirmed client process must fail the group")
	}
	if got := result.Targets[3].GroupPhase; got != GroupTargetExternalNotAttempted {
		t.Fatalf("the second serial client must not run after an unclosed probe, got %q (phase %s, err %v, %d targets)", got, result.Phase, err, len(result.Targets))
	}
	if a.joined.Load() != 2 {
		t.Fatalf("independent effects not joined: %d", a.joined.Load())
	}
}

type orderedFailureActivator struct {
	independentFixtureActivator
	ordinary error
	cleanup  error
}

func (a *orderedFailureActivator) PrepareIndependentActivation(_ context.Context, request domain.ActivationRequest) (func(context.Context) (domain.ActivationOutcome, error), error) {
	return func(ctx context.Context) (domain.ActivationOutcome, error) {
		defer a.joined.Add(1)
		if request.Client.ClientID == domain.ClientCodex {
			return domain.ActivationOutcome{}, a.ordinary
		}
		select {
		case <-a.release:
			return domain.ActivationOutcome{}, &domain.UnclosedClientProbe{Operation: "Claude listing", Err: a.cleanup}
		case <-ctx.Done():
			return domain.ActivationOutcome{}, ctx.Err()
		}
	}, nil
}

func TestIndependentLaterUnclosedProbePreservesBothFailures(t *testing.T) {
	service, _, _ := serviceFixture(t)
	ordinary := errors.New("Codex native command refused")
	cleanup := errors.New("Claude containment close failed")
	a := &orderedFailureActivator{
		independentFixtureActivator: independentFixtureActivator{release: make(chan struct{})},
		ordinary:                    ordinary, cleanup: cleanup,
	}
	service.Activator = a
	claude, codex, _, _ := mixedClients(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var published []domain.ClientID
	result, err := service.AddGroup(ctx, GroupInput{
		Targets:          []AddInput{addInput(t, claude, "https://example.com/ordered-failures"), addInput(t, codex, "https://example.com/ordered-failures")},
		OperationGroupID: "ordered-failures", Confirmed: true,
		Progress: func(event GroupProgressEvent) {
			if event.Phase == GroupProgressActivated {
				published = append(published, event.ClientID)
				if event.ClientID == domain.ClientCodex {
					// The ordinary failure must be published before Claude can fail.
					close(a.release)
				}
			}
		},
	})
	if len(published) != 2 || published[0] != domain.ClientCodex || published[1] != domain.ClientClaude || a.joined.Load() != 2 {
		t.Fatalf("effects were not published and joined in order: published=%v joined=%d err=%v", published, a.joined.Load(), err)
	}
	if !errors.Is(err, ordinary) || !errors.Is(err, cleanup) || !domain.IsUnclosedClientProbe(err) {
		t.Fatalf("later unsafe cleanup lost from aggregate: %v", err)
	}
	if ctx.Err() != nil || len(result.Targets) != 2 {
		t.Fatalf("unexpected outer cancellation or lost targets: %v %+v", ctx.Err(), result)
	}
	for _, target := range result.Targets {
		if target.GroupPhase != GroupTargetExternalFailed {
			t.Fatalf("failed native effect reported completion: %+v", target)
		}
	}
}

// Independent clients are run first internally, but the planned order must be
// restored so identifiers derived from a target's position stay stable.
func TestIndependentActivationRestoresPlannedOrder(t *testing.T) {
	service, _, _ := serviceFixture(t)
	a := &serialAwareActivator{
		independentFixtureActivator: independentFixtureActivator{release: make(chan struct{}), started: make(chan domain.ClientID, 4), mode: "success"},
		serialRan:                   make(chan domain.ClientID, 4),
	}
	close(a.release)
	service.Activator = a
	claude, codex, cursor, _ := mixedClients(t)
	inputs := []AddInput{addInput(t, cursor, "https://example.com/order"), addInput(t, claude, "https://example.com/order"), addInput(t, codex, "https://example.com/order")}
	session := &groupSession{service: service}
	session.planned = []plannedGroupTarget{{input: inputs[0], resultIndexes: []int{0}}, {input: inputs[1], resultIndexes: []int{1}}, {input: inputs[2], resultIndexes: []int{2}}}
	_, count, restore, ok := session.independentGroupActivator()
	if !ok || count != 2 || session.planned[0].input.Client.ClientID == domain.ClientCursor {
		t.Fatalf("independent clients must lead while running: %v %d", ok, count)
	}
	restore()
	for index, want := range []domain.ClientID{domain.ClientCursor, domain.ClientClaude, domain.ClientCodex} {
		if session.planned[index].input.Client.ClientID != want || session.planned[index].resultIndexes[0] != index {
			t.Fatalf("planned order not restored at %d: %v", index, session.planned[index].input.Client.ClientID)
		}
	}
}

// slowSerialActivator records the order of serial activations so a test can
// prove finished independent results are published during the serial remainder.
type slowSerialActivator struct {
	serialAwareActivator
	mu    *sync.Mutex
	order *[]string
}

func (a *slowSerialActivator) Activate(ctx context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if request.Client.ClientID == domain.ClientCursor {
		time.Sleep(300 * time.Millisecond)
	}
	a.mu.Lock()
	*a.order = append(*a.order, "serial:"+string(request.Client.ClientID))
	a.mu.Unlock()
	return domain.ActivationOutcome{Activation: domain.ActivationActive, Authentication: request.Plan.Authentication, Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled}, nil
}

func TestIndependentResultsArePublishedWhileSerialClientsStillRun(t *testing.T) {
	service, _, _ := serviceFixture(t)
	var mu sync.Mutex
	var order []string
	a := &slowSerialActivator{
		serialAwareActivator: serialAwareActivator{
			independentFixtureActivator: independentFixtureActivator{release: make(chan struct{}), started: make(chan domain.ClientID, 4), mode: "success"},
			serialRan:                   make(chan domain.ClientID, 4),
		},
		mu: &mu, order: &order,
	}
	close(a.release)
	service.Activator = a
	claude, codex, cursor, gemini := mixedClients(t)
	inputs := []AddInput{addInput(t, claude, "https://example.com/drain"), addInput(t, codex, "https://example.com/drain"), addInput(t, cursor, "https://example.com/drain"), addInput(t, gemini, "https://example.com/drain")}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	result, err := service.AddGroup(ctx, GroupInput{Targets: inputs, OperationGroupID: "drain", Confirmed: true, Progress: func(event GroupProgressEvent) {
		if event.Phase == GroupProgressActivated {
			mu.Lock()
			order = append(order, "activated:"+string(event.ClientID))
			mu.Unlock()
		}
	}})
	if err != nil || result.Phase != GroupPhaseCompleted {
		t.Fatalf("result=%+v err=%v", result.Phase, err)
	}
	position := map[string]int{}
	for index, entry := range order {
		position[entry] = index
	}
	if position["activated:codex"] > position["serial:gemini"] || position["activated:claude"] > position["serial:gemini"] {
		t.Fatalf("finished independent results waited for the whole serial remainder: %v", order)
	}
}
