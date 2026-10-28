package agentpluginscli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// These tests reuse the real immutable Git acquisition, store and SDK boundary.
// Directory inputs and native client responses are disposable fixtures.
type transferGraph struct {
	original       freshRestoreEnvironment
	repo           string
	directory      *fixedDirectoryClient
	manifest, lock string
	locked         environment.Lock
}

func TestDependencyTransferGraphRoundTrip(t *testing.T) {
	graph := newTransferGraph(t)
	original := graph.original
	before := freshRestoreFiles(t, original.root, original.client.ConfigRoot)
	exchange := graph.export(t)
	exchangeBefore := freshRestoreFiles(t, exchange)
	t.Chdir(exchange)
	graph.restore(t)
	t.Run("combined success and JSON", graph.combined)
	t.Run("combined previews have no durable effects", graph.previews)
	t.Run("strict policy refusal preserves destination", graph.policyRefusal)
	t.Run("postpublication failure saves desired pair", graph.applyFailure)
	t.Run("real preflight isolates persistent adapters", graph.staging)
	if !reflect.DeepEqual(exchangeBefore, freshRestoreFiles(t, exchange)) || !reflect.DeepEqual(before, freshRestoreFiles(t, original.root, original.client.ConfigRoot)) {
		t.Fatal("transfer or restore changed the original environment or exchange source")
	}
}

func newTransferGraph(t *testing.T) transferGraph {
	t.Helper()
	original, repo, directory := dependencyReviewFixture(t)
	a := freshRestorePackage(t, original, repo, "a", "1.3.0", "a-1.3.0", "^1.2", 1, true)
	b := freshRestorePackage(t, original, repo, "b", "1.0.0", "b", "^1.2", 1, false)
	c := freshRestorePackage(t, original, repo, "c", "1.2.0", "c-1.2.0", "", 1, true)
	b.PackageSource.Revision = c.PackageSource.Revision
	mp, lp := original.app.dependencyPaths()
	dependencyWrite(t, mp, "schema_version = 1\n# exact portable bytes\r\n[dependencies]\na = \"^1.2\"\n[dependencies.b]\ngit = \"acme/fresh-restore\"\npath = \"b\"\nref = \"main\"\n[targets]\ncodex = [\"a\", \"b\"]\n")
	dependencyRun(t, original.cliFixture, "deps", "lock", "--format", "json")
	locked := freshRestoreReadLock(t, original.cliFixture)
	freshRestoreCheckGraph(t, locked, map[string]domain.DirectoryRelease{"a": a, "b": b, "c": c})
	dependencyRun(t, original.cliFixture, "install", "--locked", "--scope", "user", "--yes", "--format", "json")
	manifest, lock := dependencyRead(t, mp), dependencyRead(t, lp)
	freshRestorePackage(t, original, repo, "b", "1.1.0", "b", "^1.2", 2, false)
	freshRestorePackage(t, original, repo, "a", "1.4.0", "a-1.4.0", "^1.2", 2, true)
	freshRestorePackage(t, original, repo, "c", "1.3.0", "c-1.3.0", "", 2, true)
	for i := range directory.bundle.Snapshot.Distributions {
		directory.bundle.Snapshot.Distributions[i].ReleasePolicies[0].Status = domain.ReleaseSuperseded
	}
	return transferGraph{original, repo, directory, manifest, lock, locked}
}

