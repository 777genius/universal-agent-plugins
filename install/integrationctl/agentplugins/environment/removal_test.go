package environment

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func removalPackage(alias string, consumers ...string) LockedPackage {
	pkg := LockedPackage{Alias: alias, Version: "1.2.0", SourceKey: "directory:acme/" + alias,
		DigestAlgorithm: TreeDigestAlgorithm, TreeDigest: "sha256:" + strings.Repeat("a", 64), ManifestDigest: "sha256:" + strings.Repeat("b", 64), Targets: []string{"codex"},
		Source: LockedSource{Registry: &LockedRegistrySource{Distribution: "acme/" + alias, DistributionID: "acme/" + alias, ReleaseSequence: 1,
			Requested: Selector{Kind: SelectorRelease, Value: "1"}, Repository: "acme/packages", Path: alias, Commit: strings.Repeat("1", 40)}}}
	for _, consumer := range consumers {
		pkg.RequiredBy = append(pkg.RequiredBy, RequiredBy{Alias: consumer, Selector: Selector{Kind: SelectorRange, Value: "^1.2"}})
	}
	return pkg
}

func removalRequirements(roots ...string) Requirements {
	req := Requirements{SchemaVersion: 1, Dependencies: map[string]Requirement{}, Targets: map[string][]string{}}
	for _, alias := range roots {
		req.Dependencies[alias] = Requirement{Alias: alias, Registry: &RegistrySource{Distribution: "acme/" + alias, Selector: Selector{Kind: SelectorRelease, Value: "1"}}}
		req.Targets["codex"] = append(req.Targets["codex"], alias)
	}
	return req
}

func removalLock(t *testing.T, req Requirements, packages ...LockedPackage) Lock {
	t.Helper()
	return Lock{LockVersion: 1, ManifestDigest: mustDigest(t, req), Packages: packages}
}

func TestRemovalLiveReferences(t *testing.T) {
	for _, tc := range []struct {
		name        string
		roots, want []string
		extra       []LockedPackage
	}{
		{name: "diamond retains shared dependency", roots: []string{"b"}, want: []string{"b", "c"}},
		{name: "last consumer disappears", want: []string{}},
		{name: "direct root retains dependency", roots: []string{"c"}, want: []string{"c"}},
		{name: "unreachable internal cycle does not retain", roots: []string{"b"}, want: []string{"b", "c"}, extra: []LockedPackage{removalPackage("d", "e"), removalPackage("e", "d")}},
		{name: "former root remains transitive", roots: []string{"a"}, want: []string{"a", "c"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previousReq := removalRequirements("a", "b", "c")
			previous := removalLock(t, previousReq, removalPackage("a"), removalPackage("b"), removalPackage("c", "a", "b"))
			previous.Packages = append(previous.Packages, tc.extra...)
			req := removalRequirements(tc.roots...)
			got, err := (Coordinator{}).Resolve(context.Background(), req, previous, "")
			if err != nil {
				t.Fatal(err)
			}
			names := make([]string, 0, len(got.Packages))
			for _, pkg := range got.Packages {
				names = append(names, pkg.Alias)
			}
			if !slices.Equal(names, tc.want) {
				t.Fatalf("packages=%v want=%v", names, tc.want)
			}
			if pkg, ok := got.Package("c"); ok {
				old, _ := previous.Package("c")
				if !SamePin(old, pkg) {
					t.Fatal("retained shared pin changed")
				}
				consumers := make([]string, 0, len(pkg.RequiredBy))
				for _, edge := range pkg.RequiredBy {
					consumers = append(consumers, edge.Alias)
				}
				wanted := []string{}
				for _, root := range tc.roots {
					if root == "a" || root == "b" {
						wanted = append(wanted, root)
					}
				}
				if !slices.Equal(consumers, wanted) {
					t.Fatalf("consumers=%v want=%v", consumers, wanted)
				}
			}
		})
	}
}

