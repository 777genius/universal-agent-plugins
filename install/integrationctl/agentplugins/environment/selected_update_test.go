package environment

import (
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
	// Preserve the established minimum-compatible graph policy: a newly
	// published lower version is selected only for explicitly refreshed roots.
	before := resolver.candidates
	updated, err := coordinator.ResolveSelected(t.Context(), requirements, original, []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b"} {
		pkg, _ := updated.Package(name)
		if pkg.Version != "1.0.0" {
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
