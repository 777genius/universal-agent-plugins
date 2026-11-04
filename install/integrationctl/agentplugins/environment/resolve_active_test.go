package environment

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"testing"
)

type representativeResolver struct {
	*conflictGraphResolver
	requests []Requirement
}

func (resolver *representativeResolver) Candidates(ctx context.Context, req Requirement, targets []string) ([]Candidate, error) {
	resolver.requests = append(resolver.requests, req)
	return resolver.conflictGraphResolver.Candidates(ctx, req, targets)
}

func (resolver *representativeResolver) Resolve(ctx context.Context, req Requirement, targets []string, old *LockedPackage, update bool) (LockedPackage, error) {
	resolver.requests = append(resolver.requests, req)
	return resolver.conflictGraphResolver.Resolve(ctx, req, targets, old, update)
}

func TestIncrementalRepresentativePreservesRootBreadthFirstOrder(t *testing.T) {
	for _, exact := range []bool{false, true} {
		t.Run(map[bool]string{false: "ranges", true: "competing exact releases"}[exact], func(t *testing.T) {
			requirements := conflictRequirements(map[string]string{"a": "^1.0", "z": "^1.0"})
			resolver := &representativeResolver{conflictGraphResolver: &conflictGraphResolver{packages: map[string][]ResolvedPackage{
				"acme/a":  {conflictPackage("a", "1.0.0", 1, map[string]string{"acme/b": "^1.0"})},
				"acme/b":  {conflictPackage("b", "1.0.0", 2, map[string]string{"acme/zz": "^1.0"})},
				"acme/z":  {conflictPackage("z", "1.0.0", 3, map[string]string{"acme/zz": "^1.5"})},
				"acme/zz": {conflictPackage("zz", "1.5.0", 4, nil)},
			}}}
			want := Selector{Kind: SelectorRange, Value: "^1.5"}
			if exact {
				resolver.packages["acme/b"][0].Dependencies["acme/zz"].Registry.Selector = Selector{Kind: SelectorRelease, Value: "5"}
				want = Selector{Kind: SelectorRelease, Value: "4"}
				resolver.packages["acme/z"][0].Dependencies["acme/zz"].Registry.Selector = want
			}
			locked, err := newGraphSolver(resolver, requirements, Lock{}, nil).resolve(t.Context())
			if exact && !errors.Is(err, ErrDependencyConflict) || !exact && err != nil {
				t.Fatalf("resolution: %v", err)
			}
			if exact && locked.LockVersion != 0 {
				t.Fatal("conflicting exact releases produced a lock")
			}
			last := resolver.requests[len(resolver.requests)-1]
			if last.Registry.Distribution != "acme/zz" || last.Registry.Selector != want {
				t.Fatalf("callback lost BFS precedence: %+v, want %+v", last, want)
			}
		})
	}
}

func rollbackFixture() (Requirements, *conflictGraphResolver) {
	requirements := conflictRequirements(map[string]string{"a": "^1.0", "z": "^1.0"})
	requirements.Targets = map[string][]string{"codex": {"a"}, "claude": {"z"}}
	high := conflictPackage("z", "1.1.0", 9, map[string]string{"acme/b": "^1.0", "acme/u": "^1.0", "acme/zzfail": "^2.0"})
	high.Dependencies["acme/b"].Registry.Selector = Selector{Kind: SelectorRelease, Value: "2"}
	u := conflictPackage("u", "1.0.0", 4, map[string]string{"acme/v": "^1.0"})
	u.Package.Alias = "temporary-owner"
	return requirements, &conflictGraphResolver{packages: map[string][]ResolvedPackage{
		"acme/a":      {conflictPackage("a", "1.0.0", 1, map[string]string{"acme/b": "^1.0"})},
		"acme/b":      {conflictPackage("b", "1.0.0", 2, map[string]string{"acme/c": "^1.0"})},
		"acme/c":      {conflictPackage("c", "1.0.0", 3, map[string]string{"acme/b": "^1.0"})},
		"acme/u":      {u},
		"acme/v":      {conflictPackage("v", "1.0.0", 5, map[string]string{"acme/u": "^1.0", "acme/b": "^1.0"})},
		"acme/z":      {high, conflictPackage("z", "1.0.0", 6, nil)},
		"acme/zzfail": {conflictPackage("zzfail", "1.0.0", 7, nil)},
	}}
}

