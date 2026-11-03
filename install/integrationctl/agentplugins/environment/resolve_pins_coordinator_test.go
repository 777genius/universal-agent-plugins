package environment

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func assertContractionMatchesFresh(t *testing.T, requirements Requirements, previous Lock, resolver *contractionResolver) Lock {
	t.Helper()
	solver := newGraphSolver(resolver, requirements, previous, nil)
	choices, nodes, err := solver.search(t.Context(), map[string]ResolvedPackage{})
	if err != nil {
		t.Fatalf("fresh resolution: %v", err)
	}
	expected, err := solver.finalizeLock(choices, nodes)
	if err != nil {
		t.Fatal(err)
	}
	described := slices.Clone(resolver.described)
	candidates, acquisitions := resolver.candidates, resolver.acquisitions
	resolver.described, resolver.candidates, resolver.acquisitions = nil, 0, 0
	actual, err := (Coordinator{Resolver: resolver}).Resolve(t.Context(), requirements, previous, "")
	if err != nil || !reflect.DeepEqual(expected, actual) || !slices.Equal(described, resolver.described) || candidates != resolver.candidates || acquisitions != resolver.acquisitions {
		t.Fatalf("contraction changed fresh result or callbacks: error=%v expected=%+v actual=%+v descriptions=%v/%v candidates=%d/%d acquisitions=%d/%d", err, expected, actual, described, resolver.described, candidates, resolver.candidates, acquisitions, resolver.acquisitions)
	}
	return actual
}

func TestCoordinatorContractionSkipsIncompatibleOldChild(t *testing.T) {
	for _, unknownVersion := range []bool{false, true} {
		t.Run(fmt.Sprint(unknownVersion), func(t *testing.T) {
			requirements, previous, resolver := contractionFixture(t, "chain", 2)
			want := "2.0.0"
			resolver.packages["acme/node0000"][0].Dependencies = fixtureRequirements(map[string]string{"acme/node0001": "^2.0"})
			if unknownVersion {
				want = "1.5.0"
				previous.Packages[1].Version = ""
				previous.Packages[1].RequiredBy = nil
				resolver.packages["acme/node0000"][0].Dependencies = fixtureRequirements(map[string]string{"acme/node0001": "^1.0"})
			}
			resolver.packages["acme/node0001"] = append(resolver.packages["acme/node0001"], conflictPackage("node0001", want, 99, nil))
			resolver.describe = func(ctx context.Context, pin LockedPackage) (ResolvedPackage, error) {
				if pin.Alias == "node0001" {
					if unknownVersion {
						pin.Version = "1.0.0"
						return ResolvedPackage{Package: pin}, nil
					}
					return ResolvedPackage{}, errors.New("obsolete incompatible child bytes are corrupt")
				}
				return resolver.conflictGraphResolver.DescribePinned(ctx, pin)
			}
			actual := assertContractionMatchesFresh(t, requirements, previous, resolver)
			child, _ := actual.Package("node0001")
			if child.Version != want || !slices.Equal(resolver.described, []string{"node0000"}) {
				t.Fatalf("incompatible old child was hydrated or described: %+v %v", child, resolver.described)
			}
		})
	}
}