func (graph transferGraph) export(t *testing.T) string {
	t.Helper()
	original, directory := graph.original, graph.directory
	downloads, lookups, native, loads := original.source.downloads, original.source.lookups, original.runner.calls, directory.calls
	originalDetector := &observedProbingDetector{clients: []domain.DetectedClient{original.client}}
	original.app.Detector = originalDetector
	exported := t.TempDir()
	t.Chdir(exported)
	transferJSON(t, dependencyRun(t, original.cliFixture, "deps", "export", "--yes", "--format", "json"))
	if original.source.downloads != downloads || original.source.lookups != lookups || original.runner.calls != native || directory.calls != loads || originalDetector.readOnlyCalls+originalDetector.probeCalls+originalDetector.targetedCalls != 0 {
		t.Fatal("plain export acquired packages, resolved refs, loaded Directory or invoked a client")
	}
	entries, err := os.ReadDir(exported)
	if err != nil || len(entries) != 2 {
		t.Fatalf("export copied more than the agreement: %v %v", entries, err)
	}
	exchange := t.TempDir()
	transferPair(t, exchange, dependencyRead(t, filepath.Join(exported, "uap.toml")), dependencyRead(t, filepath.Join(exported, "uap.lock")))
	return exchange
}

func (graph transferGraph) restore(t *testing.T) {
	t.Helper()
	original, repo, directory := graph.original, graph.repo, graph.directory
	manifest, lock, locked := graph.manifest, graph.lock, graph.locked
	loads := directory.calls
	fresh := newFreshRestoreEnvironment(t, repo, directory)
	if fresh.root == original.root || fresh.store.Path == original.store.Path || fresh.app.EnvironmentStoreRoot == original.app.EnvironmentStoreRoot || fresh.client.ConfigRoot == original.client.ConfigRoot || fresh.source.TempRoot == original.source.TempRoot {
		t.Fatal("destination reused original store, state, profile or acquisition roots")
	}
	freshDetector := &observedProbingDetector{clients: []domain.DetectedClient{fresh.client}}
	fresh.app.Detector = freshDetector
	freshRestoreAssertNoLocalState(t, fresh)
	transferJSON(t, dependencyRun(t, fresh.cliFixture, "deps", "import", "--yes", "--format", "json"))
	if fresh.source.downloads != 0 || fresh.source.lookups != 0 || fresh.runner.calls != 0 || directory.calls != loads || freshDetector.readOnlyCalls+freshDetector.probeCalls+freshDetector.targetedCalls != 0 {
		t.Fatal("plain import performed installation work")
	}
	freshMP, freshLP := fresh.app.dependencyPaths()
	if dependencyRead(t, freshMP) != manifest || dependencyRead(t, freshLP) != lock || !reflect.DeepEqual(locked, freshRestoreReadLock(t, fresh.cliFixture)) {
		t.Fatal("import changed raw agreement or immutable graph pins")
	}
	dependencyRun(t, fresh.cliFixture, "install", "--locked", "--scope", "user", "--yes", "--format", "json")
	freshRestoreCheckFetches(t, fresh.source, locked)
	freshRestoreCheckInstalled(t, fresh, locked)
	freshRestoreAssertStagingClean(t, fresh.source)
	beforeRepeat := freshRestoreFiles(t, fresh.root, fresh.client.ConfigRoot)
	mutations := fresh.runner.mutations
	transferJSON(t, dependencyRun(t, fresh.cliFixture, "deps", "import", "--format", "json"))
	dependencyRun(t, fresh.cliFixture, "install", "--locked", "--scope", "user", "--yes", "--format", "json")
	if fresh.source.downloads != 3 || fresh.source.lookups != 0 || fresh.runner.mutations != mutations || !reflect.DeepEqual(beforeRepeat, freshRestoreFiles(t, fresh.root, fresh.client.ConfigRoot)) {
		t.Fatal("repeated transfer and locked restore rewrote files, resolved refs or repeated effects")
	}
	t.Log("cold transfer restore: 3 exact acquisitions; repeat: 0 acquisitions, 0 lookups, 0 native mutations")

}

