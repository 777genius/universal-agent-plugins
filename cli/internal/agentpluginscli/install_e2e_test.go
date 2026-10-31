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

// TestInstallConvergesDependencyEnvironment proves the everyday install verb
// end to end: real Git acquisition into exclusively disposable roots through
// dependencyTestSource, which counts every download and ref lookup. It covers
// fresh install, the idempotent repeat, out-of-band deletion and drift repair,
// --locked fail-closed refusals and removal attestation consistency with sync.
func TestInstallConvergesDependencyEnvironment(t *testing.T) {
	// Standard Windows ambient names must not block native lifecycle retries.
	t.Setenv("ProgramFiles(x86)", "ignored-windows-installation-metadata")
	t.Setenv("CommonProgramFiles(x86)", "ignored-windows-installation-metadata")
	repo := writeCLIPlugin(t)
	writeCLIMCP(t, repo)
	git := func(args ...string) {
		t.Helper()
		c := exec.CommandContext(context.Background(), "git", append([]string{"-c", "commit.gpgsign=false"}, args...)...)
		c.Dir = repo
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=iliya", "GIT_AUTHOR_EMAIL=iliyazelenkog@gmail.com", "GIT_COMMITTER_NAME=iliya", "GIT_COMMITTER_EMAIL=iliyazelenkog@gmail.com")
		if b, e := c.CombinedOutput(); e != nil {
			t.Fatalf("git %v: %v %s", args, e, b)
		}
	}
	git("init", "-b", "main")
	git("add", ".")
	git("commit", "-m", "test: disposable install fixture")

	client := fixtureClient(t, domain.ClientCodex)
	client.ExecutablePath = filepath.Join(t.TempDir(), "codex")
	runner := &dependencyClaudeListing{root: client.ConfigRoot}
	f, source := newDependencyHarness(t, repo, []domain.DetectedClient{client}, installer.Config{Runner: runner})
	run := func(args ...string) string { return dependencyRun(t, f, args...) }

	if help, stderr, err := f.execute(false, "install", "--help"); err != nil || !strings.Contains(help, "uap.toml and uap.lock") {
		t.Fatalf("install help: %v\n%s\n%s", err, help, stderr)
	}

	mp, lp := f.app.dependencyPaths()
	manifest := "schema_version = 1\n[dependencies.demo]\ngit = \"acme/demo\"\nref = \"main\"\n[targets]\ncodex = [\"demo\"]\n"
	dependencyWrite(t, mp, manifest)

	// a) Fresh: nothing installed, everything declared is installed once.
	run("install", "--yes", "--format", "json")
	if source.lookups < 1 || source.downloads < 1 {
		t.Fatalf("fresh install acquired nothing: %+v", source)
	}
	reqs, err := readDependencyManifest(mp)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := readDependencyLock(lp, reqs)
	if err != nil || len(lock.Packages) != 1 {
		t.Fatalf("install did not write the dependency lock: %+v %v", lock, err)
	}
	binding := dependencyBinding(t, f)
	if binding.TreeDigest != lock.Packages[0].TreeDigest {
		t.Fatalf("installed digest diverges from uap.lock: %+v %v", binding, lock.Packages[0])
	}
	if _, err := os.Stat(binding.TargetPath); err != nil {
		t.Fatal(err)
	}
	state, err := f.store.Load()
	if err != nil || len(state.EnvironmentReceipts) != 1 {
		t.Fatalf("owned receipts: %+v %v", state, err)
	}
	status := run("deps", "status", "--format", "json")
	if strings.Contains(status, "mismatch") || strings.Contains(status, "not_synced") {
		t.Fatal(status)
	}
	if _, _, err := f.execute(false, "install", "--external-uninstalled", "--yes", "--format", "json"); err == nil || !strings.Contains(err.Error(), "at least one removal") {
		t.Fatalf("external uninstall attestation accepted without a removal: %v", err)
	}

	// b) Idempotent: an intact repeat is a successful nothing-to-do that never
	// re-acquires and never rewrites state or environment bytes.
	downloads, lookups := source.downloads, source.lookups
	beforeState, beforeLock, beforeManifest := dependencyRead(t, f.store.Path), dependencyRead(t, lp), dependencyRead(t, mp)
	repeat := run("install", "--format", "json")
	var output struct {
		Selection installSelection `json:"selection"`
	}
	if err := json.Unmarshal([]byte(repeat), &output); err != nil {
		t.Fatal(err)
	}
	if output.Selection.Scope != "user" || output.Selection.ManifestPath != mp || output.Selection.LockPath != lp {
		t.Fatalf("selection missing from install output: %s", repeat)
	}
	if source.downloads != downloads || source.lookups != lookups {
		t.Fatal("repeat install acquired source")
	}
	if dependencyRead(t, f.store.Path) != beforeState || dependencyRead(t, lp) != beforeLock || dependencyRead(t, mp) != beforeManifest {
		t.Fatal("repeat install changed state or environment bytes")
	}

	// c) Drift repair: deleting the installed item out of band is restored
	// exactly, and a modified projection is repaired, both from the recorded pin.
	projection := binding.ProjectionDigest
	if err := os.RemoveAll(binding.TargetPath); err != nil {
		t.Fatal(err)
	}
	downloads, lookups = source.downloads, source.lookups
	run("install", "--yes", "--format", "json")
	restored := dependencyBinding(t, f)
	if restored.TreeDigest != binding.TreeDigest || restored.ProjectionDigest != projection || restored.ProjectionStatus != "" {
		t.Fatalf("install did not restore the deleted item exactly: %+v", restored)
	}
	if _, err := os.Stat(restored.TargetPath); err != nil {
		t.Fatal(err)
	}
	if source.downloads != downloads || source.lookups != lookups {
		t.Fatal("restore re-acquired instead of restoring the recorded pin")
	}
	dependencyWrite(t, filepath.Join(restored.TargetPath, "user-edit.txt"), "user edit")
	run("install", "--yes", "--format", "json")
	if healed := dependencyBinding(t, f); healed.ProjectionStatus != "" || healed.ProjectionDigest != projection {
		t.Fatalf("install did not repair the drifted item: %+v", healed)
	}

	// d) --locked with a missing or changed lock fails closed without writes.
	savedLock := beforeLock
	beforeState = dependencyRead(t, f.store.Path)
	downloads, lookups = source.downloads, source.lookups
	if err := os.Remove(lp); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.execute(false, "install", "--locked", "--yes", "--format", "json"); err == nil {
		t.Fatal("--locked accepted a missing lock")
	}
	if _, err := os.Stat(lp); !os.IsNotExist(err) {
		t.Fatal("--locked refusal wrote uap.lock")
	}
	dependencyWrite(t, lp, savedLock)
	dependencyWrite(t, mp, strings.ReplaceAll(manifest, `ref = "main"`, `ref = "different"`))
	if _, _, err := f.execute(false, "install", "--locked", "--yes", "--format", "json"); err == nil {
		t.Fatal("--locked accepted a changed manifest")
	}
	if _, _, err := f.execute(false, "install", "--locked=false", "--yes", "--format", "json"); err == nil {
		t.Fatal("default install accepted a changed manifest with an existing lock")
	}
	dependencyWrite(t, mp, manifest)
	tampered := strings.ReplaceAll(savedLock, lock.ManifestDigest, strings.Replace(lock.ManifestDigest, "sha256:", "sha256:0", 1))
	dependencyWrite(t, lp, tampered)
	if _, _, err := f.execute(false, "install", "--locked", "--yes", "--format", "json"); err == nil {
		t.Fatal("--locked accepted a changed lock")
	}
	if _, _, err := f.execute(false, "install", "--yes", "--format", "json"); err == nil {
		t.Fatal("default install accepted a changed lock")
	}
	if source.downloads != downloads || source.lookups != lookups {
		t.Fatal("--locked refusals acquired source")
	}
	if dependencyRead(t, lp) != tampered || dependencyRead(t, mp) != manifest || dependencyRead(t, f.store.Path) != beforeState {
		t.Fatal("--locked refusals wrote state")
	}
	dependencyWrite(t, lp, savedLock)

	// e) Removal attestation stays consistent with deps sync: removal requires
	// --external-uninstalled, and only the attested run removes the projection.
	dependencyWrite(t, mp, "schema_version = 1\n[targets]\n")
	run("deps", "lock", "--format", "json")
	beforeRemoval := dependencyRead(t, f.store.Path)
	if _, _, err := f.execute(false, "install", "--locked", "--yes", "--format", "json"); err == nil {
		t.Fatal("removal without external uninstall attestation reported success")
	}
	if dependencyRead(t, f.store.Path) != beforeRemoval {
		t.Fatal("refused removal changed state")
	}
	if _, err := os.Stat(binding.TargetPath); err != nil {
		t.Fatal("refused removal deleted the projection")
	}
	run("install", "--locked", "--yes", "--external-uninstalled", "--format", "json")
	after, err := f.store.Load()
	if err != nil || len(after.EnvironmentReceipts) != 0 {
		t.Fatalf("terminal removal retained owner: %+v %v", after, err)
	}
	if _, err := os.Stat(binding.TargetPath); !os.IsNotExist(err) {
		t.Fatalf("removed projection still exists: %v", err)
	}
	if _, err := f.app.dependencyStore().Get(context.Background(), domain.TreeDigestAlgorithm, binding.TreeDigest); err != nil {
		t.Fatalf("removal lost the shared store: %v", err)
	}
}

