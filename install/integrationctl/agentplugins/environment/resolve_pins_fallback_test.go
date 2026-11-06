package environment

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
)

func TestPinnedContractionPreflightsEveryRoot(t *testing.T) {
	for _, change := range []string{"new source", "changed selector", "expanded targets", "selected update", "unsorted targets"} {
		t.Run(change, func(t *testing.T) {
			requirements, previous, resolver := contractionFixture(t, "independent", 1025)
			updates := map[string]bool{}
			switch change {
			case "new source":
				previous.Packages = previous.Packages[:1024]
			case "changed selector":
				requirements.Dependencies["node1024"] = conflictRequirement("node1024", "^1.0.0")
			case "expanded targets":
				requirements.Targets["claude"] = []string{"node1024"}
			case "selected update":
				updates["node1024"] = true
			case "unsorted targets":
				previous.Packages[1024].Targets = []string{"codex", "claude"}
			}
			solver := newGraphSolver(resolver, requirements, previous, updates)
			choices, nodes, complete, err := solver.tryPinnedContraction(t.Context())
			if err != nil || complete || choices != nil || nodes != nil || len(solver.descriptions) != 0 {
				t.Fatalf("fresh roots retained speculative state: %v %v %+v %+v", complete, err, choices, nodes)
			}
			assertContractionCallbacks(t, resolver, 0)
			_, _, err = solver.search(t.Context(), map[string]ResolvedPackage{})
			if !errors.Is(err, ErrResolutionLimit) {
				t.Fatalf("fresh oversized fallback lost its limit: %v", err)
			}
			assertContractionCallbacks(t, resolver, 0)
		})
	}
}

func TestPinnedContractionDescendantFallbackIsClean(t *testing.T) {
	for _, change := range []string{"new child", "incompatible selector", "target growth"} {
		t.Run(change, func(t *testing.T) {
			requirements, previous, resolver := contractionFixture(t, "chain", 3)
			switch change {
			case "new child":
				previous.Packages = previous.Packages[:2]
			case "incompatible selector":
				resolver.packages["acme/node0001"][0].Dependencies["acme/node0002"] = conflictRequirement("acme/node0002", "^2.0")
				req := resolver.packages["acme/node0001"][0].Dependencies["acme/node0002"]
				req.Registry.Distribution = "acme/node0002"
				resolver.packages["acme/node0001"][0].Dependencies["acme/node0002"] = req
			case "target growth":
				previous.Packages[2].Targets = []string{"claude"}
			}
			solver := newGraphSolver(resolver, requirements, previous, nil)
			roots := map[string]graphNode{}
			for key, node := range solver.roots {
				roots[key] = node
			}
			choices, nodes, complete, err := solver.tryPinnedContraction(t.Context())
			if err != nil || complete || choices != nil || nodes != nil || solver.budget.work != 0 || solver.budget.states != 0 || !reflect.DeepEqual(roots, solver.roots) {
				t.Fatalf("fallback leaked state: complete=%v error=%v budget=%+v", complete, err, solver.budget)
			}
			if len(solver.descriptions) == 0 {
				t.Fatal("valid immutable descriptions were not retained")
			}
			// Fallback must enforce ordinary admission even with cached facts.
			solver.budget.limits.nodes = 1
			_, _, err = solver.search(t.Context(), map[string]ResolvedPackage{})
			if !errors.Is(err, ErrResolutionLimit) || resolver.candidates != 0 || solver.budget.states == 0 || solver.budget.work == 0 {
				t.Fatalf("fresh fallback bypassed budget: %v %+v", err, solver.budget)
			}
		})
	}
}

func TestPinnedContractionUnsortedTargetsPreserveOperationalEligibility(t *testing.T) {
	for _, targets := range [][]string{{"codex", "claude"}, {"claude", "codex"}} {
		t.Run(targets[0], func(t *testing.T) {
			requirements, previous, resolver := contractionFixture(t, "independent", 1)
			previous.Packages[0].Targets = targets
			if err := previous.ValidateCompatiblePins(requirements); err != nil {
				t.Fatal(err)
			}
			operational := errors.New("eligibility operational failure")
			resolver.eligibilityError = operational
			solver := newGraphSolver(resolver, requirements, previous, nil)
			_, _, complete, err := solver.tryPinnedContraction(t.Context())
			if err != nil || complete != slices.IsSorted(targets) {
				t.Fatalf("changed historical eligibility exemption: %v %v", complete, err)
			}
			if !complete {
				_, _, err := solver.search(t.Context(), map[string]ResolvedPackage{})
				if !errors.Is(err, operational) || resolver.eligible != 1 {
					t.Fatalf("operational eligibility was lost: %v callbacks=%d", err, resolver.eligible)
				}
			} else {
				assertContractionCallbacks(t, resolver, 1)
			}
		})
	}
}

func TestPinnedContractionCancellation(t *testing.T) {
	for _, point := range []string{"root preflight", "declaration preparation", "target propagation", "callback"} {
		t.Run(point, func(t *testing.T) {
			requirements, previous, resolver := contractionFixture(t, "fanout", 65)
			ctx := &expansionCancellation{Context: context.Background(), remaining: 4}
			if point == "root preflight" {
				requirements, previous, resolver = contractionFixture(t, "independent", 65)
			}
			if point == "declaration preparation" || point == "target propagation" {
				ctx.remaining = 1000
				resolver.describe = func(callbackCtx context.Context, pin LockedPackage) (ResolvedPackage, error) {
					ctx.remaining = 4
					if point == "target propagation" {
						ctx.remaining += 64
					}
					return resolver.conflictGraphResolver.DescribePinned(callbackCtx, pin)
				}
			}
			if point == "callback" {
				ctx.remaining = 1000
				resolver.describe = func(callbackCtx context.Context, pin LockedPackage) (ResolvedPackage, error) {
					ctx.remaining = 0
					return resolver.conflictGraphResolver.DescribePinned(callbackCtx, pin)
				}
			}
			solver := newGraphSolver(resolver, requirements, previous, nil)
			choices, nodes, complete, err := solver.tryPinnedContraction(ctx)
			if !errors.Is(err, context.Canceled) || complete || choices != nil || nodes != nil || solver.budget.work != 0 || solver.budget.states != 0 {
				t.Fatalf("cancellation published historical state: %v %v", complete, err)
			}
			if point != "target propagation" && len(solver.descriptions) != 0 {
				t.Fatal("canceled description was cached")
			}
			if point == "root preflight" && len(resolver.described) != 0 {
				t.Fatal("root cancellation read package bytes")
			}
		})
	}
}
