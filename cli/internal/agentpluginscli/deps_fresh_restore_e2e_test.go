package agentpluginscli

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

type freshRestoreFetch struct{ repository, commit, path, digest string }

type freshRestoreSource struct {
	*dependencyTestSource
	verified []freshRestoreFetch
}

func (source *freshRestoreSource) AcquireGitHubVerified(ctx context.Context, repository, commit, path, digest string) (domain.PackageSnapshot, error) {
	source.verified = append(source.verified, freshRestoreFetch{repository, commit, path, digest})
	return source.dependencyTestSource.AcquireGitHubVerified(ctx, repository, commit, path, digest)
}

type freshRestoreEnvironment struct {
	cliFixture
	client domain.DetectedClient
	source *freshRestoreSource
	runner *dependencyFixtureRunner
}

func newFreshRestoreEnvironment(t *testing.T, repo string, directory *fixedDirectoryClient) freshRestoreEnvironment {
	t.Helper()
	client := fixtureClient(t, domain.ClientCodex)
	client.ExecutablePath, client.Version = filepath.Join(t.TempDir(), "codex"), "fixture-client"
	runner := &dependencyFixtureRunner{id: domain.ClientCodex, claude: dependencyClaudeListing{root: client.ConfigRoot}}
	f, source := newDependencyHarness(t, repo, []domain.DetectedClient{client}, installer.Config{Runner: runner})
	tracked := &freshRestoreSource{dependencyTestSource: source}
	f.app.SourceAcquirer, f.app.DirectoryClient = tracked, directory
	return freshRestoreEnvironment{f, client, tracked, runner}
}