// This single sequential cycle catches graph omissions, accidental re-resolution,
// stale shared pins, premature removal, and loss of persistent package data at
// the real Git/store/installer boundary. All native commands are Codex fixtures.
func TestInstallDependencyGraphLifecycle(t *testing.T) {
	t.Setenv("ProgramFiles(x86)", "ignored-windows-installation-metadata")
	t.Setenv("CommonProgramFiles(x86)", "ignored-windows-installation-metadata")
	repo := t.TempDir()
	dependencyGit(t, repo, "init", "-b", "main")
	client := fixtureClient(t, domain.ClientCodex)
	client.ExecutablePath = filepath.Join(t.TempDir(), "codex")
	client.Version = "fixture-client"
	runner := &dependencyClaudeListing{root: client.ConfigRoot}
	f, source := newDependencyHarness(t, repo, []domain.DetectedClient{client}, installer.Config{Runner: runner})
	run := func(args ...string) string { t.Helper(); return dependencyRun(t, f, args...) }
	directory := &fixedDirectoryClient{bundle: directoryv1.VerifiedBundle{Digest: "sha256:" + strings.Repeat("a", 64), Snapshot: domain.DirectorySnapshot{
		SnapshotSchemaVersion: 1, Sequence: 1, SourceCommit: strings.Repeat("a", 40),
		Products: []domain.DirectoryProduct{}, Distributions: []domain.DirectoryDistribution{}, Evidence: []domain.DirectoryEvidence{}, Revocations: []domain.DirectoryRevocation{},
	}}}
	f.app.DirectoryClient = directory
	// Each package revision occupies its own subpath, so the acquired bytes and
	// digests really belong to the exact immutable commit recorded by Directory.
	addRelease := func(alias, version, requirement string, sequence uint64) domain.DirectoryRelease {
		t.Helper()
		subpath := alias + "-" + version
		root := filepath.Join(repo, subpath)
		if err := os.MkdirAll(filepath.Join(root, "skills", alias), 0700); err != nil {
			t.Fatal(err)
		}
		manifest := map[string]any{"$schema": domain.PluginSchemaV1, "name": alias, "version": version, "description": "Disposable graph package " + alias}
		if requirement != "" {
			dependencyWrite(t, filepath.Join(root, "uap.toml"), fmt.Sprintf("schema_version = 1\n[dependencies]\n\"acme/c\" = %q\n", requirement))
		}
		body, err := json.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		dependencyWrite(t, filepath.Join(root, "plugin.json"), string(body))
		dependencyWrite(t, filepath.Join(root, "skills", alias, "SKILL.md"), fmt.Sprintf("---\nname: %s\ndescription: Disposable graph skill\n---\nGraph fixture %s %s.\n", alias, alias, version))
		dependencyWrite(t, filepath.Join(root, "mcp.json"), fmt.Sprintf(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{%q:{"type":"stdio","command":"go","args":["version"],"env":{"DATA":"${PLUGIN_DATA}"}}}}`, alias))
		dependencyGit(t, repo, "add", ".")
		dependencyGit(t, repo, "commit", "-m", "test: disposable graph "+alias+" "+version)
		command := exec.CommandContext(context.Background(), "git", "rev-parse", "HEAD")
		command.Dir = repo
		head, err := command.Output()
		if err != nil {
			t.Fatal(err)
		}
		loaded, err := f.app.acquireLocal(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		release := domain.DirectoryRelease{Sequence: sequence, PackageVersion: version, ManifestName: alias, AgentPluginsSchema: domain.PluginSchemaV1,
			PackageSource:       domain.DirectorySource{Repository: "acme/graph-fixture", Revision: strings.TrimSpace(string(head)), Path: subpath},
			TreeDigestAlgorithm: domain.TreeDigestAlgorithm, TreeDigest: loaded.envelope.TreeDigest, ManifestDigest: loaded.envelope.ManifestDigest,
			Components: []string{"mcp", "skills"}, PublishedAt: "2026-10-17T00:00:00Z"}
		if err := loaded.cleanup(); err != nil {
			t.Fatal(err)
		}
		snapshot := &directory.bundle.Snapshot
		index := -1
		for i := range snapshot.Distributions {
			if snapshot.Distributions[i].ID == "acme/"+alias {
				index = i
			}
		}
		if index < 0 {
			snapshot.Products = append(snapshot.Products, domain.DirectoryProduct{SchemaVersion: 1, ID: alias, DisplayName: alias, Description: "Disposable graph package", ManifestName: alias, Aliases: []string{alias}, ReservedAliases: []string{alias}, Categories: []string{}, DefaultDistribution: "acme/" + alias, Distributions: []string{"acme/" + alias}, MinimumCapabilities: domain.DirectoryMinimumCapabilities{Skills: "optional", MCP: "optional"}})
			snapshot.Distributions = append(snapshot.Distributions, domain.DirectoryDistribution{SchemaVersion: 1, ID: "acme/" + alias, ProductID: alias, Kind: domain.DistributionUpstream, Status: domain.DistributionActive, Packager: "acme"})
			index = len(snapshot.Distributions) - 1
		}
		evidence := intendedTrustedDirectoryEvidence(domain.DirectoryEvidence{ID: fmt.Sprintf("passed/materialization/%s/%d", alias, sequence), DistributionID: "acme/" + alias, ReleaseSequence: sequence, PackageTreeDigest: release.TreeDigest, Level: "materialization", Outcome: "passed", Client: domain.ClientCodex, ClientVersion: client.Version, InstallerVersion: f.app.Version, OS: runtime.GOOS, Architecture: runtime.GOARCH})
		snapshot.Evidence = append(snapshot.Evidence, evidence)
		distribution := &snapshot.Distributions[index]
		distribution.Releases = append(distribution.Releases, release)
		distribution.ReleasePolicies = append(distribution.ReleasePolicies, domain.DirectoryReleasePolicy{ReleaseSequence: sequence, Status: domain.ReleaseActive, MinimumInstallerVersion: "0.1.0", CurrentEvidence: []string{evidence.ID}, Targets: []domain.DirectoryTarget{{Client: domain.ClientCodex, Scopes: []domain.InstallScope{domain.ScopeUser}, Delivery: "managed", Authentication: domain.AuthenticationRequirementNotRequired}}})
		return release
	}
	addRelease("a", "1.2.0", "^1.2", 1)
	firstA := addRelease("a", "1.3.0", "^1.2", 2)
	firstB := addRelease("b", "1.0.0", "^1.2", 1)
	firstC := addRelease("c", "1.2.0", "", 1)
	mp, lp := f.app.dependencyPaths()
	manifest := "schema_version = 1\n[dependencies]\na = \"^1.2\"\nb = \"1.0.0\"\n[targets]\ncodex = [\"a\", \"b\"]\n"
	dependencyWrite(t, mp, manifest)
	readLock := func() environment.Lock {
		t.Helper()
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
	pins := func(locked environment.Lock) map[string]environment.LockedPackage {
		t.Helper()
		result := map[string]environment.LockedPackage{}
		for _, pkg := range locked.Packages {
			if _, duplicate := result[pkg.Alias]; duplicate {
				t.Fatalf("duplicate alias %s", pkg.Alias)
			}
			result[pkg.Alias] = pkg
		}
		return result
	}
	checkPin := func(pkg environment.LockedPackage, release domain.DirectoryRelease) {
		t.Helper()
		if pkg.Version != release.PackageVersion || pkg.TreeDigest != release.TreeDigest || pkg.ManifestDigest != release.ManifestDigest || pkg.Source.Registry == nil || pkg.Source.Registry.ReleaseSequence != release.Sequence || pkg.Source.Registry.Commit != release.PackageSource.Revision {
			t.Fatalf("pin mismatch: %+v want %+v", pkg, release)
		}
	}
	checkParents := func(pkg environment.LockedPackage, parents ...string) {
		t.Helper()
		want := make([]environment.RequiredBy, 0, len(parents))
		for _, parent := range parents {
			selector := "^1.2"
			if parent == "a" && pkg.Version == "1.3.0" {
				selector = "^1.3"
			}
			want = append(want, environment.RequiredBy{Alias: parent, Selector: environment.Selector{Kind: environment.SelectorRange, Value: selector}})
		}
		if !reflect.DeepEqual(pkg.RequiredBy, want) || !reflect.DeepEqual(pkg.Targets, []string{"codex"}) {
			t.Fatalf("shared edges/targets: %+v want %+v", pkg, want)
		}
	}
	bindings := func() map[string]installer.InspectedBinding {
		t.Helper()
		view, err := f.app.Installer.Inspect(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if view.Recovery.Required {
			t.Fatalf("unexpected recovery: %+v", view)
		}
		result := map[string]installer.InspectedBinding{}
		for _, installation := range view.Installations {
			for _, binding := range installation.Bindings {
				if binding.Owner == nil || binding.ClientID != "codex" {
					continue
				}
				alias := binding.Owner.Alias
				if _, duplicate := result[alias]; duplicate {
					t.Fatalf("duplicate owned binding %s", alias)
				}
				if binding.Materialization != "materialized" || binding.ProjectionStatus != "" {
					t.Fatalf("unhealthy binding: %+v", binding)
				}
				result[alias] = binding
			}
		}
		if len(view.EnvironmentReceipts) != len(result) {
			t.Fatalf("receipt/binding mismatch: %+v", view)
		}
		return result
	}
	checkBindings := func(locked environment.Lock) map[string]installer.InspectedBinding {
		t.Helper()
		got := bindings()
		if len(got) != len(locked.Packages) {
			t.Fatalf("bindings=%+v packages=%+v", got, locked.Packages)
		}
		for _, pkg := range locked.Packages {
			binding, ok := got[pkg.Alias]
			if !ok || binding.TreeDigest != pkg.TreeDigest || binding.Revision != pkg.Source.Registry.Commit || binding.Owner.SourceKey != pkg.SourceKey || binding.Owner.PackageDigest != pkg.TreeDigest {
				t.Fatalf("owned pin differs: %+v want %+v", binding, pkg)
			}
		}
		return got
	}
	type fileStamp struct {
		Body     string
		Modified time.Time
		Mode     fs.FileMode
	}
	snapshot := func(paths ...string) map[string]fileStamp {
		t.Helper()
		result := map[string]fileStamp{}
		for _, root := range paths {
			err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
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
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				result[path] = fileStamp{string(body), info.ModTime(), info.Mode()}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		return result
	}
	checkTree := func(locked environment.Lock, roots []string) {
		t.Helper()
		before := snapshot(mp, lp, f.store.Path, f.app.EnvironmentStoreRoot, f.app.ManagedRoot)
		downloads, lookups, calls := source.downloads, source.lookups, directory.calls
		human := run("deps", "tree")
		output := run("deps", "tree", "--format", "json")
		var tree struct {
			Roots    []string                    `json:"roots"`
			Packages []environment.LockedPackage `json:"packages"`
		}
		if err := json.Unmarshal([]byte(output), &tree); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(tree.Roots, roots) || !reflect.DeepEqual(tree.Packages, locked.Packages) {
			t.Fatalf("tree disagrees with lock: %s", output)
		}
		for _, pkg := range locked.Packages {
			if !strings.Contains(human, pkg.Alias+" "+pkg.Version) {
				t.Fatalf("human tree omitted pin %s: %s", pkg.Alias, human)
			}
		}
		if len(roots) == 2 && (!strings.Contains(human, "  c 1.2.0") || !strings.Contains(human, "reference")) {
			t.Fatalf("human tree omitted shared edge: %s", human)
		}
		if len(roots) == 1 && (strings.Contains(human, "a 1.") || !strings.Contains(human, "  c 1.3.0")) {
			t.Fatalf("human tree retained removed edge: %s", human)
		}
		if source.downloads != downloads || source.lookups != lookups || directory.calls != calls || !reflect.DeepEqual(before, snapshot(mp, lp, f.store.Path, f.app.EnvironmentStoreRoot, f.app.ManagedRoot)) {
			t.Fatal("deps tree wrote files or acquired source/catalog")
		}
	}
	// 1. Fresh resolution chooses the highest compatible A, with one shared C.
	run("deps", "lock", "--format", "json")
	locked := readLock()
	initial := pins(locked)
	if len(initial) != 3 {
		t.Fatalf("expected three graph nodes: %+v", locked)
	}
	checkPin(initial["a"], firstA)
	checkPin(initial["b"], firstB)
	checkPin(initial["c"], firstC)
	checkParents(initial["c"], "a", "b")
	// A transitive dependency is retained even before any client binding exists.
	for _, format := range []string{"human", "json"} {
		before := reconciliationFiles(t, f, "")
		downloads, lookups, calls := source.downloads, source.lookups, directory.calls
		output := run("remove", "c", "--target", "codex", "--format", format)
		if !strings.Contains(output, "a") || !strings.Contains(output, "b") || !strings.Contains(output, "retained") {
			t.Fatalf("transitive removal did not explain retention: %s", output)
		}
		if !reflect.DeepEqual(before, reconciliationFiles(t, f, "")) || source.downloads != downloads || source.lookups != lookups || directory.calls != calls {
			t.Fatal("transitive removal before install changed files or acquired source")
		}
		view, err := f.app.Installer.Inspect(context.Background())
		if err != nil || len(view.Installations) != 0 || len(view.EnvironmentReceipts) != 0 {
			t.Fatalf("transitive removal installed bindings: %+v %v", view, err)
		}
	}
	// 2. Publish a newer C after locking. Replay keeps the recorded graph.
	nextC := addRelease("c", "1.3.0", "", 2)
	directory.bundle.Snapshot.Sequence = 2
	directory.bundle.Digest = "sha256:" + strings.Repeat("b", 64)
	run("install", "--locked", "--yes", "--format", "json")
	installed := checkBindings(locked)
	checkTree(locked, []string{"a", "b"})
	data := installed["c"].DataRoot
	relative, err := filepath.Rel(f.root, data)
	if err != nil || !filepath.IsAbs(data) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		t.Fatalf("PLUGIN_DATA escaped disposable roots: %q", data)
	}
	marker := filepath.Join(data, "retained.txt")
	dependencyWrite(t, marker, "keep shared package data")
	// 3. An intact repeat neither acquires nor rewrites any agreement/state/projection.
	paths := make([]string, 0, 6)
	paths = append(paths, mp, lp, f.store.Path)
	for _, alias := range []string{"a", "b", "c"} {
		paths = append(paths, installed[alias].TargetPath)
	}
	before := snapshot(paths...)
	downloads, lookups := source.downloads, source.lookups
	run("install", "--locked", "--yes", "--format", "json")
	if source.downloads != downloads || source.lookups != lookups || !reflect.DeepEqual(before, snapshot(paths...)) {
		t.Fatal("repeat install acquired source or changed bytes/mtime")
	}
	// 4. Repair from the store. A later lower version cannot downgrade A;
	// an eligible higher version then advances A and the shared C closure.
	drift := filepath.Join(installed["c"].TargetPath, "user-edit.txt")
	dependencyWrite(t, drift, "drift")
	run("install", "--locked", "--yes", "--format", "json")
	repaired := checkBindings(locked)
	if repaired["c"].ProjectionDigest != installed["c"].ProjectionDigest || source.downloads != downloads || source.lookups != lookups {
		t.Fatal("repair changed the pinned projection or acquired source")
	}
	if _, err := os.Stat(drift); !os.IsNotExist(err) {
		t.Fatalf("drift survived repair: %v", err)
	}
	addRelease("a", "1.2.0", "^1.3", 3)
	directory.bundle.Snapshot.Sequence = 3
	directory.bundle.Digest = "sha256:" + strings.Repeat("c", 64)
	beforeLower := snapshot(mp, lp, f.store.Path, f.app.EnvironmentStoreRoot, f.app.ManagedRoot)
	run("deps", "update", "a", "--format", "json")
	if !reflect.DeepEqual(locked, readLock()) || !reflect.DeepEqual(beforeLower, snapshot(mp, lp, f.store.Path, f.app.EnvironmentStoreRoot, f.app.ManagedRoot)) {
		t.Fatal("later lower version changed the pinned graph or installed files")
	}
	nextA := addRelease("a", "1.4.0", "^1.3", 4)
	directory.bundle.Snapshot.Sequence = 4
	directory.bundle.Digest = "sha256:" + strings.Repeat("d", 64)
	beforeUpdate := snapshot(append([]string{mp, f.store.Path}, paths[3:]...)...)
	previousLock := dependencyRead(t, lp)
	run("deps", "update", "a", "--format", "json")
	updated := readLock()
	current := pins(updated)
	checkPin(current["a"], nextA)
	checkPin(current["b"], firstB)
	checkPin(current["c"], nextC)
	checkParents(current["c"], "a", "b")
	if !reflect.DeepEqual(initial["b"], current["b"]) || dependencyRead(t, mp) != manifest || dependencyRead(t, lp) == previousLock || !reflect.DeepEqual(beforeUpdate, snapshot(append([]string{mp, f.store.Path}, paths[3:]...)...)) {
		t.Fatal("update failed to change only lock and preserve independent B/intent/projections")
	}
	run("install", "--locked", "--yes", "--format", "json")
	applied := checkBindings(updated)
	if applied["a"].TreeDigest == installed["a"].TreeDigest || applied["c"].TreeDigest == installed["c"].TreeDigest || applied["b"].TreeDigest != installed["b"].TreeDigest {
		t.Fatal("install did not apply updated A/C pins and preserve B")
	}
	// 5. Remove consumers sequentially: C survives B, then loses its owned binding.
	dependencyWrite(t, mp, "schema_version = 1\n[dependencies]\nb = \"1.0.0\"\n[targets]\ncodex = [\"b\"]\n")
	run("deps", "lock", "--format", "json")
	retained := readLock()
	remaining := pins(retained)
	if len(remaining) != 2 {
		t.Fatalf("A removal produced wrong closure: %+v", retained)
	}
	checkPin(remaining["c"], nextC)
	checkParents(remaining["c"], "b")
	run("install", "--locked", "--yes", "--external-uninstalled", "--format", "json")
	kept := checkBindings(retained)
	if kept["c"].TreeDigest != applied["c"].TreeDigest {
		t.Fatal("removing A changed the shared C pin")
	}
	if _, err := os.Stat(applied["a"].TargetPath); !os.IsNotExist(err) {
		t.Fatalf("A projection survives removal: %v", err)
	}
	checkTree(retained, []string{"b"})
	dependencyWrite(t, mp, "schema_version = 1\n[targets]\n")
	run("deps", "lock", "--format", "json")
	empty := readLock()
	if len(empty.Packages) != 0 {
		t.Fatalf("last consumer left orphan nodes: %+v", empty)
	}
	run("install", "--locked", "--yes", "--external-uninstalled", "--format", "json")
	if got := bindings(); len(got) != 0 {
		t.Fatalf("owned bindings survive last consumer: %+v", got)
	}
	for _, binding := range applied {
		if _, err := os.Stat(binding.TargetPath); !os.IsNotExist(err) {
			t.Fatalf("removed projection remains: %s: %v", binding.TargetPath, err)
		}
	}
	for _, release := range []domain.DirectoryRelease{firstA, firstB, firstC, nextA, nextC} {
		if _, err := f.app.dependencyStore().Get(context.Background(), domain.TreeDigestAlgorithm, release.TreeDigest); err != nil {
			t.Fatalf("removal lost immutable store object %s: %v", release.TreeDigest, err)
		}
	}
	if dependencyRead(t, marker) != "keep shared package data" {
		t.Fatal("removal lost C PLUGIN_DATA")
	}
}
