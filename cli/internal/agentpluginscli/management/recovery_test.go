package management

import (
	"testing"
	"time"
)

func TestCanceledPartialEffectsRemainPendingAndNeverAgeOut(t *testing.T) {
	now := time.Now().UTC()
	for _, result := range []*OperationResult{
		{Outcome: "partial", Mutated: true},
		{Outcome: "unchanged", NextActions: []NextAction{{Kind: "recover", Reason: "cleanup required"}}},
	} {
		op := Operation{State: Canceled, Result: result, UpdatedAt: now.Add(-90 * 24 * time.Hour)}
		if !pendingOperation(op) || prunable(op, now) {
			t.Fatal("canceled unresolved work released fence")
		}
		op.ResolvedBy = "reviewed-recovery"
		if pendingOperation(op) || !prunable(op, now) {
			t.Fatal("resolved work did not become eligible for pruning")
		}
	}
}

func TestNilResultCannotClaimKnownNoEffects(t *testing.T) {
	op := Operation{State: Queued}
	for _, outcome := range []Outcome{{State: Succeeded}, {State: Failed}, {State: Canceled}, {State: Running}} {
		result := finishOperation(op, outcome, nil, false, false)
		if result.State != Interrupted || result.Result != nil {
			t.Fatal("missing result treated as known", result)
		}
	}
}