// The Directory is a verified-input fixture. Package acquisition, immutable Git
// verification, store publication and installer transactions are real. Only the
// native Codex process responses are fixtures; no agent or MCP server is run.
func TestDependencyFreshRestoreFromPortableFiles(t *testing.T) {
	// A restore must survive the standard Windows ambient environment on any host.
	t.Setenv("ProgramFiles(x86)", "ignored-windows-installation-metadata")
	t.Setenv("CommonProgramFiles(x86)", "ignored-windows-installation-metadata")
	repo := t.TempDir()
	dependencyGit(t, repo, "init", "-b", "main")
	directory := &fixedDirectoryClient{bundle: directoryv1.VerifiedBundle{Digest: "sha256:" + strings.Repeat("a", 64), Snapshot: domain.DirectorySnapshot{
		SnapshotSchemaVersion: 1, Sequence: 1, SourceCommit: strings.Repeat("a", 40),
		Products: []domain.DirectoryProduct{}, Distributions: []domain.DirectoryDistribution{}, Evidence: []domain.DirectoryEvidence{}, Revocations: []domain.DirectoryRevocation{},
	}}}
	original := newFreshRestoreEnvironment(t, repo, directory)
	firstA := freshRestorePackage(t, original, repo, "a", "1.3.0", "a-1.3.0", "^1.2", 1, true)
	firstB := freshRestorePackage(t, original, repo, "b", "1.0.0", "b", "^1.2", 1, false)
	firstC := freshRestorePackage(t, original, repo, "c", "1.2.0", "c-1.2.0", "", 1, true)
	// The moving Git root resolves after the initial registry publications.
	firstB.PackageSource.Revision = firstC.PackageSource.Revision
	mp, lp := original.app.dependencyPaths()
	dependencyWrite(t, mp, "schema_version = 1\n# portable agreement only\n[dependencies]\na = \"^1.2\"\n[dependencies.b]\ngit = \"acme/fresh-restore\"\npath = \"b\"\nref = \"main\"\n[targets]\ncodex = [\"a\", \"b\"]\n")
	dependencyRun(t, original.cliFixture, "deps", "lock", "--format", "json")
	locked := freshRestoreReadLock(t, original.cliFixture)
	freshRestoreCheckGraph(t, locked, map[string]domain.DirectoryRelease{"a": firstA, "b": firstB, "c": firstC})
	dependencyRun(t, original.cliFixture, "install", "--locked", "--yes", "--format", "json")
	freshRestoreCheckInstalled(t, original, locked)
	manifestBytes, lockBytes := dependencyRead(t, mp), dependencyRead(t, lp)

	// Newer admissible releases and different bytes at Git/main must not move any
	// pin when the two portable files are replayed into a completely empty store.
	freshRestorePackage(t, original, repo, "b", "1.1.0", "b", "^1.2", 2, false)
	freshRestorePackage(t, original, repo, "a", "1.4.0", "a-1.4.0", "^1.2", 2, true)
	freshRestorePackage(t, original, repo, "c", "1.3.0", "c-1.3.0", "", 2, true)
	originalBefore := freshRestoreFiles(t, original.root, original.client.ConfigRoot)
	originalDownloads, originalLookups, originalMutations := original.source.downloads, original.source.lookups, original.runner.mutations

	fresh := newFreshRestoreEnvironment(t, repo, directory)
	if fresh.root == original.root || fresh.client.ConfigRoot == original.client.ConfigRoot || fresh.app.EnvironmentStoreRoot == original.app.EnvironmentStoreRoot || fresh.store.Path == original.store.Path || fresh.runner == original.runner || fresh.source.TempRoot == original.source.TempRoot {
		t.Fatal("restore reused the original environment, store, state or native fixture")
	}
	freshRestoreCopyAgreement(t, fresh, manifestBytes, lockBytes)
	freshMP, freshLP := fresh.app.dependencyPaths()
	agreementBefore := freshRestoreFiles(t, fresh.app.DependenciesRoot)
	dependencyRun(t, fresh.cliFixture, "install", "--locked", "--yes", "--format", "json")
	if fresh.source.lookups != 0 || fresh.source.downloads < len(locked.Packages) || fresh.source.downloads != len(fresh.source.verified) {
		t.Fatalf("cold restore did not use exclusively verified immutable downloads: %+v", fresh.source)
	}
	wantedFetches := map[freshRestoreFetch]bool{}
	for _, pin := range locked.Packages {
		repository, commit, path := firstB.PackageSource.Repository, firstB.PackageSource.Revision, firstB.PackageSource.Path
		if pin.Source.Registry != nil {
			repository, commit, path = pin.Source.Registry.Repository, pin.Source.Registry.Commit, pin.Source.Registry.Path
		}
		wantedFetches[freshRestoreFetch{repository, commit, path, pin.TreeDigest}] = false
	}
	for _, fetch := range fresh.source.verified {
		if _, wanted := wantedFetches[fetch]; !wanted {
			t.Fatalf("restore fetched outside the recorded immutable graph: %+v", fetch)
		}
		wantedFetches[fetch] = true
	}
	for fetch, downloaded := range wantedFetches {
		if !downloaded {
			t.Fatalf("restore reused unavailable local content instead of downloading %+v", fetch)
		}
	}
	freshRestoreCheckInstalled(t, fresh, locked)
	if !reflect.DeepEqual(locked, freshRestoreReadLock(t, fresh.cliFixture)) || !reflect.DeepEqual(agreementBefore, freshRestoreFiles(t, freshMP, freshLP)) {
		t.Fatal("locked restore rewrote the portable agreement or moved a graph pin")
	}
	var tree dependencyTree
	if err := json.Unmarshal([]byte(dependencyRun(t, fresh.cliFixture, "deps", "tree", "--format", "json")), &tree); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tree.Roots, []string{"a", "b"}) || !reflect.DeepEqual(tree.Packages, locked.Packages) {
		t.Fatalf("restored tree lost direct roots or shared dependency edges: %+v", tree)
	}
	beforeRepeat := freshRestoreFiles(t, fresh.root, fresh.client.ConfigRoot)
	downloads, mutations := fresh.source.downloads, fresh.runner.mutations
	dependencyRun(t, fresh.cliFixture, "install", "--locked", "--yes", "--format", "json")
	if fresh.source.downloads != downloads || fresh.source.lookups != 0 || fresh.runner.mutations != mutations || !reflect.DeepEqual(beforeRepeat, freshRestoreFiles(t, fresh.root, fresh.client.ConfigRoot)) {
		t.Fatal("second locked install downloaded, resolved, mutated native state or rewrote files")
	}
	t.Logf("cold restore: %d real verified Git acquisitions for 3 pinned packages; repeat: 0 acquisitions, 0 ref lookups, 0 native mutations", downloads)

	// Correct portable files cannot authorize falling back to current remote bytes
	// when a pinned commit is unavailable. This separate remote has none of the
	// original objects, and this third environment again has no local store/state.
	missingRepo := t.TempDir()
	dependencyGit(t, missingRepo, "init", "-b", "main")
	dependencyWrite(t, filepath.Join(missingRepo, "unrelated.txt"), "no recorded immutable package objects")
	dependencyGit(t, missingRepo, "add", ".")
	dependencyGit(t, missingRepo, "commit", "-m", "test: unrelated disposable restore remote")
	unavailable := newFreshRestoreEnvironment(t, missingRepo, directory)
	freshRestoreCopyAgreement(t, unavailable, manifestBytes, lockBytes)
	beforeRefusal := freshRestoreFiles(t, unavailable.root, unavailable.client.ConfigRoot)
	_, _, err := unavailable.execute(false, "install", "--locked", "--yes", "--format", "json")
	if err == nil || !strings.Contains(err.Error(), "pinned manifest") || !strings.Contains(err.Error(), "fetch immutable revision") {
		t.Fatalf("missing pinned content did not fail at immutable acquisition: %v", err)
	}
	if unavailable.source.downloads != 1 || unavailable.source.lookups != 0 || unavailable.runner.mutations != 0 || !reflect.DeepEqual(beforeRefusal, freshRestoreFiles(t, unavailable.root, unavailable.client.ConfigRoot)) {
		t.Fatal("unavailable immutable source refusal resolved a replacement, published content or changed local state")
	}
	freshRestoreAssertNoLocalState(t, unavailable)
	if !reflect.DeepEqual(originalBefore, freshRestoreFiles(t, original.root, original.client.ConfigRoot)) || original.source.downloads != originalDownloads || original.source.lookups != originalLookups || original.runner.mutations != originalMutations {
		t.Fatal("fresh restore or its refusal mutated the original environment")
	}
}

