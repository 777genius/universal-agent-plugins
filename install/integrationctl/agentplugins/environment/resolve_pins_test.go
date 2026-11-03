package environment

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

type contractionResolver struct {
	*conflictGraphResolver
	described        []string
	exact, eligible  int
	describe         func(context.Context, LockedPackage) (ResolvedPackage, error)
	eligibilityError error
}

func (resolver *contractionResolver) DescribePinned(ctx context.Context, pin LockedPackage) (ResolvedPackage, error) {
	resolver.described = append(resolver.described, pin.Alias)
	if resolver.describe != nil {
		return resolver.describe(ctx, pin)
	}
	return resolver.conflictGraphResolver.DescribePinned(ctx, pin)
}

func (resolver *contractionResolver) Resolve(ctx context.Context, req Requirement, targets []string, pin *LockedPackage, update bool) (LockedPackage, error) {
	resolver.exact++
	return resolver.conflictGraphResolver.Resolve(ctx, req, targets, pin, update)
}

func (resolver *contractionResolver) Eligible(context.Context, LockedPackage, []string) (bool, error) {
	resolver.eligible++
	return resolver.eligibilityError == nil, resolver.eligibilityError
}

func contractionFixture(t *testing.T, shape string, count int) (Requirements, Lock, *contractionResolver) {
	t.Helper()
	requirements, previous := recordedGraphFixture(t, shape, count)
	_, resolver := resolutionFixture(shape, count)
	return requirements, previous, &contractionResolver{conflictGraphResolver: resolver}
}

func assertContractionCallbacks(t *testing.T, resolver *contractionResolver, expected int) {
	t.Helper()
	if len(resolver.described) != expected || resolver.candidates != 0 || resolver.acquisitions != 0 || resolver.exact != 0 || resolver.eligible != 0 {
		t.Fatalf("unexpected callbacks: described=%d candidates=%d acquisitions=%d exact=%d eligible=%d", len(resolver.described), resolver.candidates, resolver.acquisitions, resolver.exact, resolver.eligible)
	}
	seen := map[string]bool{}
	for _, alias := range resolver.described {
		if seen[alias] {
			t.Fatalf("described %s more than once", alias)
		}
		seen[alias] = true
	}
}

func TestPinnedContractionBypassesAllFreshLimits(t *testing.T) {
	for _, tc := range []struct {
		shape                  string
		count, retained, edges int
	}{
		{"independent", 1026, 1025, 0},
		{"chain", 2048, 2048, 2047},
		{"cycle", 1025, 1025, 1025},
		{"dense", 130, 130, 8385},
		{"fanout", 8194, 8194, 8193},
	} {
		t.Run(fmt.Sprintf("%s-%d", tc.shape, tc.count), func(t *testing.T) {
			requirements, previous, resolver := contractionFixture(t, tc.shape, tc.count)
			if tc.shape == "independent" {
				delete(requirements.Dependencies, "node1025")
				requirements.Targets["codex"] = slices.DeleteFunc(requirements.Targets["codex"], func(alias string) bool { return alias == "node1025" })
			}
			if err := previous.ValidateCompatiblePins(requirements); err != nil {
				t.Fatal(err)
			}
			solver := newGraphSolver(resolver, requirements, previous, nil)
			solver.budget.limits = resolutionLimits{}
			choices, nodes, complete, err := solver.tryPinnedContraction(t.Context())
			if err != nil || !complete || len(nodes) != tc.retained || len(choices) != tc.retained {
				t.Fatalf("historical closure: complete=%v nodes=%d choices=%d error=%v", complete, len(nodes), len(choices), err)
			}
			edges := 0
			for key, node := range nodes {
				if !SamePin(solver.pins[key], choices[key].Package) || node.requirement.Alias != solver.pins[key].Alias || !slices.Equal(node.targets, []string{"codex"}) {
					t.Fatalf("historical pin, alias or targets changed for %s", key)
				}
				for _, demand := range node.demands {
					if demand.parent != "" {
						edges++
					}
				}
			}
			if edges != tc.edges || solver.budget.states != 0 || solver.budget.work != 0 {
				t.Fatalf("edges=%d want=%d budget=%+v", edges, tc.edges, solver.budget)
			}
			assertContractionCallbacks(t, resolver, tc.retained)
		})
	}
}