func TestIncrementalRollbackRestoresSharedCyclesAndIntent(t *testing.T) {
	requirements, resolver := rollbackFixture()
	solver := newGraphSolver(resolver, requirements, Lock{}, nil)
	got, err := solver.resolve(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	_, cleanResolver := rollbackFixture()
	cleanResolver.packages["acme/z"] = cleanResolver.packages["acme/z"][1:]
	clean := newGraphSolver(cleanResolver, requirements, Lock{}, nil)
	want, err := clean.resolve(t.Context())
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("rejected branch changed accepted graph: %v\ngot=%+v\nwant=%+v", err, got, want)
	}
	if solver.active.demands != 3 || len(solver.active.nodes) != 4 || solver.budget.states <= clean.budget.states || solver.budget.work <= clean.budget.work {
		t.Fatalf("rollback retained graph counts or discarded lifetime effort: graph=%+v budget=%+v clean=%+v", solver.active, solver.budget, clean.budget)
	}
	for _, key := range solver.active.keys {
		node := solver.active.nodes[key]
		if len(node.demandIndex) != len(node.demands) || !slices.Equal(node.targets, clean.active.nodes[key].targets) {
			t.Fatalf("rollback retained demand indexes/targets for %s", key)
		}
	}
}

type borrowedGraphResolver struct {
	*conflictGraphResolver
	values []Candidate
	cancel context.CancelFunc
}

func (resolver *borrowedGraphResolver) Candidates(context.Context, Requirement, []string) ([]Candidate, error) {
	return resolver.values, nil
}

func (resolver *borrowedGraphResolver) AcquireCandidate(ctx context.Context, req Requirement, candidate Candidate, targets []string) (ResolvedPackage, error) {
	selected, err := resolver.conflictGraphResolver.AcquireCandidate(ctx, req, candidate, targets)
	if resolver.cancel != nil {
		resolver.cancel()
	}
	return selected, err
}

func TestIncrementalProviderFactsRemainUnchanged(t *testing.T) {
	for _, outcome := range []string{"success", "conflict", "cancellation"} {
		t.Run(outcome, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			resolver := &borrowedGraphResolver{conflictGraphResolver: &conflictGraphResolver{packages: map[string][]ResolvedPackage{}}, values: []Candidate{{Version: "1.0.0", ReleaseSequence: 1}, {Version: "1.1.0", ReleaseSequence: 2}}}
			var dependencies map[string]string
			if outcome == "conflict" {
				dependencies = map[string]string{"acme/b": "^2.0"}
			}
			if outcome == "cancellation" {
				resolver.cancel = cancel
			}
			resolver.packages["acme/a"] = []ResolvedPackage{conflictPackage("a", "1.0.0", 1, dependencies), conflictPackage("a", "1.1.0", 2, dependencies)}
			before, err := json.Marshal(resolver.packages)
			if err != nil {
				t.Fatal(err)
			}
			values := slices.Clone(resolver.values)
			locked, err := newGraphSolver(resolver, conflictRequirements(map[string]string{"a": "^1.0"}), Lock{}, nil).resolve(ctx)
			if outcome == "success" && err != nil || outcome == "conflict" && !errors.Is(err, ErrDependencyConflict) || outcome == "cancellation" && !errors.Is(err, context.Canceled) {
				t.Fatalf("unexpected %s outcome: %v", outcome, err)
			}
			after, marshalErr := json.Marshal(resolver.packages)
			if marshalErr != nil || string(before) != string(after) || !slices.Equal(values, resolver.values) || err != nil && locked.LockVersion != 0 {
				t.Fatalf("provider-owned metadata changed or error published a lock: %v", marshalErr)
			}
		})
	}
}