func TestCoordinatorContractionPreservesLexicalDescriptionFrontier(t *testing.T) {
	requirements := conflictRequirements(map[string]string{"a": "^1.0", "z": "^1.0"})
	base := &conflictGraphResolver{packages: map[string][]ResolvedPackage{
		"acme/a": {conflictPackage("a", "1.0.0", 1, map[string]string{"acme/b": "^1.0"})},
		"acme/b": {conflictPackage("b", "1.0.0", 2, map[string]string{"acme/z": "^1.0"})},
		"acme/z": {conflictPackage("z", "1.0.0", 3, nil)},
	}}
	previous, err := (Coordinator{Resolver: base}).Resolve(t.Context(), requirements, Lock{}, "")
	if err != nil {
		t.Fatal(err)
	}
	base.packages["acme/b"][0].Dependencies = fixtureRequirements(map[string]string{"acme/z": "^1.5"})
	base.packages["acme/z"] = append(base.packages["acme/z"], conflictPackage("z", "1.5.0", 4, nil))
	resolver := &contractionResolver{conflictGraphResolver: base}
	resolver.describe = func(ctx context.Context, pin LockedPackage) (ResolvedPackage, error) {
		if pin.Alias == "z" && pin.Version == "1.0.0" {
			return ResolvedPackage{}, errors.New("obsolete z bytes are corrupt")
		}
		return base.DescribePinned(ctx, pin)
	}
	// Count only the changed-intent operation, not fixture construction.
	base.candidates, base.acquisitions = 0, 0
	actual := assertContractionMatchesFresh(t, requirements, previous, resolver)
	z, _ := actual.Package("z")
	if z.Version != "1.5.0" || !slices.Equal(resolver.described, []string{"a", "b"}) {
		t.Fatalf("later root was described before earlier descendant supplied its demand: %+v %v", z, resolver.described)
	}
}

func TestCoordinatorContractionKeepsHydratedSelectedVersion(t *testing.T) {
	requirements, previous, resolver := contractionFixture(t, "independent", 2)
	root := requirements.Dependencies["node0000"]
	root.Registry.Selector = Selector{Kind: SelectorRelease, Value: "1"}
	requirements.Dependencies["node0000"] = root
	setRequested(&previous.Packages[0], root)
	previous.Packages[0].Version = ""
	resolver.packages["acme/node0001"][0].Dependencies = fixtureRequirements(map[string]string{"acme/node0000": "^1.0"})
	resolver.describe = func(ctx context.Context, pin LockedPackage) (ResolvedPackage, error) {
		selected, err := resolver.conflictGraphResolver.DescribePinned(ctx, pin)
		if err == nil && pin.Version == "" {
			selected.Package.Version = "1.0.0"
		}
		return selected, err
	}
	actual := assertContractionMatchesFresh(t, requirements, previous, resolver)
	if actual.Packages[0].Version != "1.0.0" || len(actual.Packages[0].RequiredBy) != 1 {
		t.Fatalf("later compatible demand lost hydrated selected pin: %+v", actual.Packages[0])
	}
	assertContractionCallbacks(t, resolver, 2)
}

func TestCoordinatorRetainsGraphsBeyondFreshLimits(t *testing.T) {
	for _, tc := range []struct {
		shape string
		count int
	}{{"independent", 1026}, {"dense", 130}, {"fanout", 8194}, {"chain", 2048}, {"cycle", 1025}} {
		t.Run(fmt.Sprintf("%s-%d", tc.shape, tc.count), func(t *testing.T) {
			requirements, previous, resolver := contractionFixture(t, tc.shape, tc.count)
			expected := tc.count
			if tc.shape == "independent" {
				expected--
				delete(requirements.Dependencies, "node1025")
				requirements.Targets["codex"] = slices.DeleteFunc(requirements.Targets["codex"], func(alias string) bool { return alias == "node1025" })
			}
			lock, err := (Coordinator{Resolver: resolver}).Resolve(t.Context(), requirements, previous, "")
			if err != nil || len(lock.Packages) != expected {
				t.Fatalf("public contraction: packages=%d error=%v", len(lock.Packages), err)
			}
			if err := lock.Validate(requirements); err != nil {
				t.Fatal(err)
			}
			for i, pin := range lock.Packages {
				if !SamePin(previous.Packages[i], pin) || !slices.Equal(pin.Targets, []string{"codex"}) {
					t.Fatalf("pin or targets changed for %s", pin.Alias)
				}
				if !reflect.DeepEqual(previous.Packages[i].RequiredBy, pin.RequiredBy) {
					t.Fatalf("expected historical edges differ for %s: %+v", pin.Alias, pin.RequiredBy)
				}
			}
			assertContractionCallbacks(t, resolver, expected)
		})
	}
}

