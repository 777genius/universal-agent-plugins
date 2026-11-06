package environment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

func sharedGraphFixture(leaves int, mixed bool) (Requirements, *conflictGraphResolver) {
	resolver := &conflictGraphResolver{packages: map[string][]ResolvedPackage{}}
	rootDeps := map[string]string{}
	for i := range leaves {
		alias := fmt.Sprintf("leaf%04d", i)
		resolver.packages["acme/"+alias] = []ResolvedPackage{conflictPackage(alias, "1.2.0", uint64(i+10), nil)}
	}
	for i := range 8 {
		alias := fmt.Sprintf("hub%02d", i)
		rootDeps["acme/"+alias] = "^1.0"
		dependencies := make(map[string]string, leaves)
		for j := range leaves {
			selector := "^1.0"
			if mixed && i%2 == 1 {
				selector = "^1.1"
			}
			dependencies[fmt.Sprintf("acme/leaf%04d", j)] = selector
		}
		resolver.packages["acme/"+alias] = []ResolvedPackage{conflictPackage(alias, "1.0.0", uint64(i+2), dependencies)}
	}
	resolver.packages["acme/root"] = []ResolvedPackage{conflictPackage("root", "1.0.0", 1, rootDeps)}
	return conflictRequirements(map[string]string{"root": "^1.0"}), resolver
}

func faninGraphFixture(parents int) (Requirements, *conflictGraphResolver) {
	resolver := &conflictGraphResolver{packages: map[string][]ResolvedPackage{}}
	dependencies := make(map[string]string, parents)
	for i := range parents {
		alias := fmt.Sprintf("hub%04d", i)
		dependencies["acme/"+alias] = "^1.0"
		resolver.packages["acme/"+alias] = []ResolvedPackage{conflictPackage(alias, "1.0.0", uint64(i+2), map[string]string{"acme/zzshared": "^1.0"})}
	}
	resolver.packages["acme/root"] = []ResolvedPackage{conflictPackage("root", "1.0.0", 1, dependencies)}
	resolver.packages["acme/zzshared"] = []ResolvedPackage{conflictPackage("zzshared", "1.0.0", uint64(parents+2), nil)}
	return conflictRequirements(map[string]string{"root": "^1.0"}), resolver
}

