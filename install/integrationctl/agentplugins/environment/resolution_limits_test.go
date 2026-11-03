package environment

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func resolutionFixture(shape string, count int) (Requirements, *conflictGraphResolver) {
	roots := map[string]string{}
	resolver := &conflictGraphResolver{packages: map[string][]ResolvedPackage{}}
	for i := range count {
		alias := fmt.Sprintf("node%04d", i)
		if i == 0 || shape == "independent" {
			roots[alias] = "^1.0"
		}
		dependencies := map[string]string{}
		switch shape {
		case "chain", "cycle":
			if i+1 < count || shape == "cycle" {
				dependencies[fmt.Sprintf("acme/node%04d", (i+1)%count)] = "^1.0"
			}
		case "fanout", "dense":
			if i == 0 || shape == "dense" {
				for j := i + 1; j < count; j++ {
					dependencies[fmt.Sprintf("acme/node%04d", j)] = "^1.0"
				}
			}
		}
		resolver.packages["acme/"+alias] = []ResolvedPackage{conflictPackage(alias, "1.0.0", uint64(i+1), dependencies)}
	}
	return conflictRequirements(roots), resolver
}

func TestResolutionBudgetSupportedGraphs(t *testing.T) {
	for _, tc := range []struct {
		shape string
		nodes int
	}{{"independent", 1024}, {"chain", 256}, {"fanout", 65}, {"dense", 64}, {"cycle", 64}} {
		t.Run(tc.shape, func(t *testing.T) {
			requirements, resolver := resolutionFixture(tc.shape, tc.nodes)
			solver := newGraphSolver(resolver, requirements, Lock{}, nil)
			locked, err := solver.resolve(t.Context())
			if err != nil || len(locked.Packages) != tc.nodes {
				t.Fatalf("supported graph: nodes=%d error=%v", len(locked.Packages), err)
			}
			t.Logf("nodes=%d states=%d work=%d", tc.nodes, solver.budget.states, solver.budget.work)
			if tc.shape == "cycle" || tc.shape == "dense" {
				replay := newGraphSolver(resolver, requirements, locked, nil)
				repeated, err := replay.resolve(t.Context())
				if err != nil || !reflect.DeepEqual(locked, repeated) {
					t.Fatalf("shared/cyclic replay changed its exact graph: %v", err)
				}
			}
		})
	}
}

func TestResolutionBudgetStructuralAdmission(t *testing.T) {
	limits := defaultResolutionLimits()
	for _, shape := range []string{"independent", "chain", "dense"} {
		t.Run(shape, func(t *testing.T) {
			count := 5
			if shape == "dense" {
				count = 6
			}
			requirements, resolver := resolutionFixture(shape, count)
			solver := newGraphSolver(resolver, requirements, Lock{}, nil)
			solver.budget.limits.nodes = 4
			if shape == "dense" {
				solver.budget.limits.nodes = 10
				solver.budget.limits.demands = 7
			}
			locked, err := solver.resolve(t.Context())
			if !errors.Is(err, ErrResolutionLimit) || errors.Is(err, ErrDependencyConflict) || locked.LockVersion != 0 {
				t.Fatalf("oversized graph returned a partial lock or nonterminal error: %+v %v", locked, err)
			}
			if shape == "independent" && resolver.candidates != 0 {
				t.Fatal("oversized root graph invoked source callbacks")
			}
		})
	}
	for _, lock := range []Lock{{Packages: make([]LockedPackage, limits.nodes+1)}, {Packages: []LockedPackage{{RequiredBy: make([]RequiredBy, limits.demands+1)}}}} {
		if err := validateGraphLimits(lock); !errors.Is(err, ErrResolutionLimit) {
			t.Fatalf("oversized recorded graph not rejected before traversal: %v", err)
		}
	}
	atLimit := Lock{Packages: make([]LockedPackage, limits.nodes)}
	atLimit.Packages[0].RequiredBy = make([]RequiredBy, limits.demands)
	if err := validateGraphLimits(atLimit); err != nil {
		t.Fatalf("exact structural boundary was rejected: %v", err)
	}
}

