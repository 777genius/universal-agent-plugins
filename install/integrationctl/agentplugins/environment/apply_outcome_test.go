package environment

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// scriptedMutator answers per client so tests describe one agent's behavior.
type scriptedMutator struct {
	prepareErr map[string]error
	applyErr   map[string]error
	results    map[string]installer.Result
	applied    []string
	current    string
}

func (mutator *scriptedMutator) Prepare(_ context.Context, binding PreparedBinding) (*installer.PreparedOperation, error) {
	if err := mutator.prepareErr[binding.ClientID]; err != nil {
		return nil, err
	}
	mutator.current = binding.ClientID
	return &installer.PreparedOperation{}, nil
}

func (mutator *scriptedMutator) Apply(_ context.Context, _ *installer.PreparedOperation) (installer.Result, error) {
	client := mutator.current
	mutator.applied = append(mutator.applied, client)
	result, ok := mutator.results[client]
	if !ok {
		result = installer.Result{Outcome: installer.OutcomeCompleted, Mutated: true}
	}
	return result, mutator.applyErr[client]
}

func bindings(clients ...string) PreparedSync {
	plan := PreparedSync{}
	for _, client := range clients {
		plan.Bindings = append(plan.Bindings, PreparedBinding{Alias: "demo", ClientID: client, Action: ActionInstall})
	}
	return plan
}

func statuses(outcomes []BindingOutcome) []BindingStatus {
	out := make([]BindingStatus, len(outcomes))
	for index, outcome := range outcomes {
		out[index] = outcome.Status
	}
	return out
}

func TestApplySyncOneFailingClientDoesNotBlockOthers(t *testing.T) {
	cases := map[string]*scriptedMutator{
		"prepare error":      {prepareErr: map[string]error{"codex": errors.New("codex is not detected")}},
		"apply error":        {applyErr: map[string]error{"codex": errors.New("native plugin install failed")}},
		"incomplete outcome": {results: map[string]installer.Result{"codex": {Outcome: installer.OutcomeIncomplete, Reason: "activation pending"}}},
		"conflict outcome":   {results: map[string]installer.Result{"codex": {Outcome: installer.OutcomeConflict, Reason: "foreign destination"}}},
	}
	for name, mutator := range cases {
		t.Run(name, func(t *testing.T) {
			outcomes, err := (Coordinator{Mutator: mutator}).ApplySync(context.Background(), bindings("claude", "codex", "gemini"), true)
			want := []BindingStatus{BindingApplied, BindingFailed, BindingApplied}
			if got := statuses(outcomes); fmt.Sprint(got) != fmt.Sprint(want) {
				t.Fatalf("statuses %v, want %v", got, want)
			}
			var syncErr *SyncError
			if !errors.As(err, &syncErr) || syncErr.Stopped != nil {
				t.Fatalf("want a non-stopped SyncError, got %v", err)
			}
			if outcomes[1].Err == nil {
				t.Fatal("the failed binding must carry its own cause")
			}
		})
	}
}

func TestApplySyncRecoveryRequiredStopsRemainingBindings(t *testing.T) {
	mutator := &scriptedMutator{prepareErr: map[string]error{"codex": installer.ErrRecoveryRequired}}
	outcomes, err := (Coordinator{Mutator: mutator}).ApplySync(context.Background(), bindings("claude", "codex", "gemini"), true)
	want := []BindingStatus{BindingApplied, BindingFailed, BindingNotAttempted}
	if got := statuses(outcomes); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("statuses %v, want %v", got, want)
	}
	if !errors.Is(err, installer.ErrRecoveryRequired) {
		t.Fatalf("recovery cause must stay classifiable, got %v", err)
	}
	if len(mutator.applied) != 1 {
		t.Fatalf("no binding may run after recovery is required: %v", mutator.applied)
	}
	if !errors.Is(outcomes[2].Err, installer.ErrRecoveryRequired) {
		t.Fatal("a skipped binding must say why it was skipped")
	}
}

