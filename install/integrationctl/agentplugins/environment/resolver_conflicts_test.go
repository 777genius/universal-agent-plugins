package environment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

type conflictGraphResolver struct {
	packages                 map[string][]ResolvedPackage
	candidates, acquisitions int
	failure                  error
	candidateFailures        map[uint64]error
}

func conflictRequirement(alias, expression string) Requirement {
	kind := SelectorVersion
	if strings.HasPrefix(expression, "^") {
		kind = SelectorRange
	}
	return Requirement{Alias: alias, Registry: &RegistrySource{Distribution: "acme/" + alias, Selector: Selector{Kind: kind, Value: expression}}}
}
func conflictRequirements(roots map[string]string) Requirements {
	req := Requirements{SchemaVersion: 1, Dependencies: map[string]Requirement{}, Targets: map[string][]string{"codex": {}}}
	for alias, expression := range roots {
		req.Dependencies[alias] = conflictRequirement(alias, expression)
		req.Targets["codex"] = append(req.Targets["codex"], alias)
	}
	return req
}
func conflictPackage(alias, version string, sequence uint64, deps map[string]string) ResolvedPackage {
	digest := fmt.Sprintf("sha256:%064x", sequence)
	return ResolvedPackage{Package: LockedPackage{Alias: alias, Version: version, SourceKey: "directory:acme/" + alias, DigestAlgorithm: TreeDigestAlgorithm, TreeDigest: digest, ManifestDigest: digest,
		Source: LockedSource{Registry: &LockedRegistrySource{Distribution: "acme/" + alias, DistributionID: "acme/" + alias, ProductID: alias, ReleaseSequence: sequence, Requested: Selector{Kind: SelectorRelease, Value: fmt.Sprint(sequence)}, Repository: "acme/source", Commit: fmt.Sprintf("%040x", sequence)}}}, Dependencies: fixtureRequirements(deps)}
}
func (resolver *conflictGraphResolver) Resolve(_ context.Context, requirement Requirement, _ []string, _ *LockedPackage, _ bool) (LockedPackage, error) {
	for _, pkg := range resolver.packages[requirement.Registry.Distribution] {
		if registrySelectorMatches(requirement.Registry.Selector, pkg.Package) {
			result := pkg.Package
			setRequested(&result, requirement)
			return result, nil
		}
	}
	return LockedPackage{}, errors.New("fixture exact release missing")
}
func (resolver *conflictGraphResolver) Candidates(_ context.Context, requirement Requirement, _ []string) ([]Candidate, error) {
	resolver.candidates++
	out := make([]Candidate, 0, len(resolver.packages[requirement.Registry.Distribution]))
	for _, pkg := range resolver.packages[requirement.Registry.Distribution] {
		out = append(out, Candidate{Version: pkg.Package.Version, ReleaseSequence: pkg.Package.Source.Registry.ReleaseSequence})
	}
	return out, nil
}
func (resolver *conflictGraphResolver) AcquireCandidate(_ context.Context, requirement Requirement, candidate Candidate, _ []string) (ResolvedPackage, error) {
	resolver.acquisitions++
	if resolver.failure != nil {
		return ResolvedPackage{}, resolver.failure
	}
	if err := resolver.candidateFailures[candidate.ReleaseSequence]; err != nil {
		return ResolvedPackage{}, err
	}
	for _, pkg := range resolver.packages[requirement.Registry.Distribution] {
		if pkg.Package.Source.Registry.ReleaseSequence == candidate.ReleaseSequence {
			setRequested(&pkg.Package, requirement)
			return pkg, nil
		}
	}
	return ResolvedPackage{}, errors.New("fixture candidate missing")
}
func (resolver *conflictGraphResolver) DescribePinned(_ context.Context, pin LockedPackage) (ResolvedPackage, error) {
	for _, pkg := range resolver.packages[pin.Source.Registry.Distribution] {
		if pkg.Package.TreeDigest == pin.TreeDigest {
			pkg.Package = pin
			return pkg, nil
		}
	}
	return ResolvedPackage{}, errors.New("fixture pin missing")
}
func (*conflictGraphResolver) Eligible(context.Context, LockedPackage, []string) (bool, error) {
	return true, nil
}

type conflictMutator struct{ calls int }

func (mutator *conflictMutator) Prepare(context.Context, PreparedBinding) (*installer.PreparedOperation, error) {
	mutator.calls++
	return nil, errors.New("unexpected mutation")
}
func (mutator *conflictMutator) Apply(context.Context, *installer.PreparedOperation) (installer.Result, error) {
	mutator.calls++
	return installer.Result{}, errors.New("unexpected mutation")
}