func TestCoordinatorContractionRebuildsRootAndTransitiveIntent(t *testing.T) {
	requirements, previous, resolver := contractionFixture(t, "chain", 3)
	// b used to be direct as well; removing that root retains it under a.
	setRequested(&previous.Packages[1], conflictRequirement("node0001", "^1.0"))
	// The removed c has a stale recorded edge, but a's verified closure ends at b.
	resolver.packages["acme/node0001"][0].Dependencies = nil
	delete(resolver.packages, "acme/node0002")
	root := requirements.Dependencies["node0000"]
	root.Alias = "renamed-root"
	requirements.Dependencies = map[string]Requirement{root.Alias: root}
	requirements.Targets["codex"] = []string{root.Alias}
	lock, err := (Coordinator{Resolver: resolver}).Resolve(t.Context(), requirements, previous, "")
	if err != nil || len(lock.Packages) != 2 {
		t.Fatalf("root-to-transitive contraction: %+v %v", lock, err)
	}
	for i, pin := range lock.Packages {
		if !SamePin(previous.Packages[i], pin) {
			t.Fatalf("immutable ownership changed: %+v", pin)
		}
	}
	if lock.Packages[0].Source.Registry.Requested != root.Registry.Selector ||
		lock.Packages[1].Source.Registry.Requested != (Selector{Kind: SelectorRelease, Value: "2"}) ||
		!reflect.DeepEqual(lock.Packages[1].RequiredBy, []RequiredBy{{Alias: "node0000", Selector: Selector{Kind: SelectorRange, Value: "^1.0"}}}) {
		t.Fatalf("root/transitive selector or edge policy changed: %+v", lock.Packages)
	}
	assertContractionCallbacks(t, resolver, 2)
}

func TestCoordinatorContractionDoesNotRequireOldRootProof(t *testing.T) {
	_, previous, resolver := contractionFixture(t, "chain", 3)
	root := previous.Packages[1].Requirement()
	requirements := Requirements{SchemaVersion: 1, Dependencies: map[string]Requirement{root.Alias: root}, Targets: map[string][]string{"codex": {root.Alias}}}
	delete(resolver.packages, "acme/node0000")
	lock, err := (Coordinator{Resolver: resolver}).Resolve(t.Context(), requirements, previous, "")
	if err != nil || len(lock.Packages) != 2 || lock.Packages[0].Alias != "node0001" || lock.Packages[1].Alias != "node0002" {
		t.Fatalf("promotion of existing pin failed: %+v %v", lock, err)
	}
	if len(lock.Packages[0].RequiredBy) != 0 || lock.Packages[1].RequiredBy[0].Alias != "node0001" {
		t.Fatalf("removed consumer retained: %+v", lock.Packages)
	}
	assertContractionCallbacks(t, resolver, 2)
}

func TestCoordinatorContractionTargetNarrowingAndFreshFallback(t *testing.T) {
	for _, expand := range []bool{false, true} {
		t.Run(fmt.Sprint(expand), func(t *testing.T) {
			requirements, previous, resolver := contractionFixture(t, "independent", 1025)
			if expand {
				requirements.Targets["claude"] = []string{"node1024"}
			} else {
				for i := range previous.Packages {
					previous.Packages[i].Targets = []string{"claude", "codex"}
				}
			}
			lock, err := (Coordinator{Resolver: resolver}).Resolve(t.Context(), requirements, previous, "")
			if expand {
				if !errors.Is(err, ErrResolutionLimit) || lock.LockVersion != 0 {
					t.Fatalf("fresh target expansion bypassed admission: %+v %v", lock, err)
				}
				assertContractionCallbacks(t, resolver, 0)
				return
			}
			if err != nil || len(lock.Packages) != 1025 {
				t.Fatalf("target narrowing failed: %+v %v", lock, err)
			}
			for _, pin := range lock.Packages {
				if !slices.Equal(pin.Targets, []string{"codex"}) {
					t.Fatalf("old target survived contraction: %+v", pin)
				}
			}
			assertContractionCallbacks(t, resolver, 1025)
		})
	}
}
