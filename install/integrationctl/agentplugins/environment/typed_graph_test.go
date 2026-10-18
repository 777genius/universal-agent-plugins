package environment

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// Unlike a GraphResolver, this adapter cannot establish whether a package is a leaf.
type resolverWithoutFacts struct{ calls int }

func (r *resolverWithoutFacts) Resolve(context.Context, Requirement, []string, *LockedPackage, bool) (LockedPackage, error) {
	r.calls++
	return LockedPackage{}, errors.New("must not resolve without facts")
}

func TestManagedResolutionRequiresVerifiedFacts(t *testing.T) {
	req := removalRequirements("a")
	previous := removalLock(t, req, removalPackage("a"))
	for _, old := range []Lock{{}, previous} {
		resolver := &resolverWithoutFacts{}
		lock, err := (Coordinator{Resolver: resolver}).Resolve(context.Background(), req, old, "")
		if !errors.Is(err, ErrDependencyFactsUnavailable) || len(lock.Packages) != 0 || resolver.calls != 0 {
			t.Fatalf("unverified graph accepted: %+v %v calls=%d", lock, err, resolver.calls)
		}
	}
	empty := Requirements{SchemaVersion: 1}
	lock, err := (Coordinator{}).Resolve(context.Background(), empty, previous, "")
	if err != nil || len(lock.Packages) != 0 {
		t.Fatalf("cleanup required deleted package facts: %+v %v", lock, err)
	}
}

type typedGraphResolver struct {
	*conflictGraphResolver
	git          ResolvedPackage
	exactTargets map[string][]string
	denyTarget   string
	denySource   string
	described    map[string]int
}

func (r *typedGraphResolver) Resolve(ctx context.Context, req Requirement, targets []string, old *LockedPackage, update bool) (LockedPackage, error) {
	key, _ := req.SourceKey()
	r.exactTargets[key] = append([]string{}, targets...)
	if req.Git != nil {
		pkg := r.git.Package
		setRequested(&pkg, req)
		return pkg, nil
	}
	return r.conflictGraphResolver.Resolve(ctx, req, targets, old, update)
}
func (r *typedGraphResolver) DescribePinned(ctx context.Context, pkg LockedPackage) (ResolvedPackage, error) {
	r.described[pkg.SourceKey]++
	if pkg.Source.Git != nil {
		return ResolvedPackage{Package: pkg, Dependencies: r.git.Dependencies}, nil
	}
	return r.conflictGraphResolver.DescribePinned(ctx, pkg)
}
func (r *typedGraphResolver) Eligible(_ context.Context, pkg LockedPackage, targets []string) (bool, error) {
	source := r.denySource
	if source == "" {
		source = r.git.Package.SourceKey
	}
	return r.denyTarget == "" || pkg.SourceKey != source || !slices.Contains(targets, r.denyTarget), nil
}

func typedGraphFixture() (Requirements, *typedGraphResolver) {
	req := conflictRequirements(map[string]string{"a": "^1.0", "d": "1.0.0"})
	a := req.Dependencies["a"]
	delete(req.Dependencies, "a")
	a.Alias = "acme/a"
	req.Dependencies[a.Alias] = a
	req.Targets = map[string][]string{"codex": {a.Alias}, "claude": {"d"}}
	b := Requirement{Alias: "local-b", Git: &GitSource{Repository: "acme/tools", Path: "plugins/b", Selector: Selector{Kind: SelectorRef, Value: "main"}}}
	bkey, _ := b.SourceKey()
	ap := conflictPackage("a", "1.0.0", 10, nil)
	ap.Package.Alias = "actual-a"
	ap.Dependencies = map[string]Requirement{b.Alias: b}
	dp := conflictPackage("d", "1.0.0", 11, map[string]string{"acme/c": "^1.2"})
	dp.Dependencies[b.Alias] = b
	bp := ResolvedPackage{Package: LockedPackage{Alias: "actual-b", SourceKey: bkey,
		DigestAlgorithm: TreeDigestAlgorithm, TreeDigest: "sha256:" + strings.Repeat("a", 64), ManifestDigest: "sha256:" + strings.Repeat("b", 64),
		Source: LockedSource{Git: &LockedGitSource{Repository: b.Git.Repository, Path: b.Git.Path, Requested: b.Git.Selector, Commit: strings.Repeat("1", 40)}}},
		Dependencies: map[string]Requirement{"exact-c": {Alias: "exact-c", Registry: &RegistrySource{Distribution: "acme/c", Selector: Selector{Kind: SelectorRelease, Value: "2"}}}}}
	return req, &typedGraphResolver{conflictGraphResolver: &conflictGraphResolver{packages: map[string][]ResolvedPackage{
		"acme/a": {ap}, "acme/d": {dp}, "acme/c": {conflictPackage("c", "1.2.0", 1, nil), conflictPackage("c", "1.2.0", 2, nil)},
	}}, git: bp, exactTargets: map[string][]string{}, described: map[string]int{}}
}