func TestResolutionBudgetCountsRejectedCandidatesBeforeSorting(t *testing.T) {
	requirements := conflictRequirements(map[string]string{"a": "^1.0"})
	resolver := &budgetCandidateResolver{values: []Candidate{{Version: "3.0.0", ReleaseSequence: 3}, {Version: "1.0.0", ReleaseSequence: 1}, {Version: "2.0.0", ReleaseSequence: 2}}}
	before := append([]Candidate{}, resolver.values...)
	solver := newGraphSolver(resolver, requirements, Lock{}, nil)
	solver.budget.limits.work = 4 // Expansion + search leave fewer than three candidates.
	_, err := solver.resolve(t.Context())
	if !errors.Is(err, ErrResolutionLimit) || resolver.acquired != 0 || !reflect.DeepEqual(resolver.values, before) {
		t.Fatalf("candidate admission sorted/acquired before checking work: %v %+v", err, resolver)
	}
}

type budgetCandidateResolver struct {
	GraphResolver
	values   []Candidate
	acquired int
	cancel   context.CancelFunc
}

func (resolver *budgetCandidateResolver) Candidates(context.Context, Requirement, []string) ([]Candidate, error) {
	if resolver.cancel != nil {
		resolver.cancel()
	}
	return resolver.values, nil
}

func (resolver *budgetCandidateResolver) AcquireCandidate(context.Context, Requirement, Candidate, []string) (ResolvedPackage, error) {
	resolver.acquired++
	return ResolvedPackage{}, errors.New("unexpected candidate acquisition")
}

func TestResolutionBudgetSurvivesBacktracking(t *testing.T) {
	requirements := conflictRequirements(map[string]string{"a": "^1.0", "z": "1.0.0"})
	resolver := &conflictGraphResolver{packages: map[string][]ResolvedPackage{
		"acme/a": {
			conflictPackage("a", "1.2.0", 3, map[string]string{"acme/z": "2.0.0"}),
			conflictPackage("a", "1.1.0", 2, map[string]string{"acme/z": "2.0.0"}),
			conflictPackage("a", "1.0.0", 1, nil),
		},
		"acme/z": {conflictPackage("z", "1.0.0", 1, nil)},
	}}
	solver := newGraphSolver(resolver, requirements, Lock{}, nil)
	solver.budget.limits.states = 2
	_, err := solver.resolve(t.Context())
	if !errors.Is(err, ErrResolutionLimit) || errors.Is(err, ErrDependencyConflict) || resolver.acquisitions != 2 || solver.budget.states != 2 {
		t.Fatalf("backtracking reset or swallowed its request budget: err=%v acquisitions=%d budget=%+v", err, resolver.acquisitions, solver.budget)
	}
}

// This context cancels at a deterministic observation point during expansion,
// without sleeps, helper goroutines or global test hooks.
type expansionCancellation struct {
	context.Context
	remaining int
}

func (ctx *expansionCancellation) Err() error {
	ctx.remaining--
	if ctx.remaining <= 0 {
		return context.Canceled
	}
	return nil
}

func TestResolutionCancellationInsideExpansion(t *testing.T) {
	requirements, resolver := resolutionFixture("fanout", 65)
	solver := newGraphSolver(resolver, requirements, Lock{}, nil)
	selected := resolver.packages["acme/node0000"][0]
	ctx := &expansionCancellation{Context: context.Background(), remaining: 5}
	_, err := solver.expand(ctx, map[string]ResolvedPackage{selected.Package.SourceKey: selected})
	if !errors.Is(err, context.Canceled) || solver.budget.work >= 65 || resolver.candidates != 0 {
		t.Fatalf("cancellation did not interrupt expansion: err=%v work=%d callbacks=%d", err, solver.budget.work, resolver.candidates)
	}
}

func TestResolutionCancellationStopsSourceCallbacks(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	resolver := &budgetCandidateResolver{values: []Candidate{{Version: "1.0.0", ReleaseSequence: 1}}, cancel: cancel}
	solver := newGraphSolver(resolver, conflictRequirements(map[string]string{"a": "^1.0"}), Lock{}, nil)
	locked, err := solver.resolve(ctx)
	if !errors.Is(err, context.Canceled) || resolver.acquired != 0 || locked.LockVersion != 0 {
		t.Fatalf("source callback continued after cancellation: err=%v acquisitions=%d lock=%+v", err, resolver.acquired, locked)
	}
}
