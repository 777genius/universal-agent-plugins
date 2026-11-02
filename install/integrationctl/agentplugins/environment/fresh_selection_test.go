package environment

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func TestFreshSelectionHighestStableAndPublication(t *testing.T) {
	for _, tc := range []struct {
		name, expression, want string
		kind                   SelectorKind
		sequence               uint64
		candidates             []Candidate
	}{
		{"numeric range", "^1.2", "1.10.0", SelectorRange, 5, []Candidate{{Version: "1.2.0", ReleaseSequence: 8}, {Version: "1.10.0", ReleaseSequence: 1}, {Version: "1.9.0", ReleaseSequence: 9}, {Version: "1.10.0", ReleaseSequence: 5}, {Version: "2.0.0", ReleaseSequence: 10}, {Version: "1.11.0-beta", ReleaseSequence: 11}}},
		{"exact version", "1.2.3", "1.2.3", SelectorVersion, 7, []Candidate{{Version: "1.2.3", ReleaseSequence: 2}, {Version: "1.2.3", ReleaseSequence: 7}, {Version: "1.2.4", ReleaseSequence: 8}}},
		{"zero minor range", "^0.2.3", "0.2.10", SelectorRange, 3, []Candidate{{Version: "0.2.3", ReleaseSequence: 1}, {Version: "0.3.0", ReleaseSequence: 4}, {Version: "0.2.10", ReleaseSequence: 3}}},
		{"zero patch range", "^0.0.3", "0.0.3", SelectorRange, 2, []Candidate{{Version: "0.0.3", ReleaseSequence: 1}, {Version: "0.0.3", ReleaseSequence: 2}, {Version: "0.0.4", ReleaseSequence: 3}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var baseline []byte
			for _, reverse := range []bool{false, true} {
				requirements := versionSelectionRequirements()
				requirements.Dependencies["a"].Registry.Selector = Selector{Kind: tc.kind, Value: tc.expression}
				candidates := slices.Clone(tc.candidates)
				if reverse {
					slices.Reverse(candidates)
				}
				resolver := &versionSelectionResolver{candidates: map[string][]Candidate{"acme/a": candidates}}
				locked, err := (Coordinator{Resolver: resolver}).Resolve(t.Context(), requirements, Lock{}, "")
				if err != nil {
					t.Fatal(err)
				}
				pkg, ok := locked.Package("a")
				if !ok || pkg.Version != tc.want || pkg.Source.Registry.ReleaseSequence != tc.sequence {
					t.Fatalf("fresh selection: %+v", pkg)
				}
				body, err := json.Marshal(locked)
				if err != nil {
					t.Fatal(err)
				}
				if baseline != nil && string(body) != string(baseline) {
					t.Fatal("candidate enumeration order changed the graph")
				}
				baseline = body
			}
		})
	}
}

func TestFreshSelectionRetainsPublicationAndLockBytes(t *testing.T) {
	for _, selector := range []Selector{{Kind: SelectorRange, Value: "^1.2"}, {Kind: SelectorVersion, Value: "1.2.3"}} {
		t.Run(selector.Value, func(t *testing.T) {
			requirements := versionSelectionRequirements()
			requirements.Dependencies["a"].Registry.Selector = selector
			resolver := &versionSelectionResolver{candidates: map[string][]Candidate{"acme/a": {{Version: "1.2.3", ReleaseSequence: 2}}}}
			coordinator := Coordinator{Resolver: resolver}
			old, err := coordinator.Resolve(t.Context(), requirements, Lock{}, "")
			if err != nil {
				t.Fatal(err)
			}
			before, err := json.Marshal(old)
			if err != nil {
				t.Fatal(err)
			}
			resolver.candidates["acme/a"] = append(resolver.candidates["acme/a"], Candidate{Version: "1.2.3", ReleaseSequence: 7}, Candidate{Version: "1.10.0", ReleaseSequence: 8})
			resolver.enumerated, resolver.acquired = 0, 0
			next, err := coordinator.Resolve(t.Context(), requirements, old, "")
			if err != nil {
				t.Fatal(err)
			}
			after, err := json.Marshal(next)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) || !reflect.DeepEqual(old, next) || resolver.enumerated != 0 || resolver.acquired != 0 {
				t.Fatalf("compatible lock changed: %+v", next)
			}
		})
	}
}