func TestIncrementalSharedGraphAtSupportedScale(t *testing.T) {
	requirements, resolver := sharedGraphFixture(1000, false)
	solver := newGraphSolver(resolver, requirements, Lock{}, nil)
	locked, err := solver.resolve(t.Context())
	if err != nil || len(locked.Packages) != 1009 || solver.active.demands != 8008 {
		t.Fatalf("supported shared graph failed: packages=%d err=%v", len(locked.Packages), err)
	}
	if solver.budget.states != 1010 {
		t.Fatalf("one-candidate graph unexpectedly searched %d states", solver.budget.states)
	}
	for i := range 1000 {
		leaf, ok := locked.Package(fmt.Sprintf("leaf%04d", i))
		if !ok || len(leaf.RequiredBy) != 8 {
			t.Fatalf("shared leaf lost parent demands: %+v", leaf)
		}
	}
	repeated, err := newGraphSolver(resolver, requirements, locked, nil).resolve(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(locked)
	if err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(repeated)
	if err != nil || string(before) != string(after) {
		t.Fatalf("shared graph replay changed deterministic lock bytes: %v", err)
	}
	t.Logf("packages=%d demands=%d states=%d work=%d", len(locked.Packages), solver.active.demands, solver.budget.states, solver.budget.work)
}

func TestIncrementalDemandBoundaryExcludesDirectRoots(t *testing.T) {
	for _, extra := range []int{0, 64, 65} {
		t.Run(fmt.Sprint(8128+extra), func(t *testing.T) {
			requirements, resolver := resolutionFixture("dense", 128)
			for i := range 128 {
				alias := fmt.Sprintf("node%04d", i)
				requirements.Dependencies[alias] = conflictRequirement(alias, "^1.0")
				if i > 0 {
					requirements.Targets["codex"] = append(requirements.Targets["codex"], alias)
				}
			}
			for i := range extra {
				alias := fmt.Sprintf("acme/node%04d", i)
				resolver.packages["acme/node0127"][0].Dependencies[alias] = fixtureRequirements(map[string]string{alias: "^1.0"})[alias]
			}
			solver := newGraphSolver(resolver, requirements, Lock{}, nil)
			locked, err := solver.resolve(t.Context())
			if extra == 65 {
				if !errors.Is(err, ErrResolutionLimit) || errors.Is(err, ErrDependencyConflict) || locked.LockVersion != 0 {
					t.Fatalf("one excess edge was not terminal: %v", err)
				}
				return
			}
			if err != nil || len(locked.Packages) != 128 || solver.active.demands != 8128+extra {
				t.Fatalf("valid root-plus-edge count refused: packages=%d demands=%d err=%v", len(locked.Packages), solver.active.demands, err)
			}
		})
	}
}

func TestIncrementalSupportedShapes(t *testing.T) {
	for _, shape := range []string{"chain", "fanout", "cycle", "fanin", "mixed"} {
		t.Run(shape, func(t *testing.T) {
			requirements, resolver := resolutionFixture(shape, 1024)
			count := 1024
			switch shape {
			case "fanin":
				requirements, resolver = faninGraphFixture(1000)
				count = 1002
			case "mixed":
				requirements, resolver = sharedGraphFixture(128, true)
				count = 137
			}
			solver := newGraphSolver(resolver, requirements, Lock{}, nil)
			locked, err := solver.resolve(t.Context())
			if err != nil || len(locked.Packages) != count || solver.budget.states != count+1 {
				t.Fatalf("supported %s graph: packages=%d states=%d err=%v", shape, len(locked.Packages), solver.budget.states, err)
			}
		})
	}
}

func BenchmarkIncrementalGraphShapes(b *testing.B) {
	for _, shape := range []string{"independent", "chain", "fanout", "cycle", "dense", "shared", "fanin", "mixed"} {
		b.Run(shape, func(b *testing.B) {
			requirements, base, count := incrementalBenchmarkFixture(shape)
			resolver := &graphCallbackCounter{conflictGraphResolver: base}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				resolver.eligibility, resolver.candidates, resolver.acquisitions = 0, 0, 0
				solver := newGraphSolver(resolver, requirements, Lock{}, nil)
				locked, err := solver.resolve(b.Context())
				if err != nil || len(locked.Packages) != count {
					b.Fatalf("graph: packages=%d error=%v", len(locked.Packages), err)
				}
				b.ReportMetric(float64(solver.budget.states), "states/op")
				b.ReportMetric(float64(solver.budget.work), "work/op")
				b.ReportMetric(float64(resolver.eligibility), "eligible/op")
				b.ReportMetric(float64(resolver.candidates), "candidates/op")
				b.ReportMetric(float64(resolver.acquisitions), "acquire/op")
			}
		})
	}
}

type graphCallbackCounter struct {
	*conflictGraphResolver
	eligibility int
}

func (resolver *graphCallbackCounter) Eligible(ctx context.Context, pkg LockedPackage, targets []string) (bool, error) {
	resolver.eligibility++
	return resolver.conflictGraphResolver.Eligible(ctx, pkg, targets)
}

func incrementalBenchmarkFixture(shape string) (Requirements, *conflictGraphResolver, int) {
	count := 1024
	if shape == "dense" {
		count = 128
	}
	var requirements Requirements
	var resolver *conflictGraphResolver
	switch shape {
	case "shared":
		requirements, resolver = sharedGraphFixture(1000, false)
		count = 1009
	case "mixed":
		requirements, resolver = sharedGraphFixture(128, true)
		count = 137
	case "fanin":
		requirements, resolver = faninGraphFixture(1000)
		count = 1002
	default:
		requirements, resolver = resolutionFixture(shape, count)
	}
	return requirements, resolver, count
}
