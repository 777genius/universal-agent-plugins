package environment

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func requirementsFixture() Requirements {
	return Requirements{
		SchemaVersion: ManifestSchemaVersion,
		Dependencies: map[string]Requirement{
			"review": {Alias: "review", Git: &GitSource{Repository: "acme/agent-tools", Path: "plugins/review",
				Selector: Selector{Kind: SelectorRef, Value: "main"}}},
			"docs": {Alias: "docs", Registry: &RegistrySource{Distribution: "acme/docs",
				Selector: Selector{Kind: SelectorRelease, Value: "42"}}},
		},
		Targets: map[string][]string{"codex": {"review", "docs"}, "claude": {"review"}},
	}
}

func TestManifestDigestIsIntentBoundAndOrderIndependent(t *testing.T) {
	left := requirementsFixture()
	right := requirementsFixture()
	right.Targets = map[string][]string{"claude": {"review"}, "codex": {"docs", "review"}}
	leftDigest, err := ManifestDigest(left)
	if err != nil {
		t.Fatal(err)
	}
	rightDigest, err := ManifestDigest(right)
	if err != nil {
		t.Fatal(err)
	}
	if leftDigest != rightDigest {
		t.Fatalf("normalized intent digest changed: %s != %s", leftDigest, rightDigest)
	}
	right.Dependencies["review"].Git.Selector.Value = "develop"
	changed, err := ManifestDigest(right)
	if err != nil {
		t.Fatal(err)
	}
	if changed == leftDigest {
		t.Fatal("selector change did not change intent digest")
	}
}

func TestRequirementAndLockContractsAreClosed(t *testing.T) {
	requirements := requirementsFixture()
	lock := Lock{LockVersion: LockVersion, ManifestDigest: mustDigest(t, requirements), Packages: []LockedPackage{
		{Alias: "review", SourceKey: "github:acme/agent-tools:plugins/review", DigestAlgorithm: TreeDigestAlgorithm,
			TreeDigest: "sha256:" + strings.Repeat("a", 64), ManifestDigest: "sha256:" + strings.Repeat("b", 64),
			Targets: []string{"claude", "codex"}, Source: LockedSource{Git: &LockedGitSource{Repository: "acme/agent-tools",
				Path: "plugins/review", Requested: requirements.Dependencies["review"].Git.Selector, Commit: strings.Repeat("1", 40), ResolvedRef: "refs/heads/main"}}},
		{Alias: "docs", SourceKey: "directory:acme/docs", DigestAlgorithm: TreeDigestAlgorithm,
			TreeDigest: "sha256:" + strings.Repeat("c", 64), ManifestDigest: "sha256:" + strings.Repeat("d", 64),
			Targets: []string{"codex"}, Source: LockedSource{Registry: &LockedRegistrySource{Distribution: "acme/docs",
				DistributionID: "acme/docs", ReleaseSequence: 42, Requested: requirements.Dependencies["docs"].Registry.Selector,
				Repository: "acme/docs-source", Commit: strings.Repeat("2", 40)}}},
	}}
	if err := lock.Validate(requirements); err != nil {
		t.Fatal(err)
	}
	lock.Packages[0].Source.Git.Requested.Value = "rewritten-selector"
	if err := lock.Validate(requirements); err == nil {
		t.Fatal("manifest selector accepted a rewritten locked revision")
	}
}

