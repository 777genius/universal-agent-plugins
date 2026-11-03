package environment

import (
	"context"
	"errors"
	"testing"
)

type callbackCancellationResolver struct {
	*conflictGraphResolver
	stage  string
	cancel context.CancelFunc
	after  bool
	late   int
}

func (resolver *callbackCancellationResolver) called(stage string) {
	if resolver.after {
		resolver.late++
	}
	if resolver.stage == stage {
		resolver.cancel()
		resolver.after = true
	}
}

func (resolver *callbackCancellationResolver) Candidates(ctx context.Context, req Requirement, targets []string) ([]Candidate, error) {
	values, err := resolver.conflictGraphResolver.Candidates(ctx, req, targets)
	resolver.called("candidates")
	return values, err
}

func (resolver *callbackCancellationResolver) AcquireCandidate(ctx context.Context, req Requirement, candidate Candidate, targets []string) (ResolvedPackage, error) {
	selected, err := resolver.conflictGraphResolver.AcquireCandidate(ctx, req, candidate, targets)
	resolver.called("acquire")
	return selected, err
}

func (resolver *callbackCancellationResolver) Resolve(ctx context.Context, req Requirement, targets []string, old *LockedPackage, update bool) (LockedPackage, error) {
	selected, err := resolver.conflictGraphResolver.Resolve(ctx, req, targets, old, update)
	resolver.called("resolve")
	return selected, err
}

func (resolver *callbackCancellationResolver) DescribePinned(ctx context.Context, pkg LockedPackage) (ResolvedPackage, error) {
	selected, err := resolver.conflictGraphResolver.DescribePinned(ctx, pkg)
	resolver.called("describe")
	return selected, err
}

func (resolver *callbackCancellationResolver) Eligible(ctx context.Context, pkg LockedPackage, targets []string) (bool, error) {
	allowed, err := resolver.conflictGraphResolver.Eligible(ctx, pkg, targets)
	resolver.called("eligible")
	return allowed, err
}

func TestIncrementalCancellationAfterEverySourceCallback(t *testing.T) {
	for _, stage := range []string{"candidates", "acquire", "resolve", "describe", "eligible"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			requirements, base := resolutionFixture("chain", 3)
			if stage == "resolve" || stage == "describe" {
				requirements.Dependencies["node0000"].Registry.Selector = Selector{Kind: SelectorRelease, Value: "1"}
			}
			resolver := &callbackCancellationResolver{conflictGraphResolver: base, stage: stage, cancel: cancel}
			locked, err := newGraphSolver(resolver, requirements, Lock{}, nil).resolve(ctx)
			if !errors.Is(err, context.Canceled) || locked.LockVersion != 0 || resolver.late != 0 || !resolver.after {
				t.Fatalf("callback cancellation lost: err=%v late=%d reached=%v lock=%+v", err, resolver.late, resolver.after, locked)
			}
		})
	}
}

func TestIncrementalCancellationDuringPreparationAndTargetPropagation(t *testing.T) {
	requirements, resolver := resolutionFixture("cycle", 32)
	solver := newGraphSolver(resolver, requirements, Lock{}, nil)
	choices := make(map[string]ResolvedPackage, len(resolver.packages))
	for _, packages := range resolver.packages {
		choices[packages[0].Package.SourceKey] = packages[0]
	}
	work, err := solver.newActiveGraph(t.Context(), choices)
	if err != nil {
		t.Fatal(err)
	}
	if err := solver.propagate(t.Context(), choices, work); err != nil {
		t.Fatal(err)
	}
	root := requirements.Dependencies["node0000"]
	parent := graphNode{requirement: Requirement{Alias: "late-parent"}, targets: []string{"claude"}}
	if err := work.merge(parent, "directory:acme/node0000", root, solver.budget.limits); err != nil {
		t.Fatal(err)
	}
	ctx := &expansionCancellation{Context: context.Background(), remaining: 5}
	if err := solver.propagate(ctx, choices, work); !errors.Is(err, context.Canceled) {
		t.Fatalf("target propagation ignored cancellation: %v", err)
	}
	_, fanout := resolutionFixture("fanout", 32)
	ctx = &expansionCancellation{Context: context.Background(), remaining: 5}
	if _, err := solver.prepareDependencies(ctx, fanout.packages["acme/node0000"][0].Dependencies); !errors.Is(err, context.Canceled) {
		t.Fatalf("declaration preparation ignored cancellation: %v", err)
	}
}

func TestIncrementalCancellationDuringRepresentativeTraversal(t *testing.T) {
	requirements, resolver := sharedGraphFixture(32, true)
	solver := newGraphSolver(resolver, requirements, Lock{}, nil)
	if _, err := solver.resolve(t.Context()); err != nil {
		t.Fatal(err)
	}
	ctx := &expansionCancellation{Context: context.Background(), remaining: 5}
	node := solver.active.nodes["directory:acme/leaf0031"]
	if _, err := solver.representativeRequirement(ctx, "directory:acme/leaf0031", node); !errors.Is(err, context.Canceled) {
		t.Fatalf("ambiguous representative traversal ignored cancellation: %v", err)
	}
}