func (graph transferGraph) combined(t *testing.T) {
	repo, directory, locked := graph.repo, graph.directory, graph.locked
	combined := newFreshRestoreEnvironment(t, repo, directory)
	result := transferJSON(t, dependencyRun(t, combined.cliFixture, "deps", "import", "--install", "--yes", "--format", "json"))
	if string(result["agreement_saved"]) != "true" || string(result["install_requested"]) != "true" || string(result["outcome"]) != `"completed"` {
		t.Fatalf("combined import omitted its result: %s", result)
	}
	for _, field := range []string{"actions", "results"} {
		var values []json.RawMessage
		if err := json.Unmarshal(result[field], &values); err != nil || len(values) != 3 {
			t.Fatalf("combined import lost installer %s: %s %v", field, result[field], err)
		}
	}
	freshRestoreCheckFetches(t, combined.source, locked)
	freshRestoreCheckInstalled(t, combined, locked)
	freshRestoreAssertStagingClean(t, combined.source)
	before := freshRestoreFiles(t, combined.root, combined.client.ConfigRoot)
	mutations := combined.runner.mutations
	transferJSON(t, dependencyRun(t, combined.cliFixture, "deps", "import", "--install", "--yes", "--format", "json"))
	if combined.source.downloads != 3 || combined.source.lookups != 0 || combined.runner.mutations != mutations || !reflect.DeepEqual(before, freshRestoreFiles(t, combined.root, combined.client.ConfigRoot)) {
		t.Fatal("combined no-op repeated package/native publication")
	}
	transferNoopRecheck(t, combined, graph.manifest, graph.lock)
	transferPendingGuard(t, combined, graph.manifest)
}

func transferPendingGuard(t *testing.T, combined freshRestoreEnvironment, manifest string) {
	t.Helper()
	// Pending recovery is a refusal for plain import too, before replacing
	// the agreement or silently completing the transaction.
	state, err := combined.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range state.Installations[0].Clients {
		if len(binding.Receipts) == 0 {
			t.Fatal("installed fixture lacks transaction receipt")
		}
		binding.Receipts[len(binding.Receipts)-1].Phase = "state_committed"
		state.Installations[0].Clients[key] = binding
	}
	if err := combined.store.Save(state); err != nil {
		t.Fatal(err)
	}
	cmp, clp := combined.app.dependencyPaths()
	dependencyWrite(t, cmp, "# old agreement formatting\n"+manifest)
	before := freshRestoreFiles(t, cmp, clp, combined.store.Path, combined.client.ConfigRoot)
	_, _, err = combined.execute(false, "deps", "import", "--yes", "--format", "json")
	if !errors.Is(err, installer.ErrRecoveryRequired) || !reflect.DeepEqual(before, freshRestoreFiles(t, cmp, clp, combined.store.Path, combined.client.ConfigRoot)) {
		t.Fatalf("plain import bypassed pending recovery or changed files: %v", err)
	}
}

func (graph transferGraph) previews(t *testing.T) {
	repo, directory, locked := graph.repo, graph.directory, graph.locked
	for _, mode := range []string{"json", "human", "decline"} {
		preview := newFreshRestoreEnvironment(t, repo, directory)
		before := freshRestoreFiles(t, preview.root, preview.client.ConfigRoot)
		switch mode {
		case "decline":
			out, _, err := preview.executeInput(true, "n\n", "deps", "import", "--install")
			if err != nil || !strings.Contains(out, "No changes made") {
				t.Fatalf("combined decline: %v %s", err, out)
			}
		case "human":
			out := dependencyRun(t, preview.cliFixture, "deps", "import", "--install", "--dry-run")
			for _, alias := range []string{"a", "b", "c"} {
				if !strings.Contains(out, "install "+alias+"/codex") {
					t.Fatalf("human dry-run omitted planned installation %s: %s", alias, out)
				}
			}
		default:
			result := transferJSON(t, dependencyRun(t, preview.cliFixture, "deps", "import", "--install", "--dry-run", "--format", "json"))
			if string(result["outcome"]) != `"preview"` || string(result["agreement_saved"]) != "false" {
				t.Fatalf("dry-run claimed publication: %s", result)
			}
		}
		freshRestoreCheckFetches(t, preview.source, locked)
		freshRestoreAssertNoEffects(t, preview, before)
	}
}

