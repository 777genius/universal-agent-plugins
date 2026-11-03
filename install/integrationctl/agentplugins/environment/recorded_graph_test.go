package environment

import (
	"fmt"
	"strings"
	"testing"
)

// Build a previously recorded agreement directly: validating or reducing it
// must not run candidate search or inherit that operation's resource limits.
func recordedGraphFixture(t *testing.T, shape string, count int) (Requirements, Lock) {
	t.Helper()
	requirements, resolver := resolutionFixture(shape, count)
	digest, err := ManifestDigest(requirements)
	if err != nil {
		t.Fatal(err)
	}
	lock := Lock{LockVersion: LockVersion, ManifestDigest: digest}
	bySource := map[string]int{}
	for _, key := range sortedKeys(resolver.packages) {
		pkg := resolver.packages[key][0].Package
		pkg.Targets = []string{"codex"}
		if root, ok := requirements.Dependencies[pkg.Alias]; ok {
			setRequested(&pkg, root)
		}
		bySource[pkg.SourceKey] = len(lock.Packages)
		lock.Packages = append(lock.Packages, pkg)
	}
	for _, key := range sortedKeys(resolver.packages) {
		selected := resolver.packages[key][0]
		for _, declaration := range sortedKeys(selected.Dependencies) {
			req := selected.Dependencies[declaration]
			source, err := req.SourceKey()
			if err != nil {
				t.Fatal(err)
			}
			child := &lock.Packages[bySource[source]]
			child.RequiredBy = append(child.RequiredBy, RequiredBy{Alias: selected.Package.Alias, Selector: req.RequestedSelector()})
		}
	}
	return requirements, lock
}

func TestRecordedGraphBeyondSearchLimits(t *testing.T) {
	for _, tc := range []struct {
		shape string
		nodes int
	}{{"independent", 1025}, {"dense", 129}, {"chain", 2048}, {"cycle", 1025}} {
		t.Run(fmt.Sprintf("%s-%d", tc.shape, tc.nodes), func(t *testing.T) {
			requirements, lock := recordedGraphFixture(t, tc.shape, tc.nodes)
			if err := lock.Validate(requirements); err != nil {
				t.Fatalf("valid recorded graph rejected: %v", err)
			}
			empty := conflictRequirements(map[string]string{})
			if err := lock.ValidateCompatiblePins(empty); err != nil {
				t.Fatalf("historical pins rejected before reduction: %v", err)
			}
			resolver := &conflictGraphResolver{}
			reduced, err := (Coordinator{Resolver: resolver}).Resolve(t.Context(), empty, lock, "")
			if err != nil || len(reduced.Packages) != 0 || resolver.candidates != 0 || resolver.acquisitions != 0 {
				t.Fatalf("remove all historical dependencies: lock=%+v error=%v callbacks=%+v", reduced, err, resolver)
			}
		})
	}
}

func TestRecordedGraphStillValidatesEdgesAndReachability(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		change     func(*Lock)
	}{
		{"unknown consumer", "unknown required_by consumer", func(lock *Lock) { lock.Packages[1].RequiredBy[0].Alias = "missing" }},
		{"duplicate edge", "duplicate required_by edge", func(lock *Lock) {
			lock.Packages[1].RequiredBy = append(lock.Packages[1].RequiredBy, lock.Packages[1].RequiredBy[0])
		}},
		{"wrong selector", "does not satisfy consumer", func(lock *Lock) { lock.Packages[1].RequiredBy[0].Selector.Value = "^2.0" }},
		{"unreachable tail", "unreachable from current roots", func(lock *Lock) { lock.Packages[1].RequiredBy = nil }},
		{"wrong target", "locked targets changed", func(lock *Lock) { lock.Packages[1].Targets = []string{"claude"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requirements, lock := recordedGraphFixture(t, "chain", 1025)
			tc.change(&lock)
			if err := lock.Validate(requirements); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("invalid recorded graph: got %v; want %q", err, tc.want)
			}
		})
	}
}

func TestRecordedGraphReductionKeepsSelectedPin(t *testing.T) {
	_, previous := recordedGraphFixture(t, "independent", 1025)
	requirements := conflictRequirements(map[string]string{"node0000": "^1.0"})
	pin := previous.Packages[0]
	resolver := &conflictGraphResolver{packages: map[string][]ResolvedPackage{
		"acme/node0000": {{Package: pin}},
	}}
	reduced, err := (Coordinator{Resolver: resolver}).Resolve(t.Context(), requirements, previous, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(reduced.Packages) != 1 || !SamePin(pin, reduced.Packages[0]) || resolver.candidates != 0 || resolver.acquisitions != 0 {
		t.Fatalf("reduction changed the retained pin or searched candidates: %+v callbacks=%+v", reduced, resolver)
	}
}

func TestRecordedCyclePropagatesEveryRootTarget(t *testing.T) {
	requirements, lock := recordedGraphFixture(t, "cycle", 1025)
	requirements.Dependencies["node0512"] = conflictRequirement("node0512", "^1.0")
	requirements.Targets["claude"] = []string{"node0512"}
	var err error
	lock.ManifestDigest, err = ManifestDigest(requirements)
	if err != nil {
		t.Fatal(err)
	}
	for i := range lock.Packages {
		lock.Packages[i].Targets = []string{"claude", "codex"}
	}
	setRequested(&lock.Packages[512], requirements.Dependencies["node0512"])
	if err := lock.Validate(requirements); err != nil {
		t.Fatalf("cycle did not inherit both root targets: %v", err)
	}
	lock.Packages[1024].Targets = []string{"codex"}
	if err := lock.Validate(requirements); err == nil || !strings.Contains(err.Error(), "locked targets changed") {
		t.Fatalf("cycle with a missing target was accepted: %v", err)
	}
}