func TestApplySyncStopMarkerAndResultOutcomesEndTheSync(t *testing.T) {
	cases := map[string]*scriptedMutator{
		"marked guard":     {prepareErr: map[string]error{"codex": Stop(errors.New("inputs changed after review"))}},
		"plan changed":     {applyErr: map[string]error{"codex": installer.ErrPlanChanged}},
		"recovery outcome": {results: map[string]installer.Result{"codex": {Outcome: installer.OutcomeRecovery}}},
		"canceled outcome": {results: map[string]installer.Result{"codex": {Outcome: installer.OutcomeCancelled}}},
	}
	for name, mutator := range cases {
		t.Run(name, func(t *testing.T) {
			outcomes, err := (Coordinator{Mutator: mutator}).ApplySync(context.Background(), bindings("claude", "codex", "gemini"), true)
			if outcomes[2].Status != BindingNotAttempted {
				t.Fatalf("later binding must not run after a stopping failure: %v", statuses(outcomes))
			}
			var syncErr *SyncError
			if !errors.As(err, &syncErr) || syncErr.Stopped == nil {
				t.Fatalf("want a stopped SyncError, got %v", err)
			}
		})
	}
}

func TestApplySyncCanceledContextStopsBeforeLaterBindings(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	mutator := &cancelingMutator{scriptedMutator: &scriptedMutator{}, cancel: cancel}
	outcomes, err := (Coordinator{Mutator: mutator}).ApplySync(ctx, bindings("claude", "codex"), true)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	if outcomes[1].Status != BindingNotAttempted {
		t.Fatalf("statuses %v", statuses(outcomes))
	}
}

type cancelingMutator struct {
	*scriptedMutator
	cancel context.CancelFunc
}

func (mutator *cancelingMutator) Apply(ctx context.Context, prepared *installer.PreparedOperation) (installer.Result, error) {
	mutator.cancel()
	return installer.Result{Outcome: installer.OutcomeCompleted, Mutated: true}, ctx.Err()
}

func TestApplySyncRejectsPlanWithMissingActionBeforeMutating(t *testing.T) {
	mutator := &scriptedMutator{}
	plan := bindings("claude", "codex")
	plan.Bindings[1].Action = ""
	if _, err := (Coordinator{Mutator: mutator}).ApplySync(context.Background(), plan, true); !errors.Is(err, ErrSourceChange) {
		t.Fatalf("want ErrSourceChange, got %v", err)
	}
	if len(mutator.applied) != 0 {
		t.Fatal("an unusable plan must not be applied partially")
	}
}

func TestApplySyncSuccessReturnsNoErrorAndKeepsUnchangedBindings(t *testing.T) {
	plan := bindings("claude", "codex")
	plan.Bindings[0].Action = ActionUnchanged
	outcomes, err := (Coordinator{Mutator: &scriptedMutator{}}).ApplySync(context.Background(), plan, true)
	if err != nil || fmt.Sprint(statuses(outcomes)) != fmt.Sprint([]BindingStatus{BindingUnchanged, BindingApplied}) {
		t.Fatalf("%v %v", statuses(outcomes), err)
	}
	if got := Results(outcomes); len(got) != 1 {
		t.Fatalf("Results must list only bindings that produced a result: %v", got)
	}
}

func TestSyncErrorUnwrapListsEachCauseOnce(t *testing.T) {
	stop := errors.New("stop cause")
	other := errors.New("other cause")
	err := &SyncError{Stopped: stop, stoppedAt: 0, Outcomes: []BindingOutcome{
		{Status: BindingFailed, Err: stop}, {Status: BindingFailed, Err: other}, {Status: BindingNotAttempted, Err: stop},
	}}
	if causes := err.Unwrap(); len(causes) != 2 {
		t.Fatalf("causes %v", causes)
	}
	if !errors.Is(err, stop) || !errors.Is(err, other) {
		t.Fatal("every cause must stay reachable")
	}
}