func freshRestorePackage(t *testing.T, fixture freshRestoreEnvironment, repo, alias, version, subpath, requirement string, sequence uint64, publish bool) domain.DirectoryRelease {
	t.Helper()
	root := filepath.Join(repo, subpath)
	if err := os.MkdirAll(filepath.Join(root, "skills", alias), 0700); err != nil {
		t.Fatal(err)
	}
	manifest := map[string]any{"$schema": domain.PluginSchemaV1, "name": alias, "version": version, "description": "Disposable fresh restore package " + alias}
	if requirement != "" {
		manifest["extensions"] = map[string]any{domain.ExtensionNamespace: map[string]any{"dependencies": map[string]string{"acme/c": requirement}}}
	}
	body, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	dependencyWrite(t, filepath.Join(root, "plugin.json"), string(body))
	dependencyWrite(t, filepath.Join(root, "skills", alias, "SKILL.md"), fmt.Sprintf("---\nname: %s\ndescription: Fresh restore fixture\n---\nRecorded package %s %s.\n", alias, alias, version))
	dependencyGit(t, repo, "add", ".")
	dependencyGit(t, repo, "commit", "-m", "test: fresh restore "+alias+" "+version)
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
	release := domain.DirectoryRelease{Sequence: sequence, PackageVersion: version, ManifestName: alias, AgentPluginsSchema: domain.PluginSchemaV1,
		PackageSource:       domain.DirectorySource{Repository: "acme/fresh-restore", Revision: strings.TrimSpace(string(head)), Path: subpath},
		TreeDigestAlgorithm: domain.TreeDigestAlgorithm, TreeDigest: loaded.envelope.TreeDigest, ManifestDigest: loaded.envelope.ManifestDigest,
		Components: []string{"skills"}, PublishedAt: "2026-10-18T00:00:00Z"}
	if err := loaded.cleanup(); err != nil {
		t.Fatal(err)
	}
	if !publish {
		return release
	}
	snapshot := &fixture.app.DirectoryClient.(*fixedDirectoryClient).bundle.Snapshot
	index := -1
	for i := range snapshot.Distributions {
		if snapshot.Distributions[i].ID == "acme/"+alias {
			index = i
		}
	}
	if index < 0 {
		snapshot.Products = append(snapshot.Products, domain.DirectoryProduct{SchemaVersion: 1, ID: alias, DisplayName: alias, Description: "Disposable restore package", ManifestName: alias, Aliases: []string{alias}, ReservedAliases: []string{alias}, Categories: []string{}, DefaultDistribution: "acme/" + alias, Distributions: []string{"acme/" + alias}, MinimumCapabilities: domain.DirectoryMinimumCapabilities{Skills: "optional", MCP: "optional"}})
		snapshot.Distributions = append(snapshot.Distributions, domain.DirectoryDistribution{SchemaVersion: 1, ID: "acme/" + alias, ProductID: alias, Kind: domain.DistributionUpstream, Status: domain.DistributionActive, Packager: "acme"})
		index = len(snapshot.Distributions) - 1
	}
	evidence := intendedTrustedDirectoryEvidence(domain.DirectoryEvidence{ID: fmt.Sprintf("passed/restore/%s/%d", alias, sequence), DistributionID: "acme/" + alias, ReleaseSequence: sequence, PackageTreeDigest: release.TreeDigest, Level: "materialization", Outcome: "passed", Client: domain.ClientCodex, ClientVersion: fixture.client.Version, InstallerVersion: fixture.app.Version, OS: runtime.GOOS, Architecture: runtime.GOARCH})
	snapshot.Evidence = append(snapshot.Evidence, evidence)
	distribution := &snapshot.Distributions[index]
	distribution.Releases = append(distribution.Releases, release)
	distribution.ReleasePolicies = append(distribution.ReleasePolicies, domain.DirectoryReleasePolicy{ReleaseSequence: sequence, Status: domain.ReleaseActive, MinimumInstallerVersion: "0.1.0", CurrentEvidence: []string{evidence.ID}, Targets: []domain.DirectoryTarget{{Client: domain.ClientCodex, Scopes: []domain.InstallScope{domain.ScopeUser}, Delivery: "managed", Authentication: domain.AuthenticationRequirementNotRequired}}})
	return release
}