func TestGraphConflictingRequirements(t *testing.T) {
	cases := []struct {
		name  string
		roots map[string]string
		a, b  string
		want  []string
	}{
		{"shared incompatible", map[string]string{"a": "1.0.0", "b": "1.0.0"}, "^1.2", "^2.0", []string{"acme/c", "a range ^1.2", "b range ^2.0"}},
		{"exact root against incoming range", map[string]string{"a": "1.0.0", "c": "2.0.0"}, "^1.2", "", []string{"acme/c", "a range ^1.2", "c version 2.0.0"}},
		{"exact release root against range", map[string]string{"a": "1.0.0", "c": "2.0.0"}, "^1.2", "release", []string{"acme/c", "a range ^1.2", "c release 2"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := conflictRequirements(tc.roots)
			if tc.b == "release" {
				root := req.Dependencies["c"]
				root.Registry.Selector = Selector{Kind: SelectorRelease, Value: "2"}
				req.Dependencies["c"] = root
			}
			resolver := &conflictGraphResolver{packages: map[string][]ResolvedPackage{"acme/a": {conflictPackage("a", "1.0.0", 10, map[string]string{"acme/c": tc.a})}, "acme/b": {conflictPackage("b", "1.0.0", 11, map[string]string{"acme/c": tc.b})}, "acme/c": {conflictPackage("c", "1.2.0", 1, nil), conflictPackage("c", "2.0.0", 2, nil)}}}
			mutator := &conflictMutator{}
			coordinator := Coordinator{Resolver: resolver, Mutator: mutator}
			lock, err := coordinator.Resolve(context.Background(), req, Lock{}, "")
			if err == nil {
				t.Fatalf("conflict published lock: %+v", lock)
			}
			for _, part := range tc.want {
				if !strings.Contains(err.Error(), part) {
					t.Errorf("error %q lacks %q", err, part)
				}
			}
			if len(lock.Packages) != 0 || mutator.calls != 0 {
				t.Fatalf("partial publication or mutation: %+v calls=%d", lock, mutator.calls)
			}
		})
	}
}