func TestPrepareSyncSeparatesUnchangedUpdateAndSourceChange(t *testing.T) {
	requirements := Requirements{SchemaVersion: ManifestSchemaVersion,
		Dependencies: map[string]Requirement{"review": {Alias: "review", Git: &GitSource{Repository: "acme/agent-tools",
			Selector: Selector{Kind: SelectorRef, Value: "main"}}}}, Targets: map[string][]string{"codex": {"review"}}}
	lock := Lock{LockVersion: LockVersion, ManifestDigest: mustDigest(t, requirements)}
	pkg := LockedPackage{Alias: "review", SourceKey: "github:acme/agent-tools:", DigestAlgorithm: TreeDigestAlgorithm,
		TreeDigest: "sha256:" + strings.Repeat("a", 64), ManifestDigest: "sha256:" + strings.Repeat("b", 64), Targets: []string{"codex"},
		Source: LockedSource{Git: &LockedGitSource{Repository: "acme/agent-tools", Commit: strings.Repeat("1", 40),
			Requested: requirements.Dependencies["review"].Git.Selector}}}
	lock.Packages = []LockedPackage{pkg}
	plan, err := (Coordinator{}).PrepareSync(requirements, lock, []BindingObservation{{Alias: "review", ClientID: "codex",
		SourceKey: pkg.SourceKey, PackageDigest: pkg.TreeDigest, Revision: pkg.Source.Git.Commit, Managed: true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Bindings) != 1 || plan.Bindings[0].Action != ActionUnchanged {
		t.Fatalf("unchanged action = %+v", plan.Bindings)
	}
	plan, err = (Coordinator{}).PrepareSync(requirements, lock, []BindingObservation{{Alias: "review", ClientID: "codex", SourceKey: pkg.SourceKey, PackageDigest: pkg.TreeDigest, Revision: pkg.Source.Git.Commit, Managed: true, Incomplete: true}})
	if err != nil || len(plan.Bindings) != 1 || plan.Bindings[0].Action != ActionUpdate {
		t.Fatalf("incomplete lifecycle must resume at same pin: %+v %v", plan, err)
	}
	_, err = (Coordinator{}).PrepareSync(requirements, lock, []BindingObservation{{Alias: "review", ClientID: "codex",
		SourceKey: "github:other/repository:", PackageDigest: pkg.TreeDigest, Revision: pkg.Source.Git.Commit, Managed: true}})
	if !errors.Is(err, ErrSourceChange) {
		t.Fatalf("source change was not refused: %v", err)
	}
}

type countingResolver struct{ calls int }

func (resolver *countingResolver) Resolve(_ context.Context, requirement Requirement, _ *LockedPackage, _ bool) (LockedPackage, error) {
	resolver.calls++
	key, _ := requirement.SourceKey()
	return LockedPackage{Alias: requirement.Alias, SourceKey: key, DigestAlgorithm: TreeDigestAlgorithm,
		TreeDigest: "sha256:" + strings.Repeat("a", 64), ManifestDigest: "sha256:" + strings.Repeat("b", 64),
		Source: LockedSource{Git: &LockedGitSource{Repository: requirement.Git.Repository, Requested: requirement.Git.Selector,
			Commit: strings.Repeat("1", 40)}}}, nil
}

func TestTargetOnlyManifestChangeRetainsPinWithoutResolverLookup(t *testing.T) {
	requirements := Requirements{SchemaVersion: ManifestSchemaVersion,
		Dependencies: map[string]Requirement{"review": {Alias: "review", Git: &GitSource{Repository: "acme/agent-tools",
			Selector: Selector{Kind: SelectorRef, Value: "main"}}}}, Targets: map[string][]string{"codex": {"review"}, "claude": {"review"}}}
	oldTargets := requirements
	oldTargets.Targets = map[string][]string{"codex": {"review"}}
	previous := Lock{LockVersion: LockVersion, ManifestDigest: mustDigest(t, oldTargets), Packages: []LockedPackage{{
		Alias: "review", SourceKey: "github:acme/agent-tools:", DigestAlgorithm: TreeDigestAlgorithm,
		TreeDigest: "sha256:" + strings.Repeat("a", 64), ManifestDigest: "sha256:" + strings.Repeat("b", 64),
		Targets: []string{"codex"}, Source: LockedSource{Git: &LockedGitSource{Repository: "acme/agent-tools",
			Requested: requirements.Dependencies["review"].Git.Selector, Commit: strings.Repeat("1", 40)}}}}}
	resolver := &countingResolver{}
	resolved, err := (Coordinator{Resolver: resolver}).Resolve(context.Background(), requirements, previous, "")
	if err != nil {
		t.Fatal(err)
	}
	if resolver.calls != 0 || len(resolved.Packages) != 1 || len(resolved.Packages[0].Targets) != 2 {
		t.Fatalf("target-only change performed lookup or lost targets: calls=%d lock=%+v", resolver.calls, resolved)
	}
}

func mustDigest(t *testing.T, requirements Requirements) string {
	t.Helper()
	digest, err := ManifestDigest(requirements)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func TestUpdateRefCannotRefreshExactPinsOrUnrelatedAlias(t *testing.T) {
	requirements := Requirements{SchemaVersion: ManifestSchemaVersion, Dependencies: map[string]Requirement{
		"one": {Alias: "one", Git: &GitSource{Repository: "acme/one", Selector: Selector{Kind: SelectorRef, Value: "main"}}},
		"two": {Alias: "two", Git: &GitSource{Repository: "acme/two", Selector: Selector{Kind: SelectorRef, Value: "main"}}},
	}, Targets: map[string][]string{"codex": {"one", "two"}}}
	resolver := &countingResolver{}
	coordinator := Coordinator{Resolver: resolver}
	old, err := coordinator.Resolve(context.Background(), requirements, Lock{}, "")
	if err != nil {
		t.Fatal(err)
	}
	resolver.calls = 0
	updated, err := coordinator.Resolve(context.Background(), requirements, old, "one")
	if err != nil {
		t.Fatal(err)
	}
	if resolver.calls != 1 || updated.Packages[1].TreeDigest != old.Packages[1].TreeDigest {
		t.Fatal("targeted update touched unrelated alias")
	}
	requirements.Dependencies["one"].Git.Selector = Selector{Kind: SelectorCommit, Value: strings.Repeat("1", 40)}
	resolver.calls = 0
	if _, err := coordinator.Resolve(context.Background(), requirements, Lock{}, "one"); err == nil || resolver.calls != 0 {
		t.Fatal("exact pin update acquired bytes")
	}
}