func freshRestoreReadLock(t *testing.T, fixture cliFixture) environment.Lock {
	t.Helper()
	mp, lp := fixture.app.dependencyPaths()
	requirements, err := readDependencyManifest(mp)
	if err != nil {
		t.Fatal(err)
	}
	locked, err := readDependencyLock(lp, requirements)
	if err != nil {
		t.Fatal(err)
	}
	return locked
}

func freshRestoreCheckGraph(t *testing.T, locked environment.Lock, releases map[string]domain.DirectoryRelease) {
	t.Helper()
	if len(locked.Packages) != 3 {
		t.Fatalf("diamond must contain exactly 3 nodes: %+v", locked)
	}
	for _, pin := range locked.Packages {
		release, known := releases[pin.Alias]
		if !known || pin.Version != release.PackageVersion || pin.TreeDigest != release.TreeDigest || pin.ManifestDigest != release.ManifestDigest || lockedCommit(pin) != release.PackageSource.Revision || !reflect.DeepEqual(pin.Targets, []string{"codex"}) {
			t.Fatalf("incorrect immutable graph pin: %+v want %+v", pin, release)
		}
		if pin.Alias == "b" {
			if pin.Source.Git == nil || pin.Source.Git.Repository != release.PackageSource.Repository || pin.Source.Git.Path != "b" || pin.Source.Git.Requested != (environment.Selector{Kind: environment.SelectorRef, Value: "main"}) {
				t.Fatalf("Git root identity lost: %+v", pin)
			}
		} else if pin.Source.Registry == nil || pin.Source.Registry.ReleaseSequence != 1 || pin.Source.Registry.DistributionID != "acme/"+pin.Alias {
			t.Fatalf("registry publication identity lost: %+v", pin)
		}
		if pin.Alias == "c" {
			want := []environment.RequiredBy{{Alias: "a", Selector: environment.Selector{Kind: environment.SelectorRange, Value: "^1.2"}}, {Alias: "b", Selector: environment.Selector{Kind: environment.SelectorRange, Value: "^1.2"}}}
			if !reflect.DeepEqual(pin.RequiredBy, want) {
				t.Fatalf("shared dependency edges differ: %+v", pin)
			}
		} else if len(pin.RequiredBy) != 0 {
			t.Fatalf("direct root acquired a parent: %+v", pin)
		}
	}
}