func TestTypedMixedGraphPreservesSourceIdentityAndTargets(t *testing.T) {
	req, resolver := typedGraphFixture()
	coordinator := Coordinator{Resolver: resolver}
	lock, err := coordinator.Resolve(context.Background(), req, Lock{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Packages) != 4 {
		t.Fatalf("incomplete mixed closure: %+v", lock)
	}
	b, ok := lock.Package("actual-b")
	if !ok || !slices.Equal(b.Targets, []string{"claude", "codex"}) || len(b.RequiredBy) != 2 {
		t.Fatalf("Git child lost edges/union: %+v", b)
	}
	if !slices.Equal(resolver.exactTargets[b.SourceKey], b.Targets) {
		t.Fatalf("exact acquisition lost targets: %v", resolver.exactTargets)
	}
	c, _ := lock.Package("c")
	if c.Source.Registry.ReleaseSequence != 2 || len(c.RequiredBy) != 2 {
		t.Fatalf("exact release lost intersection: %+v", c)
	}
	if err := lock.Validate(req); err != nil {
		t.Fatal(err)
	}
	// A declaration can become direct under a different key without rebinding its owner.
	req.Dependencies["tools"] = Requirement{Alias: "tools", Git: &GitSource{Repository: "acme/tools", Path: "plugins/b", Selector: b.Source.Git.Requested}}
	req.Targets["codex"] = append(req.Targets["codex"], "tools")
	promoted, err := coordinator.Resolve(context.Background(), req, lock, "")
	if err != nil {
		t.Fatal(err)
	}
	promotedB, _ := promoted.Package("actual-b")
	if !SamePin(b, promotedB) {
		t.Fatalf("promotion changed pin/owner: %+v", promotedB)
	}
	delete(req.Dependencies, "tools")
	req.Targets["codex"] = []string{"acme/a"}
	demoted, err := coordinator.Resolve(context.Background(), req, promoted, "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(lock, demoted) {
		t.Fatalf("direct/transitive round trip changed graph: %+v", demoted)
	}
	plan, err := coordinator.PrepareSyncFor(req, lock, nil, []string{"acme/a"}, nil)
	if err != nil || len(plan.Bindings) != 1 || plan.Bindings[0].Alias != "actual-a" {
		t.Fatalf("canonical selection lost stable alias: %+v %v", plan, err)
	}
	resolver.described = map[string]int{}
	retainedReq := conflictRequirements(map[string]string{"d": "1.0.0"})
	retainedReq.Targets = map[string][]string{"claude": {"d"}}
	retained, err := coordinator.Resolve(context.Background(), retainedReq, lock, "")
	if err != nil {
		t.Fatal(err)
	}
	retainedB, _ := retained.Package("actual-b")
	if !SamePin(b, retainedB) || len(retainedB.RequiredBy) != 1 || retainedB.RequiredBy[0].Alias != "d" || !slices.Equal(retainedB.Targets, []string{"claude"}) {
		t.Fatalf("removing a root changed shared Git owner or left edges: %+v", retainedB)
	}
	if resolver.described["directory:acme/a"] != 0 {
		t.Fatal("removal read deleted package declarations")
	}
	observed := []BindingObservation{{Alias: b.Alias, ClientID: "claude", Managed: true, SourceKey: b.SourceKey, PackageDigest: b.TreeDigest, Revision: b.Source.Git.Commit, InstallationID: "existing-installation", BindingID: "existing-binding"}}
	retainedPlan, err := coordinator.PrepareSyncFor(retainedReq, retained, observed, []string{b.Alias}, nil)
	if err != nil || len(retainedPlan.Bindings) != 1 || retainedPlan.Bindings[0].Action != ActionUnchanged || retainedPlan.Bindings[0].InstallationID != "existing-installation" || retainedPlan.Bindings[0].BindingID != "existing-binding" {
		t.Fatalf("retained Git ownership changed: %+v %v", retainedPlan, err)
	}
	// A changed Git edge cannot be hidden behind an otherwise valid pin.
	changed := lock
	changed.Packages = append([]LockedPackage{}, lock.Packages...)
	for i := range changed.Packages {
		if changed.Packages[i].Alias == "actual-b" {
			changed.Packages[i].RequiredBy = append([]RequiredBy{}, b.RequiredBy...)
			changed.Packages[i].RequiredBy[0].Selector.Value = "other"
		}
	}
	if err := changed.Validate(req); err == nil {
		t.Fatal("tampered Git edge accepted")
	}
}

func TestTypedMixedGraphRejectsLateTargetAndSelectorConflicts(t *testing.T) {
	t.Run("late target union", func(t *testing.T) {
		req, resolver := typedGraphFixture()
		resolver.denyTarget = "codex"
		resolver.denySource = "directory:acme/c"
		if lock, err := (Coordinator{Resolver: resolver}).Resolve(context.Background(), req, Lock{}, ""); err == nil || len(lock.Packages) != 0 {
			t.Fatalf("unsupported union accepted: %+v %v", lock, err)
		}
	})
	for _, selector := range []Selector{{Kind: SelectorRef, Value: "other"}, {Kind: SelectorCommit, Value: strings.Repeat("1", 40)}, {Kind: SelectorDefault}} {
		t.Run(string(selector.Kind)+selector.Value, func(t *testing.T) {
			req, resolver := typedGraphFixture()
			d := resolver.packages["acme/d"][0]
			d.Dependencies["local-b"] = Requirement{Alias: "local-b", Git: &GitSource{Repository: "acme/tools", Path: "plugins/b", Selector: selector}}
			resolver.packages["acme/d"][0] = d
			if lock, err := (Coordinator{Resolver: resolver}).Resolve(context.Background(), req, Lock{}, ""); err == nil || len(lock.Packages) != 0 {
				t.Fatalf("different Git selector accepted: %+v %v", lock, err)
			}
		})
	}
	t.Run("distinct exact releases with same version", func(t *testing.T) {
		req, resolver := typedGraphFixture()
		d := resolver.packages["acme/d"][0]
		d.Dependencies["acme/c"] = Requirement{Alias: "acme/c", Registry: &RegistrySource{Distribution: "acme/c", Selector: Selector{Kind: SelectorRelease, Value: "1"}}}
		if lock, err := (Coordinator{Resolver: resolver}).Resolve(context.Background(), req, Lock{}, ""); err == nil || len(lock.Packages) != 0 {
			t.Fatalf("distinct release accepted: %+v %v", lock, err)
		}
	})
}

func TestDeclarationKeysDoNotWidenLockedAliasesOrOldIntent(t *testing.T) {
	req := Requirement{Alias: "demo", Git: &GitSource{Repository: "acme/demo", Selector: Selector{Kind: SelectorRef, Value: "main"}}}
	root := Requirements{SchemaVersion: 1, Dependencies: map[string]Requirement{"demo": req}, Targets: map[string][]string{"codex": {"demo"}}}
	digest, err := ManifestDigest(root)
	if err != nil || digest != "sha256:7edcd3b04a51e26f04de67eec7946610747d31260f1f822b58fffe426a2a22e4" {
		t.Fatalf("legacy intent changed: %s %v", digest, err)
	}
	req.Alias = "acme/demo"
	if err := req.Validate(); err != nil {
		t.Fatal(err)
	}
	pin := removalPackage("demo")
	pin.Alias = "acme/demo"
	if err := validatePin(pin); err == nil {
		t.Fatal("canonical declaration admitted as unsafe locked alias")
	}
}

func TestExactIncomingReleasePreservesDirectRootSelector(t *testing.T) {
	req, resolver := typedGraphFixture()
	req.Dependencies["acme/c"] = Requirement{Alias: "acme/c", Registry: &RegistrySource{Distribution: "acme/c", Selector: Selector{Kind: SelectorRange, Value: "^1.2"}}}
	req.Targets["codex"] = append(req.Targets["codex"], "acme/c")
	lock, err := (Coordinator{Resolver: resolver}).Resolve(context.Background(), req, Lock{}, "")
	if err != nil {
		t.Fatal(err)
	}
	c, _ := lock.Package("c")
	if c.Source.Registry.Requested != req.Dependencies["acme/c"].Registry.Selector || c.Source.Registry.ReleaseSequence != 2 {
		t.Fatalf("incoming exact release changed root intent: %+v", c)
	}
}

func TestCanonicalRootUpdateKeepsIndependentPins(t *testing.T) {
	req, resolver := typedGraphFixture()
	coordinator := Coordinator{Resolver: resolver}
	old, err := coordinator.Resolve(context.Background(), req, Lock{}, "")
	if err != nil {
		t.Fatal(err)
	}
	nextA := resolver.packages["acme/a"][0]
	nextA.Package.Version = "1.1.0"
	cloned := *nextA.Package.Source.Registry
	cloned.ReleaseSequence = 20
	nextA.Package.Source.Registry = &cloned
	resolver.packages["acme/a"] = append(resolver.packages["acme/a"], nextA)
	// Minimum selection remains the existing first release even during update.
	resolver.packages["acme/a"] = resolver.packages["acme/a"][1:]
	updated, err := coordinator.Resolve(context.Background(), req, old, "acme/a")
	if err != nil {
		t.Fatal(err)
	}
	a, _ := updated.Package("actual-a")
	if a.Version != "1.1.0" {
		t.Fatalf("canonical update failed: %+v", updated)
	}
	for _, alias := range []string{"actual-b", "c", "d"} {
		before, _ := old.Package(alias)
		after, _ := updated.Package(alias)
		if !SamePin(before, after) {
			t.Fatalf("canonical update changed unrelated pin %s", alias)
		}
	}
}

func TestGitDefaultSelectorTraversesTypedGraph(t *testing.T) {
	req, resolver := typedGraphFixture()
	for _, distribution := range []string{"acme/a", "acme/d"} {
		b := resolver.packages[distribution][0].Dependencies["local-b"]
		cloned := *b.Git
		cloned.Selector = Selector{Kind: SelectorDefault}
		b.Git = &cloned
		resolver.packages[distribution][0].Dependencies["local-b"] = b
	}
	lock, err := (Coordinator{Resolver: resolver}).Resolve(context.Background(), req, Lock{}, "")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := lock.Package("actual-b")
	if b.Source.Git.Requested != (Selector{Kind: SelectorDefault}) || len(b.RequiredBy) != 2 {
		t.Fatalf("default source lost typed facts: %+v", b)
	}
}

func TestCanonicalSourceCaseRetainsLockedIdentity(t *testing.T) {
	oldReq := removalRequirements("a")
	old := removalLock(t, oldReq, removalPackage("a"))
	req := Requirements{SchemaVersion: 1, Dependencies: map[string]Requirement{"Acme/A": {Alias: "Acme/A", Registry: &RegistrySource{Distribution: "Acme/A", Selector: Selector{Kind: SelectorRelease, Value: "1"}}}}, Targets: map[string][]string{"codex": {"Acme/A"}}}
	resolver := &conflictGraphResolver{packages: map[string][]ResolvedPackage{"acme/a": {{Package: old.Packages[0]}}}}
	lock, err := (Coordinator{Resolver: resolver}).Resolve(context.Background(), req, old, "")
	if err != nil {
		t.Fatal(err)
	}
	if !SamePin(old.Packages[0], lock.Packages[0]) {
		t.Fatal("source spelling changed locked identity")
	}
}

func TestNestedExactRegistryResolutionGetsConsumerUnion(t *testing.T) {
	req, resolver := typedGraphFixture()
	delete(resolver.packages["acme/d"][0].Dependencies, "acme/c")
	lock, err := (Coordinator{Resolver: resolver}).Resolve(context.Background(), req, Lock{}, "")
	if err != nil {
		t.Fatal(err)
	}
	c, _ := lock.Package("c")
	if !slices.Equal(resolver.exactTargets[c.SourceKey], []string{"claude", "codex"}) {
		t.Fatalf("nested exact registry request lacked consumer union: %v", resolver.exactTargets)
	}
}