func (graph transferGraph) policyRefusal(t *testing.T) {
	repo, directory := graph.repo, graph.directory
	refused := newFreshRestoreEnvironment(t, repo, directory)
	before := freshRestoreFiles(t, refused.root, refused.client.ConfigRoot)
	policy := &directory.bundle.Snapshot.Distributions[0].ReleasePolicies[0]
	old := policy.Status
	policy.Status = domain.ReleaseRevoked
	defer func() { policy.Status = old }()
	_, _, err := refused.execute(false, "deps", "import", "--install", "--yes", "--format", "json")
	if !errors.Is(err, domain.ErrDirectoryIneligible) {
		t.Fatalf("combined import bypassed strict policy preflight: %v", err)
	}
	freshRestoreAssertNoEffects(t, refused, before)
}

func (graph transferGraph) applyFailure(t *testing.T) {
	repo, directory := graph.repo, graph.directory
	manifest, lock, locked := graph.manifest, graph.lock, graph.locked
	failed := newFreshRestoreEnvironment(t, repo, directory)
	failed.runner.claude.failNextCodexAdd = true
	out, _, err := failed.execute(false, "deps", "import", "--install", "--yes", "--format", "json")
	if err == nil {
		t.Fatal("postpublication native failure reported success")
	}
	result := transferJSON(t, out)
	if string(result["agreement_saved"]) != "true" || string(result["outcome"]) != `"partial"` || !strings.Contains(err.Error(), "install --locked") {
		t.Fatalf("failure must report saved desired state and locked retry: %v %s", err, out)
	}
	failedMP, failedLP := failed.app.dependencyPaths()
	if dependencyRead(t, failedMP) != manifest || dependencyRead(t, failedLP) != lock || !reflect.DeepEqual(locked, freshRestoreReadLock(t, failed.cliFixture)) {
		t.Fatal("installer failure rolled back or rewrote imported desired state")
	}
	dependencyRun(t, failed.cliFixture, "install", "--locked", "--scope", "user", "--yes", "--format", "json")
	freshRestoreCheckInstalled(t, failed, locked)
	if failed.source.lookups != 0 {
		t.Fatal("retry resolved moving selectors")
	}
}

func transferJSON(t *testing.T, body string) map[string]json.RawMessage {
	t.Helper()
	var result map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("transfer must emit exactly one JSON object: %v\n%s", err, body)
	}
	for _, key := range []string{"operation", "scope", "source_directory", "destination_directory", "dry_run", "changed", "outcome", "files", "semantic_changes", "install_requested", "agreement_saved"} {
		if _, ok := result[key]; !ok {
			t.Fatalf("missing transfer result field %q: %s", key, body)
		}
	}
	return result
}

func transferPair(t *testing.T, root, manifest, lock string) {
	t.Helper()
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	dependencyWrite(t, filepath.Join(root, "uap.toml"), manifest)
	dependencyWrite(t, filepath.Join(root, "uap.lock"), lock)
}