func TestPinnedContractionTargetOnlyAboveNodeLimit(t *testing.T) {
	requirements, previous, resolver := contractionFixture(t, "independent", 1025)
	for i := range previous.Packages {
		previous.Packages[i].Targets = []string{"claude", "codex"}
	}
	solver := newGraphSolver(resolver, requirements, previous, nil)
	choices, nodes, complete, err := solver.tryPinnedContraction(t.Context())
	if err != nil || !complete || len(choices) != 1025 {
		t.Fatalf("target contraction: %v %v %d", complete, err, len(choices))
	}
	for _, node := range nodes {
		if !slices.Equal(node.targets, []string{"codex"}) {
			t.Fatalf("removed target retained: %v", node.targets)
		}
	}
	assertContractionCallbacks(t, resolver, 1025)
}

func TestPinnedContractionCyclesPropagateSharedTargets(t *testing.T) {
	requirements, previous, resolver := contractionFixture(t, "cycle", 1025)
	requirements.Dependencies["node0512"] = conflictRequirement("node0512", "^1.0")
	requirements.Targets["claude"] = []string{"node0512"}
	for i := range previous.Packages {
		previous.Packages[i].Targets = []string{"claude", "codex"}
	}
	setRequested(&previous.Packages[512], requirements.Dependencies["node0512"])
	solver := newGraphSolver(resolver, requirements, previous, nil)
	_, nodes, complete, err := solver.tryPinnedContraction(t.Context())
	if err != nil || !complete || len(nodes) != 1025 {
		t.Fatalf("shared cycle: %v %v %d", complete, err, len(nodes))
	}
	for _, node := range nodes {
		if !slices.Equal(node.targets, []string{"claude", "codex"}) {
			t.Fatalf("cycle lost inherited targets: %v", node.targets)
		}
	}
	assertContractionCallbacks(t, resolver, 1025)
}

func TestPinnedContractionUsesDeclarationsNotRecordedEdges(t *testing.T) {
	requirements, previous, resolver := contractionFixture(t, "chain", 4)
	// Missing recorded a->b and an unnecessary a->d edge are compatible old
	// metadata. Verified declarations retain b and remove the unreachable c<->d.
	previous.Packages[1].RequiredBy = nil
	previous.Packages[2].RequiredBy = []RequiredBy{{Alias: "node0003", Selector: Selector{Kind: SelectorRange, Value: "^1.0"}}}
	previous.Packages[3].RequiredBy = append(previous.Packages[3].RequiredBy, RequiredBy{Alias: "node0000", Selector: Selector{Kind: SelectorRange, Value: "^1.0"}})
	resolver.packages["acme/node0001"][0].Dependencies = nil
	delete(resolver.packages, "acme/node0002")
	delete(resolver.packages, "acme/node0003")
	if err := previous.ValidateCompatiblePins(requirements); err != nil {
		t.Fatal(err)
	}
	solver := newGraphSolver(resolver, requirements, previous, nil)
	_, nodes, complete, err := solver.tryPinnedContraction(t.Context())
	if err != nil || !complete || len(nodes) != 2 {
		t.Fatalf("declaration closure: %v %v %+v", complete, err, nodes)
	}
	child := nodes[previous.Packages[1].SourceKey]
	if len(child.demands) != 1 || child.demands[0].parent != "node0000" {
		t.Fatalf("verified edge missing: %+v", child.demands)
	}
	if !reflect.DeepEqual(resolver.described, []string{"node0000", "node0001"}) {
		t.Fatalf("removed packages were read: %v", resolver.described)
	}
}