func TestGraphBacktracksCandidateDependencies(t *testing.T) {
	var firstLock []byte
	for _, order := range [][]string{{"a", "b"}, {"b", "a"}} {
		t.Run("overlap minimum "+strings.Join(order, ""), func(t *testing.T) {
			req := conflictRequirements(map[string]string{})
			for _, alias := range order {
				req.Dependencies[alias] = conflictRequirement(alias, "1.0.0")
				req.Targets["codex"] = append(req.Targets["codex"], alias)
			}
			resolver := &conflictGraphResolver{packages: map[string][]ResolvedPackage{"acme/a": {conflictPackage("a", "1.0.0", 1, map[string]string{"acme/c": "^1.2"})}, "acme/b": {conflictPackage("b", "1.0.0", 2, map[string]string{"acme/c": "^1.3"})}, "acme/c": {conflictPackage("c", "1.2.0", 3, nil), conflictPackage("c", "1.3.0", 4, nil), conflictPackage("c", "1.10.0", 5, nil), conflictPackage("c", "2.0.0", 6, nil)}}}
			lock, err := (Coordinator{Resolver: resolver}).Resolve(context.Background(), req, Lock{}, "")
			if err != nil {
				t.Fatal(err)
			}
			c, ok := lock.Package("c")
			if !ok || c.Version != "1.3.0" || len(c.RequiredBy) != 2 {
				t.Fatalf("overlap minimum: %+v", lock)
			}
			body, err := json.Marshal(lock)
			if err != nil {
				t.Fatal(err)
			}
			if firstLock != nil && string(firstLock) != string(body) {
				t.Fatalf("root input order changed lock bytes:\n%s\n%s", firstLock, body)
			}
			firstLock = body
		})
	}

	for _, independent := range []bool{false, true} {
		t.Run(fmt.Sprintf("update independent root %v", independent), func(t *testing.T) {
			req := conflictRequirements(map[string]string{"a": "^1.0"})
			if independent {
				req.Dependencies["b"] = conflictRequirement("b", "^1.0")
				req.Targets["codex"] = append(req.Targets["codex"], "b")
			}
			resolver := &conflictGraphResolver{packages: map[string][]ResolvedPackage{"acme/a": {conflictPackage("a", "1.1.0", 1, map[string]string{"acme/b": "^1.0"})}, "acme/b": {conflictPackage("b", "1.2.0", 2, nil)}}}
			coordinator := Coordinator{Resolver: resolver}
			previous, err := coordinator.Resolve(context.Background(), req, Lock{}, "")
			if err != nil {
				t.Fatal(err)
			}
			before, _ := json.Marshal(previous)
			resolver.packages["acme/a"] = []ResolvedPackage{conflictPackage("a", "1.0.0", 3, map[string]string{"acme/b": "^1.3"}), resolver.packages["acme/a"][0]}
			resolver.packages["acme/b"] = append(resolver.packages["acme/b"], conflictPackage("b", "1.3.0", 4, nil))
			updated, err := coordinator.Resolve(context.Background(), req, previous, "a")
			if err != nil {
				t.Fatal(err)
			}
			selected, _ := updated.Package("a")
			shared, _ := updated.Package("b")
			wantA, wantB := "1.0.0", "1.3.0"
			if independent {
				wantA, wantB = "1.1.0", "1.2.0"
			}
			if selected.Version != wantA || shared.Version != wantB {
				t.Fatalf("independent pin changed: %+v", updated)
			}
			after, _ := json.Marshal(previous)
			if string(before) != string(after) {
				t.Fatal("update mutated prior lock")
			}
		})
	}

	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprint(reverse), func(t *testing.T) {
			a := []ResolvedPackage{conflictPackage("a", "1.0.0", 10, map[string]string{"acme/c": "^1.0", "acme/d": "^1.0"}), conflictPackage("a", "1.1.0", 11, map[string]string{"acme/c": "^2.0"})}
			if reverse {
				a[0], a[1] = a[1], a[0]
			}
			resolver := &conflictGraphResolver{packages: map[string][]ResolvedPackage{"acme/a": a, "acme/b": {conflictPackage("b", "1.0.0", 20, map[string]string{"acme/c": "^2.0"})}, "acme/c": {conflictPackage("c", "1.0.0", 30, nil), conflictPackage("c", "2.0.0", 31, nil)}, "acme/d": {conflictPackage("d", "1.0.0", 40, nil)}}}
			lock, err := (Coordinator{Resolver: resolver}).Resolve(context.Background(), conflictRequirements(map[string]string{"a": "^1.0", "b": "1.0.0"}), Lock{}, "")
			if err != nil {
				t.Fatal(err)
			}
			selected, _ := lock.Package("a")
			shared, _ := lock.Package("c")
			if selected.Version != "1.1.0" || shared.Version != "2.0.0" || len(shared.RequiredBy) != 2 {
				t.Fatalf("wrong backtracked graph: %+v", lock)
			}
			if _, exists := lock.Package("d"); exists {
				t.Fatal("rejected candidate's dependency survived")
			}
		})
	}
}

func TestGraphCyclesTerminateWithTargets(t *testing.T) {
	for _, tc := range []struct {
		name, back string
		fails      bool
	}{{"compatible", "^1.0", false}, {"conflicting", "^2.0", true}} {
		t.Run(tc.name, func(t *testing.T) {
			resolver := &conflictGraphResolver{packages: map[string][]ResolvedPackage{"acme/a": {conflictPackage("a", "1.0.0", 1, map[string]string{"acme/b": "^1.0"})}, "acme/b": {conflictPackage("b", "1.0.0", 2, map[string]string{"acme/a": tc.back})}}}
			req := conflictRequirements(map[string]string{"a": "1.0.0"})
			req.Targets["claude"] = []string{"a"}
			lock, err := (Coordinator{Resolver: resolver}).Resolve(context.Background(), req, Lock{}, "")
			if (err != nil) != tc.fails {
				t.Fatalf("lock=%+v error=%v", lock, err)
			}
			if tc.fails {
				return
			}
			if len(lock.Packages) != 2 {
				t.Fatalf("cycle duplicated nodes: %+v", lock)
			}
			for _, pkg := range lock.Packages {
				if !reflect.DeepEqual(pkg.Targets, []string{"claude", "codex"}) || len(pkg.RequiredBy) != 1 {
					t.Fatalf("cycle lost targets/edge: %+v", pkg)
				}
			}
			if resolver.acquisitions != 2 {
				t.Fatalf("cycle reacquired %d candidates", resolver.acquisitions)
			}
		})
	}
}