// A command that chooses cwd as its source instead of the global environment,
// rewrites the agreement, or copies private environment state fails here.
func TestDependencyTransferExportDefaultDirectory(t *testing.T) {
	fixture := transferEmptyAgreement(t)
	mp, lp := fixture.app.dependencyPaths()
	manifest, lock := dependencyRead(t, mp), dependencyRead(t, lp)
	exchange := t.TempDir()
	if err := os.Chmod(exchange, 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(exchange)
	result := transferJSON(t, dependencyRun(t, fixture.cliFixture, "deps", "export", "--yes", "--format", "json"))
	if string(result["operation"]) != `"export"` || string(result["outcome"]) != `"completed"` || string(result["changed"]) != "true" {
		t.Fatalf("unexpected transfer outcome: %s", result)
	}
	if dependencyRead(t, filepath.Join(exchange, "uap.toml")) != manifest || dependencyRead(t, filepath.Join(exchange, "uap.lock")) != lock {
		t.Fatal("export did not preserve the exact global pair")
	}
	entries, err := os.ReadDir(exchange)
	if err != nil || len(entries) != 2 {
		t.Fatalf("export must copy only the two agreement files: %v %v", entries, err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(exchange)
		if err != nil || info.Mode().Perm() != 0755 {
			t.Fatalf("export changed cwd mode: %v %v", info, err)
		}
		for _, name := range []string{"uap.toml", "uap.lock"} {
			info, err := os.Stat(filepath.Join(exchange, name))
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatalf("new agreement file is not private: %v %v", info, err)
			}
		}
	}
}

// Failures after an atomic rename are possible (directory fsync), so restoration
// must inspect actual candidate bytes. Never clobber an external edit on rollback.
func TestDependencyTransferImportMutationGuards(t *testing.T) {
	for _, guard := range []string{"active mutation", "legacy schema"} {
		t.Run(guard, func(t *testing.T) {
			fixture := transferEmptyAgreement(t)
			mp, lp := fixture.app.dependencyPaths()
			exchange := t.TempDir()
			transferPair(t, exchange, "# incoming\n"+dependencyRead(t, mp), dependencyRead(t, lp))
			if guard == "active mutation" {
				release, err := (processlock.Lock{Path: fixture.app.EnvironmentLockPath}).Acquire(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = release() }()
			} else {
				dependencyWrite(t, fixture.store.Path, `{"schema_version":2}`)
			}
			before := freshRestoreFiles(t, mp, lp, fixture.store.Path, exchange)
			_, _, err := fixture.execute(false, "deps", "import", exchange, "--yes", "--format", "json")
			if guard == "active mutation" {
				if !errors.Is(err, processlock.ErrActive) {
					t.Fatalf("active global mutation not refused: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "migration") {
				t.Fatalf("legacy schema not refused: %v", err)
			}
			if !reflect.DeepEqual(before, freshRestoreFiles(t, mp, lp, fixture.store.Path, exchange)) || fixture.runner.calls != 0 || fixture.source.downloads != 0 {
				t.Fatal("guard refusal changed agreement, source, state or profile")
			}
		})
	}
}

func TestDependencyTransferDestinationLeavesAndAlias(t *testing.T) {
	for _, kind := range []string{"symlink", "directory", "same directory alias"} {
		t.Run(kind, func(t *testing.T) {
			if runtime.GOOS == "windows" && kind != "directory" {
				t.Skip("symlink permissions are host dependent")
			}
			fixture := transferEmptyAgreement(t)
			destination := t.TempDir()
			target := filepath.Join(t.TempDir(), "target")
			dependencyWrite(t, target, "external target")
			if kind == "same directory alias" {
				destination = filepath.Join(destination, "alias")
				if err := os.Symlink(fixture.app.DependenciesRoot, destination); err != nil {
					t.Fatal(err)
				}
			} else if kind == "symlink" {
				if err := os.Symlink(target, filepath.Join(destination, "uap.toml")); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Mkdir(filepath.Join(destination, "uap.lock"), 0700); err != nil {
				t.Fatal(err)
			}
			before := freshRestoreFiles(t, fixture.root, destination, target)
			out, _, err := fixture.execute(false, "deps", "export", destination, "--format", "json")
			if kind == "same directory alias" {
				if err != nil || string(transferJSON(t, out)["outcome"]) != `"unchanged"` {
					t.Fatalf("alias self-copy was not a no-op: %v %s", err, out)
				}
			} else if err == nil {
				t.Fatal("nonregular destination accepted")
			}
			if !reflect.DeepEqual(before, freshRestoreFiles(t, fixture.root, destination, target)) {
				t.Fatal("alias or nonregular destination changed source/target")
			}
		})
	}
}