func freshRestoreCopyAgreement(t *testing.T, fixture freshRestoreEnvironment, manifest, lock string) {
	t.Helper()
	freshRestoreAssertNoLocalState(t, fixture)
	entries, err := os.ReadDir(fixture.source.TempRoot)
	if err != nil || len(entries) != 0 {
		t.Fatalf("restore acquisition workspace is not empty: %v %v", entries, err)
	}
	mp, lp := fixture.app.dependencyPaths()
	dependencyWrite(t, mp, manifest)
	dependencyWrite(t, lp, lock)
	entries, err = os.ReadDir(fixture.app.DependenciesRoot)
	if err != nil || len(entries) != 2 || entries[0].Name() != "uap.lock" || entries[1].Name() != "uap.toml" {
		t.Fatalf("restore transferred more than the two portable files: %v %v", entries, err)
	}
	if fixture.source.downloads != 0 || fixture.source.lookups != 0 || fixture.runner.mutations != 0 {
		t.Fatal("cold harness acquired content or native state before restore")
	}
}

func freshRestoreAssertNoLocalState(t *testing.T, fixture freshRestoreEnvironment) {
	t.Helper()
	for _, path := range []string{fixture.store.Path, fixture.app.EnvironmentStoreRoot, fixture.app.ManagedRoot, fixture.operations, fixture.client.ConfigRoot, fixture.app.EnvironmentLockPath} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("restore environment already has local store/state/profile %s: %v", path, err)
		}
	}
}

func freshRestoreCheckInstalled(t *testing.T, fixture freshRestoreEnvironment, locked environment.Lock) {
	t.Helper()
	view, err := fixture.app.Installer.Inspect(context.Background())
	if err != nil || view.Recovery.Required || len(view.EnvironmentReceipts) != 3 || len(view.Installations) != 3 {
		t.Fatalf("restore did not establish three healthy owned installations: %+v %v", view, err)
	}
	seen := map[string]bool{}
	for _, installation := range view.Installations {
		if len(installation.Bindings) != 1 {
			t.Fatalf("unexpected restore bindings: %+v", installation)
		}
		binding := installation.Bindings[0]
		if binding.Owner == nil || binding.Materialization != "materialized" || binding.Activation != "active" || binding.Verification != "installation_verified" || binding.ProjectionStatus != "" {
			t.Fatalf("unhealthy restored binding: %+v", binding)
		}
		for _, pin := range locked.Packages {
			if binding.Owner.Alias != pin.Alias {
				continue
			}
			if seen[pin.Alias] || binding.TreeDigest != pin.TreeDigest || binding.Revision != lockedCommit(pin) || binding.Owner.SourceKey != pin.SourceKey || binding.Owner.PackageDigest != pin.TreeDigest {
				t.Fatalf("restored binding disagrees with pin: %+v want %+v", binding, pin)
			}
			seen[pin.Alias] = true
			stored, err := fixture.app.dependencyStore().Get(context.Background(), pin.DigestAlgorithm, pin.TreeDigest)
			if err != nil {
				t.Fatalf("restored store cannot verify %s: %v", pin.Alias, err)
			}
			var manifest struct{ Name, Version string }
			if err := json.Unmarshal([]byte(dependencyRead(t, filepath.Join(stored.Root, "plugin.json"))), &manifest); err != nil || manifest.Name != pin.Alias || manifest.Version != pin.Version {
				t.Fatalf("restored manifest differs from recorded version: %+v %v", manifest, err)
			}
		}
	}
	if len(seen) != 3 {
		t.Fatalf("restored bindings omitted a graph node: %v", seen)
	}
}

type freshRestoreStamp struct {
	body     string
	mode     fs.FileMode
	modified time.Time
}

func freshRestoreFiles(t *testing.T, roots ...string) map[string]freshRestoreStamp {
	t.Helper()
	files := map[string]freshRestoreStamp{}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if os.IsNotExist(err) && path == root {
				files[path] = freshRestoreStamp{body: "<absent>"}
				return nil
			}
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			var body []byte
			if entry.Type()&os.ModeSymlink != 0 {
				var target string
				target, err = os.Readlink(path)
				body = []byte(target)
			} else {
				body, err = os.ReadFile(path)
			}
			if err != nil {
				return err
			}
			files[path] = freshRestoreStamp{string(body), info.Mode(), info.ModTime()}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return files
}