func TestGraphDeterministicFailureAndAcquisitionAbort(t *testing.T) {
	for _, tc := range []struct {
		name    string
		failure error
	}{{"exhausted conflict", nil}, {"acquisition fails hard", errors.New("immutable digest mismatch")}} {
		t.Run(tc.name, func(t *testing.T) {
			var first string
			for _, order := range [][]string{{"a", "b"}, {"b", "a"}} {
				req := conflictRequirements(map[string]string{})
				for _, alias := range order {
					req.Dependencies[alias] = conflictRequirement(alias, "^1.0")
					req.Targets["codex"] = append(req.Targets["codex"], alias)
				}
				before, _ := json.Marshal(req)
				resolver := &conflictGraphResolver{failure: tc.failure, packages: map[string][]ResolvedPackage{"acme/a": {conflictPackage("a", "1.0.0", 1, map[string]string{"acme/c": "^1.0"}), conflictPackage("a", "1.1.0", 2, map[string]string{"acme/c": "^1.0"})}, "acme/b": {conflictPackage("b", "1.0.0", 3, map[string]string{"acme/c": "^2.0"})}, "acme/c": {conflictPackage("c", "1.0.0", 4, nil), conflictPackage("c", "2.0.0", 5, nil)}}}
				lock, err := (Coordinator{Resolver: resolver}).Resolve(context.Background(), req, Lock{}, "")
				if err == nil || len(lock.Packages) != 0 {
					t.Fatalf("failure returned partial graph: %+v %v", lock, err)
				}
				if first != "" && first != err.Error() {
					t.Fatalf("input order changed error: %q != %q", first, err)
				}
				first = err.Error()
				after, _ := json.Marshal(req)
				if string(before) != string(after) {
					t.Fatal("conflict changed intent")
				}
				if tc.failure != nil && (!errors.Is(err, tc.failure) || resolver.acquisitions != 1) {
					t.Fatalf("acquisition fallback occurred: %v calls=%d", err, resolver.acquisitions)
				}
			}
		})
	}
}

// Policy discovered after acquisition may reject a single candidate. Transport,
// authenticity and manifest failures must not silently select another release.
func TestGraphSkipsOnlyIneligibleCandidate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		failure error
		skips   bool
	}{
		{"late eligibility", fmt.Errorf("MCP policy: %w", ErrCandidateIneligible), true},
		{"network", errors.New("network unavailable"), false},
		{"digest", errors.New("immutable digest mismatch"), false},
		{"manifest", errors.New("invalid manifest"), false},
		{"version", errors.New("directory version differs from manifest"), false},
		{"untyped eligibility text", errors.New(ErrCandidateIneligible.Error()), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolver := &conflictGraphResolver{packages: map[string][]ResolvedPackage{"acme/a": {conflictPackage("a", "1.1.0", 2, nil), conflictPackage("a", "1.0.0", 1, nil)}}, candidateFailures: map[uint64]error{1: tc.failure}}
			lock, err := (Coordinator{Resolver: resolver}).Resolve(context.Background(), conflictRequirements(map[string]string{"a": "^1.0"}), Lock{}, "")
			if !tc.skips {
				if !errors.Is(err, tc.failure) || len(lock.Packages) != 0 || resolver.acquisitions != 1 {
					t.Fatalf("fatal acquisition selected a fallback: %+v %v calls=%d", lock, err, resolver.acquisitions)
				}
				return
			}
			if err != nil || len(lock.Packages) != 1 || lock.Packages[0].Version != "1.1.0" || resolver.acquisitions != 2 {
				t.Fatalf("late ineligibility blocked safe candidate: %+v %v calls=%d", lock, err, resolver.acquisitions)
			}
		})
	}
	// An exhausted child becomes a graph conflict so its parent can backtrack.
	resolver := &conflictGraphResolver{packages: map[string][]ResolvedPackage{
		"acme/a": {conflictPackage("a", "1.0.0", 1, map[string]string{"acme/b": "1.0.0"}), conflictPackage("a", "1.1.0", 2, nil)},
		"acme/b": {conflictPackage("b", "1.0.0", 3, nil)},
	}, candidateFailures: map[uint64]error{3: ErrCandidateIneligible}}
	lock, err := (Coordinator{Resolver: resolver}).Resolve(context.Background(), conflictRequirements(map[string]string{"a": "^1.0"}), Lock{}, "")
	if err != nil || len(lock.Packages) != 1 || lock.Packages[0].Version != "1.1.0" || resolver.acquisitions != 3 {
		t.Fatalf("exhausted child failed to backtrack its parent: %+v %v calls=%d", lock, err, resolver.acquisitions)
	}
}
