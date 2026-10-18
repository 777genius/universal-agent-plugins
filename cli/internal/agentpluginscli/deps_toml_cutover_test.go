package agentpluginscli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// The CLI exercises mixed typed edges with real immutable Git/store paths.
func TestDependencyTOMLMixedSourceLifecycleAndRestore(t *testing.T) {
	repo := t.TempDir()
	dependencyGit(t, repo, "init", "-b", "main")
	directory := &fixedDirectoryClient{bundle: directoryv1.VerifiedBundle{Digest: "sha256:" + strings.Repeat("a", 64), Snapshot: domain.DirectorySnapshot{SnapshotSchemaVersion: 1, Sequence: 1, SourceCommit: strings.Repeat("a", 40), Products: []domain.DirectoryProduct{}, Distributions: []domain.DirectoryDistribution{}, Evidence: []domain.DirectoryEvidence{}, Revocations: []domain.DirectoryRevocation{}}}}
	original := newFreshRestoreEnvironment(t, repo, directory)
	freshRestorePackage(t, original, repo, "c", "1.2.0", "c", "", 1, true)
	if err := os.MkdirAll(filepath.Join(repo, "b"), 0700); err != nil {
		t.Fatal(err)
	}
	dependencyWrite(t, filepath.Join(repo, "b", "uap.toml"), "schema_version = 1\n[dependencies.c]\nregistry = \"uap\"\ndistribution = \"acme/c\"\nrelease = 1\n[targets]\nauthor-only = [\"unknown-local-entry\"]\n")
	freshRestorePackage(t, original, repo, "b", "1.0.0", "b", "", 1, false)
	if err := os.MkdirAll(filepath.Join(repo, "a"), 0700); err != nil {
		t.Fatal(err)
	}
	dependencyWrite(t, filepath.Join(repo, "a", "uap.toml"), "schema_version = 1\n[dependencies.b]\ngit = \"acme/fresh-restore\"\npath = \"b\"\nref = \"main\"\n")
	freshRestorePackage(t, original, repo, "a", "1.0.0", "a", "", 1, true)
	freshRestorePackage(t, original, repo, "d", "1.0.0", "d", "^1.2", 1, true)
	mp, lp := original.app.dependencyPaths()
	dependencyWrite(t, mp, "schema_version = 1\n[dependencies]\n\"acme/a\" = \"^1.0\"\nd = \"^1.0\"\n[targets]\ncodex = [\"acme/a\", \"d\"]\n")
	dependencyRun(t, original.cliFixture, "install", "--yes", "--format", "json")
	lock := freshRestoreReadLock(t, original.cliFixture)
	if len(lock.Packages) != 4 {
		t.Fatalf("closure: %+v", lock.Packages)
	}
	b, ok := lock.Package("b")
	if !ok || b.Source.Git == nil || b.Source.Git.Requested.Kind != environment.SelectorRef {
		t.Fatalf("Git child: %+v", b)
	}
	c, ok := lock.Package("c")
	if !ok || len(c.RequiredBy) != 2 || len(c.Targets) != 1 || c.Targets[0] != "codex" {
		t.Fatalf("shared exact/SemVer child: %+v", c)
	}
	treeJSON := dependencyRun(t, original.cliFixture, "deps", "tree", "--format", "json")
	var tree dependencyTree
	if err := json.Unmarshal([]byte(treeJSON), &tree); err != nil {
		t.Fatal(err)
	}
	if len(tree.RootMappings) != 1 || tree.RootMappings[0].Key != "acme/a" || tree.RootMappings[0].Alias != "a" {
		t.Fatalf("root mapping: %s", treeJSON)
	}

	// Offline short-name lookup uses locked source facts even if the stable alias
	// differs; a basename collision must refuse instead of choosing a candidate.
	pinA, _ := lock.Package("a")
	pinA.Alias, pinA.RequiredBy = "previous-owner", nil
	short := environment.Requirements{SchemaVersion: 1, Dependencies: map[string]environment.Requirement{"a": {Alias: "a", Registry: &environment.RegistrySource{Distribution: "a", Selector: pinA.Source.Registry.Requested}}}, Targets: map[string][]string{"codex": {"a"}}}
	canonical := cloneSavedRequirements(short)
	sourceCopy := *canonical.Dependencies["a"].Registry
	sourceCopy.Distribution = "acme/a"
	entry := canonical.Dependencies["a"]
	entry.Registry = &sourceCopy
	canonical.Dependencies["a"] = entry
	offline := environment.Lock{LockVersion: environment.LockVersion, Packages: []environment.LockedPackage{pinA}}
	offline.ManifestDigest, _ = environment.ManifestDigest(canonical)
	body, _ := json.Marshal(offline)
	if _, err := decodeDependencyLockBytes(body, cloneSavedRequirements(short), false); err != nil {
		t.Fatalf("offline alias mismatch: %v", err)
	}
	other := pinA
	other.Alias = "other-owner"
	other.SourceKey = "directory:other/a"
	registry := *pinA.Source.Registry
	registry.Distribution = "other/a"
	registry.DistributionID = "other/a"
	registry.ProductID = "other-product"
	other.Source.Registry = &registry
	offline.Packages = append(offline.Packages, other)
	body, _ = json.Marshal(offline)
	if _, err := decodeDependencyLockBytes(body, cloneSavedRequirements(short), false); err == nil || !strings.Contains(err.Error(), "unambiguously") {
		t.Fatalf("ambiguous offline mapping accepted: %v", err)
	}

	downloads, lookups := original.source.downloads, original.source.lookups
	dependencyRun(t, original.cliFixture, "install", "--locked", "--format", "json")
	if downloads != original.source.downloads || lookups != original.source.lookups {
		t.Fatal("repeat acquired mutable source")
	}
	fresh := newFreshRestoreEnvironment(t, repo, directory)
	freshRestoreCopyAgreement(t, fresh, dependencyRead(t, mp), dependencyRead(t, lp))
	dependencyRun(t, fresh.cliFixture, "install", "--locked", "--yes", "--format", "json")
	if fresh.source.lookups != 0 || len(fresh.source.verified) != 4 {
		t.Fatalf("cold restore: lookups=%d exact fetches=%+v", fresh.source.lookups, fresh.source.verified)
	}
	seen := map[freshRestoreFetch]bool{}
	for _, fetch := range fresh.source.verified {
		if seen[fetch] {
			t.Fatalf("duplicate exact fetch: %+v", fetch)
		}
		seen[fetch] = true
	}

	// A late consumer target reaches the already pinned exact C through the
	// existing provider. The candidate must fail before portable intent or Apply.
	claude := fixtureClient(t, domain.ClientClaude)
	claude.ExecutablePath = filepath.Join(t.TempDir(), "claude")
	original.app.Detector = staticDetector{clients: []domain.DetectedClient{original.client, claude}}
	beforeManifest, beforeLock, beforeState := dependencyRead(t, mp), dependencyRead(t, lp), dependencyRead(t, original.store.Path)
	nativeCalls := original.runner.calls
	_, _, err := original.execute(false, "add", "acme/c", "--target", "claude", "--format", "json")
	if err == nil || !strings.Contains(err.Error(), "dependency conflict for directory:acme/c") {
		t.Fatalf("late exact child target must refuse: %v", err)
	}
	if dependencyRead(t, mp) != beforeManifest || dependencyRead(t, lp) != beforeLock || dependencyRead(t, original.store.Path) != beforeState || original.runner.calls != nativeCalls {
		t.Fatal("failed target union changed intent or applied native state")
	}

	// Once the same immutable release qualifies the expanded union, both client
	// bindings install from that pin through the existing native fixture runner.
	claude.Version = original.client.Version
	original.app.Detector = staticDetector{clients: []domain.DetectedClient{original.client, claude}}
	original.runner.claude.root = claude.ConfigRoot
	snapshot := &directory.bundle.Snapshot
	evidence := intendedTrustedDirectoryEvidence(domain.DirectoryEvidence{ID: "passed/mixed/c/claude", DistributionID: "acme/c", ReleaseSequence: 1, PackageTreeDigest: c.TreeDigest, Level: "materialization", Outcome: "passed", Client: domain.ClientClaude, ClientVersion: claude.Version, InstallerVersion: original.app.Version, OS: runtime.GOOS, Architecture: runtime.GOARCH})
	snapshot.Evidence = append(snapshot.Evidence, evidence)
	for i := range snapshot.Distributions {
		if snapshot.Distributions[i].ID == "acme/c" {
			policy := &snapshot.Distributions[i].ReleasePolicies[0]
			policy.CurrentEvidence = append(policy.CurrentEvidence, evidence.ID)
			policy.Targets = append(policy.Targets, domain.DirectoryTarget{Client: domain.ClientClaude, Scopes: []domain.InstallScope{domain.ScopeUser}, Delivery: "managed", Authentication: domain.AuthenticationRequirementNotRequired})
		}
	}
	dependencyRun(t, original.cliFixture, "add", "acme/c", "--target", "claude", "--format", "json")
	union := freshRestoreReadLock(t, original.cliFixture)
	unionC, _ := union.Package("c")
	unionState, err := original.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	unionBindings := 0
	for _, r := range unionState.EnvironmentReceipts {
		if r.SourceKey == c.SourceKey {
			unionBindings++
		}
	}
	if !reflect.DeepEqual(unionC.Targets, []string{"claude", "codex"}) || unionBindings != 2 {
		t.Fatalf("expanded union incomplete: %+v receipts=%d", unionC, unionBindings)
	}
	dependencyRun(t, original.cliFixture, "remove", "c", "--target", "claude", "--external-uninstalled", "--format", "json")

	// Promoting a retained source to a direct root, then removing that declaration,
	// preserves actual installer identity and receipts, in addition to its pin.
	stateBefore, err := original.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	receipts := func(state domain.StateFileV2) []domain.EnvironmentReceipt {
		out := []domain.EnvironmentReceipt{}
		for _, r := range state.EnvironmentReceipts {
			if r.SourceKey == c.SourceKey {
				out = append(out, r)
			}
		}
		return out
	}
	ownerBefore := receipts(stateBefore)
	dependencyRun(t, original.cliFixture, "add", "acme/c", "--target", "codex", "--format", "json")
	promoted := freshRestoreReadLock(t, original.cliFixture)
	promotedC, _ := promoted.Package("c")
	promotedState, err := original.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ownerBefore, receipts(promotedState)) || promotedC.TreeDigest != c.TreeDigest || promotedC.Alias != c.Alias || len(promotedC.RequiredBy) != 2 {
		t.Fatal("direct promotion changed existing pin or receipt identity")
	}
	dependencyRun(t, original.cliFixture, "remove", "c", "--format", "json")
	demoted := freshRestoreReadLock(t, original.cliFixture)
	demotedC, _ := demoted.Package("c")
	demotedState, err := original.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ownerBefore, receipts(demotedState)) || demotedC.TreeDigest != c.TreeDigest || len(demotedC.RequiredBy) != 2 {
		t.Fatal("direct removal changed retained owner identity")
	}
	// Lookup revealing a short/canonical duplicate refuses before lock publication.
	duplicate := newFreshRestoreEnvironment(t, repo, directory)
	duplicateMP, duplicateLP := duplicate.app.dependencyPaths()
	dependencyWrite(t, duplicateMP, "schema_version = 1\n[dependencies]\na = \"^1.0\"\n\"acme/a\" = \"^1.0\"\n[targets]\ncodex = [\"a\", \"acme/a\"]\n")
	if _, _, err := duplicate.execute(false, "deps", "lock", "--format", "json"); err == nil {
		t.Fatal("postlookup duplicate accepted")
	}
	if _, err := os.Stat(duplicateLP); !os.IsNotExist(err) {
		t.Fatalf("duplicate published lock: %v", err)
	}

	dependencyRun(t, original.cliFixture, "remove", "acme/a", "--external-uninstalled", "--format", "json")
	retained := freshRestoreReadLock(t, original.cliFixture)
	if len(retained.Packages) != 2 {
		t.Fatalf("retained graph: %+v", retained.Packages)
	}
	nextC, _ := retained.Package("c")
	if nextC.TreeDigest != c.TreeDigest || nextC.Alias != c.Alias || len(nextC.RequiredBy) != 1 {
		t.Fatalf("shared pin changed: %+v", nextC)
	}
}
