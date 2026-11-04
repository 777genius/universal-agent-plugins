package environment

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestCandidatesOwnAdmittedMetadata(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "cancellation"}[canceled], func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			resolver := &budgetCandidateResolver{values: []Candidate{
				{Version: "1.0.0", ReleaseSequence: 1},
				{Version: "1.1.0", ReleaseSequence: 2},
				{Version: "1.1.0", ReleaseSequence: 3},
			}}
			before := append([]Candidate(nil), resolver.values...)
			if canceled {
				resolver.cancel = cancel
			}
			solver := newGraphSolver(resolver, conflictRequirements(map[string]string{"a": "^1.0"}), Lock{}, nil)
			got, err := solver.candidates(ctx, solver.roots["directory:acme/a"])
			if canceled {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("candidate cancellation: %v", err)
				}
			} else if err != nil || !reflect.DeepEqual(got, []Candidate{before[2], before[1], before[0]}) {
				t.Fatalf("candidate ordering: %v %v", got, err)
			}
			if !reflect.DeepEqual(resolver.values, before) {
				t.Fatalf("provider metadata changed: %v", resolver.values)
			}
		})
	}
}