func TestSelectedUpdateUsesHighestForNewNodesAndRetainsSharedPins(t *testing.T) {
	requirements := conflictRequirements(map[string]string{"a": "^1.0", "b": "1.0.0"})
	resolver := &conflictGraphResolver{packages: map[string][]ResolvedPackage{
		"acme/a": {conflictPackage("a", "1.0.0", 1, map[string]string{"acme/c": "^1.0"})},
		"acme/b": {conflictPackage("b", "1.0.0", 1, map[string]string{"acme/c": "1.0.0"})},
		"acme/c": {conflictPackage("c", "1.0.0", 1, nil)},
	}}
	coordinator := Coordinator{Resolver: resolver}
	old, err := coordinator.Resolve(t.Context(), requirements, Lock{}, "")
	if err != nil {
		t.Fatal(err)
	}
	resolver.packages["acme/a"] = append(resolver.packages["acme/a"], conflictPackage("a", "1.1.0", 2, map[string]string{"acme/c": "^1.0", "acme/d": "^1.0"}))
	resolver.packages["acme/c"] = append(resolver.packages["acme/c"], conflictPackage("c", "1.5.0", 2, nil))
	resolver.packages["acme/d"] = []ResolvedPackage{conflictPackage("d", "1.2.0", 1, nil), conflictPackage("d", "1.10.0", 2, nil)}
	next, err := coordinator.ResolveSelected(t.Context(), requirements, old, []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := next.Package("a")
	d, _ := next.Package("d")
	if a.Version != "1.1.0" || d.Version != "1.10.0" || len(next.Packages) != 4 {
		t.Fatalf("selected/new graph: %+v", next)
	}
	for _, alias := range []string{"b", "c"} {
		before, _ := old.Package(alias)
		after, _ := next.Package(alias)
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("retained %s changed: %+v", alias, after)
		}
	}
}

// Exact publication selectors use the resolver's exact path, not enumeration.
type freshExactReleaseResolver struct{ versionSelectionResolver }

func (r *freshExactReleaseResolver) Resolve(ctx context.Context, req Requirement, targets []string, _ *LockedPackage, _ bool) (LockedPackage, error) {
	for _, candidate := range r.candidates[req.Registry.Distribution] {
		if fmt.Sprint(candidate.ReleaseSequence) == req.Registry.Selector.Value {
			selected, err := r.AcquireCandidate(ctx, req, candidate, targets)
			return selected.Package, err
		}
	}
	return LockedPackage{}, fmt.Errorf("requested fixture publication absent")
}

func TestFreshSelectionExactReleaseKeepsItsSequence(t *testing.T) {
	requirements := versionSelectionRequirements()
	requirements.Dependencies["a"].Registry.Selector = Selector{Kind: SelectorRelease, Value: "2"}
	resolver := &freshExactReleaseResolver{versionSelectionResolver: versionSelectionResolver{candidates: map[string][]Candidate{"acme/a": {{Version: "1.2.3", ReleaseSequence: 2}, {Version: "1.2.3", ReleaseSequence: 7}, {Version: "1.10.0", ReleaseSequence: 8}}}}}
	locked, err := (Coordinator{Resolver: resolver}).Resolve(t.Context(), requirements, Lock{}, "")
	if err != nil {
		t.Fatal(err)
	}
	pkg, _ := locked.Package("a")
	if pkg.Version != "1.2.3" || pkg.Source.Registry.ReleaseSequence != 2 || resolver.enumerated != 0 || resolver.acquired != 1 {
		t.Fatalf("exact publication moved: %+v", locked)
	}
}
