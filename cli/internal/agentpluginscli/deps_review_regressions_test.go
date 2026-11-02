package agentpluginscli

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func dependencyReviewFixture(t *testing.T) (freshRestoreEnvironment, string, *fixedDirectoryClient) {
	t.Helper()
	repo := t.TempDir()
	dependencyGit(t, repo, "init", "-b", "main")
	directory := &fixedDirectoryClient{bundle: directoryv1.VerifiedBundle{Digest: "sha256:" + strings.Repeat("a", 64), Snapshot: domain.DirectorySnapshot{SnapshotSchemaVersion: 1, Sequence: 1, SourceCommit: strings.Repeat("a", 40)}}}
	return newFreshRestoreEnvironment(t, repo, directory), repo, directory
}

// Each refusal is bound to the host. MCP identity is deliberately absent from
// catalog prechecks and becomes known only after the authentic bytes are loaded.
func TestDependencySelectionSkipsHostAndLoadedMCPIneligibility(t *testing.T) {
	for _, dependency := range []string{"", "go"} {
		t.Run("runtime dependency="+dependency, func(t *testing.T) {
			fixture, repo, directory := dependencyReviewFixture(t)
			first := freshRestorePackage(t, fixture, repo, "a", "1.1.0", "a-1.1.0", "", 2, true)
			if dependency != "" {
				first = dependencyReviewReviseRelease(t, fixture, repo, first, func(root string) {
					dependencyWrite(t, filepath.Join(root, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"a":{"type":"stdio","command":"go","args":["version"]}}}`)
				})
			}
			safe := freshRestorePackage(t, fixture, repo, "a", "1.0.0", "a-1.0.0", "", 1, true)
			failure := intendedTrustedDirectoryEvidence(domain.DirectoryEvidence{ID: "host-runtime-failure", DistributionID: "acme/a", ReleaseSequence: first.Sequence, PackageTreeDigest: first.TreeDigest, Level: "runtime", Outcome: "failed", Client: domain.ClientCodex, InstallerVersion: fixture.app.Version, OS: runtime.GOOS, Architecture: runtime.GOARCH, DependencyIdentity: dependency})
			failure.ClientVersion = "" // Candidate filtering has no detected client version.
			directory.bundle.Snapshot.Evidence = append(directory.bundle.Snapshot.Evidence, failure)
			policy := &directory.bundle.Snapshot.Distributions[0].ReleasePolicies[0]
			policy.CurrentEvidence = append(policy.CurrentEvidence, failure.ID)
			mp, _ := fixture.app.dependencyPaths()
			dependencyWrite(t, mp, "schema_version = 1\n[dependencies]\na = \"^1.0\"\n[targets]\ncodex = [\"a\"]\n")
			requirement := environment.Requirement{Alias: "a", Registry: &environment.RegistrySource{Distribution: "acme/a", Selector: environment.Selector{Kind: environment.SelectorRange, Value: "^1.0"}}}
			candidates, err := (dependencyResolver{app: fixture.app}).Candidates(context.Background(), requirement, []string{"codex"})
			wantCandidates := 1
			if dependency != "" {
				wantCandidates = 2
			}
			if err != nil || len(candidates) != wantCandidates {
				t.Fatalf("host precheck returned wrong candidates: %+v %v", candidates, err)
			}
			dependencyRun(t, fixture.cliFixture, "deps", "lock", "--format", "json")
			locked := freshRestoreReadLock(t, fixture.cliFixture)
			if len(locked.Packages) != 1 || locked.Packages[0].Version != safe.PackageVersion || locked.Packages[0].TreeDigest != safe.TreeDigest || locked.Packages[0].Source.Registry.Commit != safe.PackageSource.Revision {
				t.Fatalf("selection failed to fall back from ineligible highest: %+v", locked)
			}
			fetchedFirst := false
			for _, fetch := range fixture.source.verified {
				fetchedFirst = fetchedFirst || fetch.commit == first.PackageSource.Revision
			}
			if fetchedFirst != (dependency != "") {
				t.Fatalf("host-known refusal must precede fetch; MCP refusal must follow fetch: %+v", fixture.source.verified)
			}
			if _, err := fixture.app.dependencyStore().Get(context.Background(), domain.TreeDigestAlgorithm, first.TreeDigest); err == nil {
				t.Fatal("ineligible candidate was published in the store")
			}
		})
	}
}

// Rebinding these fixture metadata fields follows a new real commit. It never
// supplies different bytes under an existing immutable source identity.
func dependencyReviewReviseRelease(t *testing.T, fixture freshRestoreEnvironment, repo string, release domain.DirectoryRelease, edit func(string)) domain.DirectoryRelease {
	t.Helper()
	root := filepath.Join(repo, release.PackageSource.Path)
	edit(root)
	dependencyGit(t, repo, "add", ".")
	dependencyGit(t, repo, "commit", "-m", "test: revise disposable review fixture")
	command := exec.CommandContext(context.Background(), "git", "rev-parse", "HEAD")
	command.Dir = repo
	head, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := fixture.app.acquireLocal(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	release.PackageSource.Revision = strings.TrimSpace(string(head))
	release.TreeDigest, release.ManifestDigest = loaded.envelope.TreeDigest, loaded.envelope.ManifestDigest
	if loaded.envelope.MCP.Present {
		release.Components = []string{"skills", "mcp"}
	}
	if err := loaded.cleanup(); err != nil {
		t.Fatal(err)
	}
	snapshot := &fixture.app.DirectoryClient.(*fixedDirectoryClient).bundle.Snapshot
	for i := range snapshot.Distributions {
		distribution := &snapshot.Distributions[i]
		if distribution.ID != "acme/"+release.ManifestName {
			continue
		}
		for j := range distribution.Releases {
			if distribution.Releases[j].Sequence == release.Sequence {
				distribution.Releases[j] = release
			}
		}
	}
	for i := range snapshot.Evidence {
		evidence := &snapshot.Evidence[i]
		if evidence.DistributionID == "acme/"+release.ManifestName && evidence.ReleaseSequence == release.Sequence {
			evidence.PackageTreeDigest = release.TreeDigest
		}
	}
	return release
}

func TestDependencyLateTargetUnionRetainsLoadedMCPIdentity(t *testing.T) {
	fixture, repo, directory := dependencyReviewFixture(t)
	first := freshRestorePackage(t, fixture, repo, "c", "1.1.0", "c-1.1.0", "", 2, true)
	first = dependencyReviewReviseRelease(t, fixture, repo, first, func(root string) {
		dependencyWrite(t, filepath.Join(root, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"c":{"type":"stdio","command":"go","args":["version"]}}}`)
	})
	safe := freshRestorePackage(t, fixture, repo, "c", "1.0.0", "c-1.0.0", "", 1, true)
	freshRestorePackage(t, fixture, repo, "z", "1.0.0", "z-1.0.0", "^1.0", 1, true)
	snapshot := &directory.bundle.Snapshot
	for i := range snapshot.Distributions {
		distribution := &snapshot.Distributions[i]
		for j := range distribution.ReleasePolicies {
			policy := &distribution.ReleasePolicies[j]
			policy.Targets = append(policy.Targets, domain.DirectoryTarget{Client: domain.ClientClaude, Scopes: []domain.InstallScope{domain.ScopeUser}, Delivery: "managed", Authentication: domain.AuthenticationRequirementNotRequired})
			for _, evidence := range snapshot.Evidence {
				if evidence.DistributionID == distribution.ID && evidence.ReleaseSequence == policy.ReleaseSequence && evidence.Client == domain.ClientCodex {
					evidence.ID += "/claude"
					evidence.Client = domain.ClientClaude
					snapshot.Evidence = append(snapshot.Evidence, evidence)
					policy.CurrentEvidence = append(policy.CurrentEvidence, evidence.ID)
				}
			}
		}
	}
	failure := intendedTrustedDirectoryEvidence(domain.DirectoryEvidence{ID: "claude-go-failure", DistributionID: "acme/c", ReleaseSequence: first.Sequence, PackageTreeDigest: first.TreeDigest, Level: "runtime", Outcome: "failed", Client: domain.ClientClaude, InstallerVersion: fixture.app.Version, DependencyIdentity: "go"})
	failure.ClientVersion = ""
	snapshot.Evidence = append(snapshot.Evidence, failure)
	snapshot.Distributions[0].ReleasePolicies[0].CurrentEvidence = append(snapshot.Distributions[0].ReleasePolicies[0].CurrentEvidence, failure.ID)
	mp, _ := fixture.app.dependencyPaths()
	dependencyWrite(t, mp, "schema_version = 1\n[dependencies]\nc = \"^1.0\"\nz = \"1.0.0\"\n[targets]\ncodex = [\"c\"]\nclaude = [\"z\"]\n")
	dependencyRun(t, fixture.cliFixture, "deps", "lock", "--format", "json")
	locked := freshRestoreReadLock(t, fixture.cliFixture)
	pin, ok := locked.Package("c")
	if !ok || len(locked.Packages) != 2 || pin.Version != safe.PackageVersion || pin.TreeDigest != safe.TreeDigest || !reflect.DeepEqual(pin.Targets, []string{"claude", "codex"}) {
		t.Fatalf("late target union accepted executable-specific failed evidence: %+v", locked)
	}
	fetchedFirst := false
	for _, fetch := range fixture.source.verified {
		fetchedFirst = fetchedFirst || fetch.commit == first.PackageSource.Revision
	}
	if !fetchedFirst {
		t.Fatal("late target test did not acquire the conflicting highest candidate")
	}
	firstPin := pin
	firstPin.Version, firstPin.TreeDigest, firstPin.ManifestDigest = first.PackageVersion, first.TreeDigest, first.ManifestDigest
	source := *pin.Source.Registry
	firstPin.Source.Registry = &source
	source.ReleaseSequence, source.Commit, source.Path = first.Sequence, first.PackageSource.Revision, first.PackageSource.Path
	eligible, err := (dependencyResolver{app: fixture.app}).Eligible(context.Background(), firstPin, []string{"claude", "codex"})
	if err != nil || eligible {
		t.Fatalf("known stored executable identity was discarded: eligible=%v error=%v", eligible, err)
	}
}

func TestSavedAddHydratesUnselectedLegacyPins(t *testing.T) {
	for _, source := range []string{"git", "registry"} {
		t.Run(source, func(t *testing.T) {
			fixture, repo, _ := dependencyReviewFixture(t)
			release := freshRestorePackage(t, fixture, repo, "a", "1.0.0", "a-1.0.0", "", 1, true)
			mp, _ := fixture.app.dependencyPaths()
			intent := "registry = \"uap\"\ndistribution = \"acme/a\"\nrelease = \"1\"\n"
			if source == "git" {
				intent = fmt.Sprintf("git = \"acme/original\"\ncommit = %q\npath = %q\n", release.PackageSource.Revision, release.PackageSource.Path)
			}
			dependencyWrite(t, mp, "schema_version = 1\n[dependencies.a]\n"+intent+"[targets]\ncodex = [\"a\"]\n")
			dependencyRun(t, fixture.cliFixture, "install", "--yes", "--format", "json")
			mutateReconciliationLock(t, fixture.cliFixture, func(lock *environment.Lock) { lock.Packages[0].Version = "" })
			previous := freshRestoreReadLock(t, fixture.cliFixture).Packages[0]
			binding := dependencyBinding(t, fixture.cliFixture)
			projection := freshRestoreFiles(t, binding.TargetPath)
			other := freshRestorePackage(t, fixture, repo, "other", "1.0.0", "other-1.0.0", "", 1, false)
			dependencyRun(t, fixture.cliFixture, "add", githubIdentity("acme/other", other.PackageSource.Revision, other.PackageSource.Path), "--alias", "other", "--target", "codex", "--format", "json")
			locked := freshRestoreReadLock(t, fixture.cliFixture)
			hydrated, ok := locked.Package("a")
			previous.Version = "1.0.0"
			if !ok || !environment.SamePin(previous, hydrated) || len(locked.Packages) != 2 || !reflect.DeepEqual(projection, freshRestoreFiles(t, binding.TargetPath)) {
				t.Fatalf("unrelated saved add changed the legacy pin or its projection: %+v", locked)
			}
		})
	}
}

func TestUnselectedPinsPermitOnlyLegacyVersionHydration(t *testing.T) {
	for _, source := range []string{"git", "registry"} {
		for _, change := range []string{"hydrate", "known version", "erase version", "commit", "release", "tree", "manifest"} {
			if source == "git" && change == "release" {
				continue
			}
			t.Run(source+"/"+change, func(t *testing.T) {
				old := environment.LockedPackage{Alias: "a", TreeDigest: "sha256:" + strings.Repeat("a", 64), ManifestDigest: "sha256:" + strings.Repeat("b", 64)}
				if source == "git" {
					old.Source.Git = &environment.LockedGitSource{Repository: "acme/a", Commit: strings.Repeat("1", 40)}
				} else {
					old.Source.Registry = &environment.LockedRegistrySource{Distribution: "acme/a", Commit: strings.Repeat("1", 40), ReleaseSequence: 1}
				}
				if change == "known version" || change == "erase version" {
					old.Version = "1.0.0"
				}
				next := old
				next.Version = "1.1.0"
				switch change {
				case "erase version":
					next.Version = ""
				case "commit", "release":
					if source == "git" {
						copied := *old.Source.Git
						next.Source.Git = &copied
						copied.Commit = strings.Repeat("2", 40)
					} else {
						copied := *old.Source.Registry
						next.Source.Registry = &copied
						if change == "commit" {
							copied.Commit = strings.Repeat("2", 40)
						} else {
							copied.ReleaseSequence = 2
						}
					}
				case "tree":
					next.TreeDigest = "sha256:" + strings.Repeat("c", 64)
				case "manifest":
					next.ManifestDigest = "sha256:" + strings.Repeat("c", 64)
				}
				err := verifyUnselectedPins(environment.Lock{Packages: []environment.LockedPackage{old}}, environment.Lock{Packages: []environment.LockedPackage{next}}, []string{"other"})
				if (err == nil) != (change == "hydrate") {
					t.Fatalf("immutable change %q accepted=%v: %v", change, err == nil, err)
				}
			})
		}
	}
}

func TestSavedExactRegistryCycleUpdateRebuildsValidatedEdges(t *testing.T) {
	fixture, repo, _ := dependencyReviewFixture(t)
	freshRestorePackage(t, fixture, repo, "c", "1.0.0", "c-1.0.0", "^1.0", 1, true)
	mp, lp := fixture.app.dependencyPaths()
	dependencyWrite(t, mp, "schema_version = 1\n[dependencies.c]\nregistry = \"uap\"\ndistribution = \"acme/c\"\nrelease = \"1\"\n[targets]\ncodex = [\"c\"]\n")
	dependencyRun(t, fixture.cliFixture, "install", "--yes", "--format", "json")
	originalLock := dependencyRead(t, lp)
	freshRestorePackage(t, fixture, repo, "c", "2.0.0", "c-2.0.0", "^2.0", 2, true)
	// A genuinely invalid input graph must still refuse before update acquisition.
	mutateReconciliationLock(t, fixture.cliFixture, func(lock *environment.Lock) { lock.Packages[0].RequiredBy[0].Selector.Value = "^9.0" })
	before := freshRestoreFiles(t, mp, lp, fixture.store.Path)
	downloads := fixture.source.downloads
	if _, _, err := fixture.execute(false, "update", "c", "--format", "json"); err == nil || !strings.Contains(err.Error(), "does not satisfy consumer") {
		t.Fatalf("invalid original cycle was not rejected: %v", err)
	}
	if downloads != fixture.source.downloads || !reflect.DeepEqual(before, freshRestoreFiles(t, mp, lp, fixture.store.Path)) {
		t.Fatal("invalid original graph changed state or acquired an update")
	}
	dependencyWrite(t, lp, originalLock)
	dependencyRun(t, fixture.cliFixture, "update", "c", "--format", "json")
	locked := freshRestoreReadLock(t, fixture.cliFixture)
	if len(locked.Packages) != 1 || locked.Packages[0].Version != "2.0.0" || locked.Packages[0].Source.Registry.ReleaseSequence != 2 || !reflect.DeepEqual(locked.Packages[0].RequiredBy, []environment.RequiredBy{{Alias: "c", Selector: environment.Selector{Kind: environment.SelectorRange, Value: "^2.0"}}}) {
		t.Fatalf("new exact release inherited stale cycle edges: %+v", locked)
	}
}