func TestRemovalTargetBindings(t *testing.T) {
	for _, tc := range []struct {
		name    string
		roots   []string
		directC bool
		split   bool
		removed []string
	}{
		{name: "same target retains c", roots: []string{"b"}, removed: []string{"a/codex"}},
		{name: "different targets remove only lost binding", roots: []string{"b"}, split: true, removed: []string{"a/codex", "c/codex"}},
		{name: "last target removes c", removed: []string{"a/codex", "b/codex", "c/codex"}},
		{name: "direct assignment retains c", directC: true, removed: []string{"a/codex", "b/codex"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oldReq := removalRequirements("a", "b")
			old := removalLock(t, oldReq, removalPackage("a"), removalPackage("b"), removalPackage("c", "a", "b"))
			if tc.split {
				oldReq.Targets = map[string][]string{"codex": {"a"}, "claude": {"b"}}
				old = removalLock(t, oldReq, removalPackage("a"), removalPackage("b"), removalPackage("c", "a", "b"))
				old.Packages[1].Targets = []string{"claude"}
				old.Packages[2].Targets = []string{"claude", "codex"}
			}
			req := removalRequirements(tc.roots...)
			if tc.split {
				req.Targets = map[string][]string{"claude": {"b"}}
			}
			if tc.directC {
				req = removalRequirements("c")
			}
			next, err := (Coordinator{}).Resolve(context.Background(), req, old, "")
			if err != nil {
				t.Fatal(err)
			}
			observed := make([]BindingObservation, 0, len(old.Packages))
			for _, pkg := range old.Packages {
				for _, target := range pkg.Targets {
					observed = append(observed, BindingObservation{Alias: pkg.Alias, ClientID: target, Managed: true, SourceKey: pkg.SourceKey, PackageDigest: pkg.TreeDigest, Revision: pkg.Source.Registry.Commit})
				}
			}
			plan, err := (Coordinator{}).PrepareSync(req, next, observed)
			if err != nil {
				t.Fatal(err)
			}
			removed := []string{}
			for _, binding := range plan.Bindings {
				if binding.Action == ActionRemove {
					removed = append(removed, binding.Alias+"/"+binding.ClientID)
				}
				if binding.Alias == "c" && binding.Action == ActionUnchanged && len(tc.roots) > 0 && !strings.Contains(binding.Reason, "b") {
					t.Fatal("retention lacks consumer reason")
				}
			}
			if !slices.Equal(removed, tc.removed) {
				t.Fatalf("removes=%v want=%v", removed, tc.removed)
			}
		})
	}
}

func TestRemovalSelectionAndAttestation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		selected  []string
		attest    *Attestation
		wantError bool
	}{
		{name: "expanded selection attests only root", selected: []string{"a", "c"}, attest: &Attestation{Alias: "a", ClientID: "codex", AuthComplete: true}},
		{name: "unselected attestation fails", selected: []string{"c"}, attest: &Attestation{Alias: "a", ClientID: "codex", AuthComplete: true}, wantError: true},
		{name: "wrong target fails", selected: []string{"a", "c"}, attest: &Attestation{Alias: "a", ClientID: "claude", AuthComplete: true}, wantError: true},
		{name: "empty completion fails", selected: []string{"a", "c"}, attest: &Attestation{Alias: "a", ClientID: "codex"}, wantError: true},
		{name: "closure selection without attestation", selected: []string{"a", "c"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := removalRequirements("a", "b")
			lock := removalLock(t, req, removalPackage("a"), removalPackage("b"), removalPackage("c", "a", "b"))
			if got := lock.ClosureAliases([]string{"a"}); !reflect.DeepEqual(got, []string{"a", "c"}) {
				t.Fatalf("closure=%v", got)
			}
			observed := make([]BindingObservation, 0, len(lock.Packages))
			for _, pkg := range lock.Packages {
				observed = append(observed, BindingObservation{Alias: pkg.Alias, ClientID: "codex", Managed: true, SourceKey: pkg.SourceKey, PackageDigest: pkg.TreeDigest, Revision: pkg.Source.Registry.Commit})
			}
			plan, err := (Coordinator{}).PrepareSyncFor(req, lock, observed, tc.selected, tc.attest)
			if (err != nil) != tc.wantError {
				t.Fatalf("err=%v wantError=%v", err, tc.wantError)
			}
			if err != nil {
				return
			}
			for _, binding := range plan.Bindings {
				if binding.Alias == "b" {
					t.Fatal("independent root selected")
				}
				if binding.Alias == "c" && (binding.AuthComplete || binding.ActivationComplete) {
					t.Fatal("attestation leaked to dependency")
				}
			}
		})
	}
}
