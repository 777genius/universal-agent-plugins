package environment

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestVersionSelectionBoundaries(t *testing.T) {
	for _, tc := range []struct {
		expression, version string
		want                bool
	}{
		{"1.2.3", "1.2.3", true}, {"1.2.3", "1.2.4", false},
		{"^1.2", "1.2.0", true}, {"^1.2", "1.3.0", true}, {"^1.2", "1.1.9", false}, {"^1.2", "2.0.0", false},
		{"^0.2", "0.2.0", true}, {"^0.2", "0.3.0", false}, {"^0.2.3", "0.2.2", false}, {"^0.2.3", "0.2.9", true},
		{"^0.0", "0.0.9", true}, {"^0.0", "0.1.0", false}, {"^0.0.3", "0.0.3", true}, {"^0.0.3", "0.0.4", false},
		{"^1.2.3", "1.2.2", false}, {"^1.2.3", "1.2.3", true}, {"^1.2", "1.2.3-beta", false},
	} {
		t.Run(tc.expression+"/"+tc.version, func(t *testing.T) {
			kind := SelectorVersion
			if strings.HasPrefix(tc.expression, "^") {
				kind = SelectorRange
			}
			selector := Selector{Kind: kind, Value: tc.expression}
			if err := selector.Validate(); err != nil {
				t.Fatal(err)
			}
			if got := SelectorMatchesVersion(selector, tc.version); got != tc.want {
				t.Fatalf("%+v matches %s=%v", selector, tc.version, got)
			}
		})
	}
}

type versionSelectionResolver struct {
	candidates           map[string][]Candidate
	dependencies         map[string]map[string]string
	deny                 map[string]bool
	enumerated, acquired int
}

func (r *versionSelectionResolver) Resolve(ctx context.Context, req Requirement, old *LockedPackage, update bool) (LockedPackage, error) {
	return LockedPackage{}, fmt.Errorf("unexpected exact resolution")
}
func (r *versionSelectionResolver) Candidates(_ context.Context, req Requirement, _ []string) ([]Candidate, error) {
	r.enumerated++
	return append([]Candidate(nil), r.candidates[req.Registry.Distribution]...), nil
}
func (r *versionSelectionResolver) AcquireCandidate(_ context.Context, req Requirement, c Candidate, targets []string) (ResolvedPackage, error) {
	r.acquired++
	distribution := req.Registry.Distribution
	alias := req.Alias
	pkg := LockedPackage{Alias: alias, SourceKey: "directory:" + distribution, Version: c.Version, DigestAlgorithm: TreeDigestAlgorithm, TreeDigest: "sha256:" + strings.Repeat("a", 64), ManifestDigest: "sha256:" + strings.Repeat("b", 64), Targets: targets, Source: LockedSource{Registry: &LockedRegistrySource{Distribution: distribution, DistributionID: distribution, ProductID: distribution, ReleaseSequence: c.ReleaseSequence, Requested: req.Registry.Selector, Repository: "acme/source", Commit: strings.Repeat("1", 40)}}}
	return ResolvedPackage{Package: pkg, Dependencies: r.dependencies[distribution+"@"+c.Version]}, nil
}
func (r *versionSelectionResolver) DescribePinned(_ context.Context, pkg LockedPackage) (ResolvedPackage, error) {
	return ResolvedPackage{Package: pkg, Dependencies: r.dependencies[pkg.Source.Registry.Distribution+"@"+pkg.Version]}, nil
}
func (r *versionSelectionResolver) Eligible(_ context.Context, pkg LockedPackage, targets []string) (bool, error) {
	for _, target := range targets {
		if r.deny[pkg.Source.Registry.Distribution+"@"+pkg.Version+"/"+target] {
			return false, nil
		}
	}
	return true, nil
}
func versionSelectionRequirements() Requirements {
	return Requirements{SchemaVersion: 1, Dependencies: map[string]Requirement{"a": {Alias: "a", Registry: &RegistrySource{Distribution: "acme/a", Selector: Selector{Kind: SelectorRange, Value: "^1.2"}}}}, Targets: map[string][]string{"codex": {"a"}}}
}

func TestVersionSelectionMinimalGraph(t *testing.T) {
	for _, tc := range []struct {
		name     string
		deny     map[string]bool
		want     string
		sequence uint64
		stale    bool
	}{
		{name: "numeric order and equal sequences", want: "1.2.0", sequence: 2},
		{name: "target union", deny: map[string]bool{"acme/a@1.2.0/claude": true}, want: "1.3.0", sequence: 4},
		{name: "discard rejected dependencies", deny: map[string]bool{"acme/c@1.2.0/codex": true}, want: "1.3.0", sequence: 4, stale: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := versionSelectionRequirements()
			req.Targets["claude"] = []string{"a"}
			resolver := &versionSelectionResolver{candidates: map[string][]Candidate{"acme/a": {{Version: "1.10.0", ReleaseSequence: 1}, {Version: "1.3.0", ReleaseSequence: 4}, {Version: "1.2.0", ReleaseSequence: 3}, {Version: "1.2.0", ReleaseSequence: 2}}, "acme/c": {{Version: "1.2.0", ReleaseSequence: 1}}}, deny: tc.deny, dependencies: map[string]map[string]string{}}
			if tc.stale {
				resolver.dependencies["acme/a@1.2.0"] = map[string]string{"acme/c": "^1.2"}
			}
			lock, err := (Coordinator{Resolver: resolver}).Resolve(context.Background(), req, Lock{}, "")
			if err != nil {
				t.Fatal(err)
			}
			pkg, ok := lock.Package("a")
			if !ok || pkg.Version != tc.want || pkg.Source.Registry.ReleaseSequence != tc.sequence || len(pkg.Targets) != 2 {
				t.Fatalf("selected=%+v", pkg)
			}
			if len(lock.Packages) != 1 {
				t.Fatalf("rejected candidate left nodes: %+v", lock.Packages)
			}
		})
	}
}

func TestVersionSelectionReplayKeepsCompatiblePin(t *testing.T) {
	for _, expression := range []string{"^1.2", "^1.3", "1.3.0"} {
		t.Run(expression, func(t *testing.T) {
			req := versionSelectionRequirements()
			resolver := &versionSelectionResolver{candidates: map[string][]Candidate{"acme/a": {{Version: "1.3.0", ReleaseSequence: 3}}}}
			coordinator := Coordinator{Resolver: resolver}
			old, err := coordinator.Resolve(context.Background(), req, Lock{}, "")
			if err != nil {
				t.Fatal(err)
			}
			req.Dependencies["a"].Registry.Selector = Selector{Kind: SelectorRange, Value: expression}
			if expression == "1.3.0" {
				req.Dependencies["a"].Registry.Selector.Kind = SelectorVersion
			}
			resolver.candidates["acme/a"] = []Candidate{{Version: "1.2.0", ReleaseSequence: 1}, {Version: "1.3.0", ReleaseSequence: 3}}
			resolver.enumerated, resolver.acquired = 0, 0
			lock, err := coordinator.Resolve(context.Background(), req, old, "")
			if err != nil {
				t.Fatal(err)
			}
			if len(lock.Packages) != 1 || lock.Packages[0].Version != "1.3.0" || resolver.enumerated != 0 || resolver.acquired != 0 {
				t.Fatalf("replay=%+v enumerate=%d acquire=%d", lock, resolver.enumerated, resolver.acquired)
			}
		})
	}
}
