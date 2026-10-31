package environment

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestResolveSelectedUpdatesOneGraphAndRetainsUnselectedPins(t *testing.T) {
	requirements := conflictRequirements(map[string]string{"a": "^1.0", "b": "^1.0", "c": "^1.0", "exact": "1.0.0"})
	resolver := &conflictGraphResolver{packages: map[string][]ResolvedPackage{}}
	names := []string{"a", "b", "c", "exact"}
	for i, name := range names {
		version := "1.1.0"
		if name == "exact" {
			version = "1.0.0"
		}
		resolver.packages["acme/"+name] = []ResolvedPackage{conflictPackage(name, version, uint64(i+1), nil)}
	}
	coordinator := Coordinator{Resolver: resolver}
	original, err := coordinator.Resolve(t.Context(), requirements, Lock{}, "")
	if err != nil {
		t.Fatal(err)
	}
	for i, name := range names {
		resolver.packages["acme/"+name] = append(resolver.packages["acme/"+name], conflictPackage(name, "1.0.0", uint64(i+11), nil))
	}
	// Explicit updates choose the latest compatible release. A later publication
	// of a lower version cannot silently downgrade an installed package.
	before := resolver.candidates
	updated, err := coordinator.ResolveSelected(t.Context(), requirements, original, []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b"} {
		pkg, _ := updated.Package(name)
		if pkg.Version != "1.1.0" {
			t.Fatal("selected root did not update", name, pkg)
		}
	}
	for _, name := range []string{"c", "exact"} {
		old, _ := original.Package(name)
		current, _ := updated.Package(name)
		if !reflect.DeepEqual(old, current) {
			t.Fatal("unselected root changed", name)
		}
	}
	if resolver.candidates-before != 2 {
		t.Fatal("bulk selection did not perform exactly two moving-root decisions", resolver.candidates-before)
	}
	if _, err := coordinator.ResolveSelected(t.Context(), requirements, original, []string{"exact"}); err == nil {
		t.Fatal("exact intent admitted as a moving selection")
	}
}

func TestSelectedUpdateLatestBacktrackingAndEligibility(t *testing.T) {
	for _, denyOld := range []bool{false, true} {
		t.Run(fmt.Sprint(denyOld), func(t *testing.T) {
			req := versionSelectionRequirements()
			resolver := &versionSelectionResolver{candidates: map[string][]Candidate{"acme/a": {{Version: "1.2.0", ReleaseSequence: 1}}}, dependencies: map[string]map[string]string{}, deny: map[string]bool{}}
			coordinator := Coordinator{Resolver: resolver}
			old, err := coordinator.Resolve(t.Context(), req, Lock{}, "")
			if err != nil {
				t.Fatal(err)
			}
			resolver.candidates["acme/a"] = append(resolver.candidates["acme/a"], Candidate{Version: "1.4.0", ReleaseSequence: 4}, Candidate{Version: "1.3.0", ReleaseSequence: 3})
			resolver.deny["acme/a@1.4.0/codex"] = true
			next, err := coordinator.ResolveSelected(t.Context(), req, old, []string{"a"})
			if err != nil {
				t.Fatal(err)
			}
			pkg, _ := next.Package("a")
			if pkg.Version != "1.3.0" {
				t.Fatal(pkg)
			}
			resolver.candidates["acme/a"] = []Candidate{{Version: "1.2.0", ReleaseSequence: 1}}
			resolver.deny["acme/a@1.2.0/codex"] = denyOld
			next, err = coordinator.ResolveSelected(t.Context(), req, old, []string{"a"})
			if denyOld {
				if err == nil {
					t.Fatal("ineligible old pin retained", next)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSelectedUpdateRefusesOnlyLowerCandidates(t *testing.T) {
	req := conflictRequirements(map[string]string{"a": "^1.0"})
	resolver := &conflictGraphResolver{packages: map[string][]ResolvedPackage{"acme/a": {conflictPackage("a", "1.2.0", 1, nil)}}}
	coordinator := Coordinator{Resolver: resolver}
	old, err := coordinator.Resolve(t.Context(), req, Lock{}, "")
	if err != nil {
		t.Fatal(err)
	}
	resolver.packages["acme/a"] = []ResolvedPackage{conflictPackage("a", "1.1.0", 2, nil)}
	if next, err := coordinator.ResolveSelected(t.Context(), req, old, []string{"a"}); !errors.Is(err, ErrDependencyConflict) {
		t.Fatal("downgrade did not retain typed conflict", next, err)
	}
}

func TestSelectedRootsResolveSharedClosureTogether(t *testing.T) {
	requirements := conflictRequirements(map[string]string{"a": "^1.0", "b": "^1.0", "untouched": "^1.0"})
	resolver := &conflictGraphResolver{packages: map[string][]ResolvedPackage{
		"acme/a":         {conflictPackage("a", "1.0.0", 1, map[string]string{"acme/c": "^1.0"})},
		"acme/b":         {conflictPackage("b", "1.0.0", 2, map[string]string{"acme/c": "^1.0"})},
		"acme/c":         {conflictPackage("c", "1.0.0", 3, nil)},
		"acme/untouched": {conflictPackage("untouched", "1.0.0", 4, nil)},
	}}
	coordinator := Coordinator{Resolver: resolver}
	old, err := coordinator.Resolve(t.Context(), requirements, Lock{}, "")
	if err != nil {
		t.Fatal(err)
	}
	resolver.packages["acme/a"] = append(resolver.packages["acme/a"], conflictPackage("a", "1.2.0", 12, map[string]string{"acme/c": "^1.2"}))
	resolver.packages["acme/b"] = append(resolver.packages["acme/b"], conflictPackage("b", "1.2.0", 13, map[string]string{"acme/c": "^1.3"}))
	resolver.packages["acme/c"] = append(resolver.packages["acme/c"], conflictPackage("c", "1.2.0", 14, nil), conflictPackage("c", "1.3.0", 15, nil))
	before := resolver.acquisitions
	next, err := coordinator.ResolveSelected(t.Context(), requirements, old, []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	shared, _ := next.Package("c")
	if shared.Version != "1.3.0" || len(shared.RequiredBy) != 2 || len(shared.Targets) != 1 || len(next.Packages) != 4 {
		t.Fatal(next)
	}
	if resolver.acquisitions-before != 3 {
		t.Fatal("selected roots did not share one candidate graph", resolver.acquisitions-before)
	}
	previous, _ := old.Package("untouched")
	retained, _ := next.Package("untouched")
	if !reflect.DeepEqual(previous, retained) {
		t.Fatal("unselected root changed", retained)
	}
}
