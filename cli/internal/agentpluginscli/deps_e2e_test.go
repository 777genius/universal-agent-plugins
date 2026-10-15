package agentpluginscli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type dependencyTestScanner struct{ fail bool }

func (s dependencyTestScanner) Evaluate(context.Context, domain.SecurityEvaluationInput) (domain.SecurityAssessment, error) {
	if s.fail {
		return domain.SecurityAssessment{}, errors.New("scanner unavailable")
	}
	return domain.SecurityAssessment{}, nil
}

type dependencyTestSource struct {
	sourceacquisition.Acquirer
	downloads, lookups int
}

func (s *dependencyTestSource) AcquireGitHub(ctx context.Context, repo, rev, path string) (domain.PackageSnapshot, error) {
	s.downloads++
	return s.Acquirer.AcquireGitHub(ctx, repo, rev, path)
}
func (s *dependencyTestSource) AcquireGitHubVerified(ctx context.Context, repo, rev, path, digest string) (domain.PackageSnapshot, error) {
	s.downloads++
	return s.Acquirer.AcquireGitHubVerified(ctx, repo, rev, path, digest)
}
func (s *dependencyTestSource) ResolveGitHubSelector(ctx context.Context, repo, selector string) (sourceacquisition.RefResolution, error) {
	s.lookups++
	return s.Acquirer.ResolveGitHubSelector(ctx, repo, selector)
}

// Real Git acquisition, standard loader, digest store, public SDK transaction,
// Codex projection and CLI commands run against exclusively disposable roots.
func TestDependencyEnvironmentCLIEndToEnd(t *testing.T) {
	repo := writeCLIPlugin(t)
	writeCLIMCP(t, repo)
	git := func(args ...string) {
		t.Helper()
		c := exec.CommandContext(context.Background(), "git", args...)
		c.Dir = repo
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=iliya", "GIT_AUTHOR_EMAIL=iliyazelenkog@gmail.com", "GIT_COMMITTER_NAME=iliya", "GIT_COMMITTER_EMAIL=iliyazelenkog@gmail.com")
		b, e := c.CombinedOutput()
		if e != nil {
			t.Fatalf("git %v: %v %s", args, e, b)
		}
	}
	git("init", "-b", "main")
	git("add", ".")
	git("commit", "-m", "test: disposable dependency fixture")
	var runners []*dependencyClaudeListing
	setup := func() (cliFixture, *dependencyTestSource) {
		client := fixtureClient(t, domain.ClientCodex)
		client.ExecutablePath = filepath.Join(t.TempDir(), "codex")
		claude := fixtureClient(t, domain.ClientClaude)
		claude.ExecutablePath = filepath.Join(t.TempDir(), "claude")
		f := newCLIFixture(t, []domain.DetectedClient{client, claude})
		root := filepath.Join(f.root, "data")
		helper, e := os.Executable()
		if e != nil {
			t.Fatal(e)
		}
		runner := &dependencyClaudeListing{root: claude.ConfigRoot}
		runners = append(runners, runner)
		f.app.Installer, e = installer.New(installer.Config{StateRoot: root, StateFile: f.store.Path, LockFile: filepath.Join(root, "mutation.lock"), OperationsDir: f.operations, PluginDataBase: filepath.Join(root, "plugin-data"), ManagedRoot: f.app.ManagedRoot, TempRoot: filepath.Join(root, "installer-tmp"), Registry: f.app.ClientRegistry, HelperExecutable: helper, Runner: runner})
		if e != nil {
			t.Fatal(e)
		}
		f.app.DependenciesRoot = root
		f.app.EnvironmentLockPath = filepath.Join(root, "environment.lock")
		f.app.EnvironmentStoreRoot = filepath.Join(root, "store")
		f.app.SecurityEvaluator = dependencyTestScanner{}
		source := &dependencyTestSource{Acquirer: sourceacquisition.Acquirer{TempRoot: t.TempDir(), URLForRepo: func(string) string { return repo }}}
		f.app.SourceAcquirer = source
		if e := os.MkdirAll(root, 0700); e != nil {
			t.Fatal(e)
		}
		return f, source
	}
	f, source := setup()
	manifest := []byte("schema_version = 1\n[dependencies.demo]\ngit = \"acme/demo\"\nref = \"main\"\n[targets]\ncodex = [\"demo\"]\n")
	mp, lp := f.app.dependencyPaths()
	if e := os.WriteFile(mp, manifest, 0600); e != nil {
		t.Fatal(e)
	}
	run := func(f cliFixture, args ...string) string {
		t.Helper()
		out, stderr, e := f.execute(false, args...)
		if e != nil {
			t.Fatalf("%v: %v\n%s\n%s", args, e, out, stderr)
		}
		return out
	}
	run(f, "deps", "lock", "--dry-run", "--format", "json")
	run(f, "deps", "sync", "--dry-run", "--yes", "--format", "json")
	for _, args := range [][]string{{"deps", "lock", "--scope", "project"}, {"deps", "lock", "--target", "codex"}} {
		if _, _, err := f.execute(false, args...); err == nil {
			t.Fatalf("accepted unsupported environment selection: %v", args)
		}
	}
	for _, p := range []string{lp, f.store.Path, f.app.EnvironmentStoreRoot, f.app.EnvironmentLockPath} {
		if _, e := os.Stat(p); !os.IsNotExist(e) {
			t.Fatalf("dry run wrote %s", p)
		}
	}
	run(f, "deps", "lock", "--format", "json")
	runners[0].failNextCodexAdd = true
	if _, _, err := f.execute(false, "deps", "sync", "--locked", "--yes", "--format", "json"); err == nil {
		t.Fatal("injected post-materialization activation failure reported success")
	}
	failedView, err := f.app.Installer.Inspect(context.Background())
	if err != nil || failedView.Recovery.Required || len(failedView.EnvironmentReceipts) != 1 {
		t.Fatalf("expected committed recoverable lifecycle failure without pending transaction: %+v %v", failedView, err)
	}
	if status := run(f, "deps", "status", "--format", "json"); !strings.Contains(status, "partial") {
		t.Fatal("failed lifecycle hidden: " + status)
	}
	failedProjection := failedView.Installations[0].Bindings[0].ProjectionDigest
	run(f, "deps", "sync", "--locked", "--yes", "--format", "json")
	resumedView, err := f.app.Installer.Inspect(context.Background())
	if err != nil || resumedView.Installations[0].Bindings[0].Activation != string(domain.ActivationActive) || resumedView.Installations[0].Bindings[0].Verification != "installation_verified" || resumedView.Installations[0].Bindings[0].ProjectionDigest != failedProjection {
		t.Fatalf("same-pin retry did not resume lifecycle: %+v %v", resumedView, err)
	}
	if status := run(f, "deps", "status", "--format", "json"); !strings.Contains(status, "partial") {
		t.Fatal("unchecked authentication must remain visible: " + status)
	}
	beforeDownloads, beforeLookups := source.downloads, source.lookups
	run(f, "deps", "sync", "--locked", "--yes", "--format", "json")
	run(f, "deps", "lock", "--format", "json")
	if source.downloads != beforeDownloads || source.lookups != beforeLookups {
		t.Fatal("intact repeat fetched source or resolved ref")
	}
	if err := os.WriteFile(mp, []byte(strings.ReplaceAll(string(manifest), `ref = "main"`, `ref = "different"`)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.execute(false, "deps", "sync", "--locked", "--yes", "--format", "json"); err == nil {
		t.Fatal("locked manifest mismatch accepted")
	}
	if err := os.WriteFile(mp, manifest, 0600); err != nil {
		t.Fatal(err)
	}
	if source.lookups != beforeLookups || source.downloads != beforeDownloads {
		t.Fatal("locked mismatch acquired source")
	}
	status := run(f, "deps", "status", "--format", "json")
	if strings.Contains(status, "mismatch") || strings.Contains(status, "not_synced") {
		t.Fatal(status)
	}
	state, e := f.store.Load()
	if e != nil || len(state.EnvironmentReceipts) != 1 {
		t.Fatalf("owned receipts: %+v %v", state, e)
	}
	id := state.EnvironmentReceipts[0].InstallationID
	var managed string
	for _, binding := range state.Installations[0].Clients {
		managed = binding.TargetLocator
	}
	reqAtStart, err := readDependencyManifest(mp)
	if err != nil {
		t.Fatal(err)
	}
	lockAtStart, err := readDependencyLock(lp, reqAtStart)
	if err != nil {
		t.Fatal(err)
	}
	pinned := lockAtStart.Packages[0]
	stored, err := f.app.dependencyStore().Get(context.Background(), pinned.DigestAlgorithm, pinned.TreeDigest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stored.Root, "store-tamper.txt"), []byte("corrupt original only"), 0600); err != nil {
		t.Fatal(err)
	}
	status = run(f, "deps", "status", "--format", "json")
	if !strings.Contains(status, "store_mismatch") || strings.Contains(status, "projection_mismatch") {
		t.Fatal("store/projection facts conflated: " + status)
	}
	savedLock, err := os.ReadFile(lp)
	if err != nil {
		t.Fatal(err)
	}
	independentEdit := filepath.Join(managed, "independent-edit.txt")
	if err := os.WriteFile(independentEdit, []byte("user change"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, agreement := range []string{"missing", "stale", "removed-assignment"} {
		if agreement == "missing" {
			if err := os.Remove(lp); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.WriteFile(mp, []byte("schema_version = 1\n[targets]\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if agreement == "removed-assignment" {
				run(f, "deps", "lock", "--format", "json")
			}
		}
		lookups, downloads := source.lookups, source.downloads
		report := run(f, "deps", "status", "--format", "json")
		for _, fact := range []string{"not_synced", "store_mismatch", "projection_mismatch"} {
			if !strings.Contains(report, fact) {
				t.Fatalf("%s hid %s: %s", agreement, fact, report)
			}
		}
		if source.lookups != lookups || source.downloads != downloads {
			t.Fatal("status performed source acquisition")
		}
		if err := os.WriteFile(mp, manifest, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(lp, savedLock, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(independentEdit); err != nil {
		t.Fatal(err)
	}
	run(f, "deps", "sync", "--locked", "--yes", "--format", "json")
	driftPath := filepath.Join(managed, "user-edit.txt")
	if e := os.WriteFile(driftPath, []byte("preserve before repair"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, _, e := f.execute(false, "deps", "sync", "--locked", "--yes", "--format", "json"); e == nil {
		t.Fatal("sync overwrote drift")
	}
	if _, e := os.Stat(driftPath); e != nil {
		t.Fatal("drift was overwritten")
	}
	run(f, "repair", id, "--target", "codex", "--format", "json")
	if source.lookups != beforeLookups {
		t.Fatal("exact repair performed ref lookup")
	}
	status = run(f, "deps", "status", "--format", "json")
	if strings.Contains(status, "mismatch") || strings.Contains(status, "not_synced") {
		t.Fatal(status)
	}
	if _, _, e := f.execute(false, "remove", id, "--target", "codex", "--format", "json"); e == nil {
		t.Fatal("imperative remove adopted managed authority")
	}
	// A mixed owned/foreign recovery scope is rejected as a whole, before repair
	// or recovery can touch any receipt. Then the owned-only scope is recoverable.
	state, e = f.store.Load()
	if e != nil {
		t.Fatal(e)
	}
	var pending domain.MutationReceipt
	for key, binding := range state.Installations[0].Clients {
		n := len(binding.Receipts) - 1
		binding.Receipts[n].Phase = "state_committed"
		pending = binding.Receipts[n]
		state.Installations[0].Clients[key] = binding
	}
	foreign := pending
	foreign.OperationID = "foreign-pending"
	foreign.EnvironmentAuthority = nil
	state.TransactionReceipts = []domain.MutationReceipt{foreign}
	if e := f.store.Save(state); e != nil {
		t.Fatal(e)
	}
	beforeRecovery, e := os.ReadFile(f.store.Path)
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e := f.execute(false, "deps", "recover", "--yes", "--format", "json"); e == nil {
		t.Fatal("mixed recovery was accepted")
	}
	if _, _, e := f.execute(false, "repair", id, "--target", "codex", "--format", "json"); e == nil {
		t.Fatal("repair bypassed foreign pending recovery")
	}
	afterRecovery, _ := os.ReadFile(f.store.Path)
	if string(beforeRecovery) != string(afterRecovery) {
		t.Fatal("refused recovery changed state")
	}
	state.TransactionReceipts = nil
	if e := f.store.Save(state); e != nil {
		t.Fatal(e)
	}
	run(f, "deps", "recover", "--yes", "--format", "json")
	view, e := f.app.Installer.Inspect(context.Background())
	if e != nil || view.Recovery.Required {
		t.Fatalf("owned recovery incomplete: %+v %v", view.Recovery, e)
	}

	if e := os.WriteFile(filepath.Join(repo, "revision.txt"), []byte("revision two"), 0600); e != nil {
		t.Fatal(e)
	}
	git("add", ".")
	git("commit", "-m", "test: advance disposable dependency")
	run(f, "deps", "update", "demo", "--format", "json")
	run(f, "deps", "sync", "--locked", "--yes", "--format", "json")
	if source.lookups != beforeLookups+1 {
		t.Fatalf("targeted update lookups %d", source.lookups)
	}
	transferred, e := os.ReadFile(lp)
	if e != nil {
		t.Fatal(e)
	}
	f2, _ := setup()
	mp2, lp2 := f2.app.dependencyPaths()
	if err := os.WriteFile(mp2, manifest, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lp2, transferred, 0600); err != nil {
		t.Fatal(err)
	}
	f2.app.SecurityEvaluator = dependencyTestScanner{fail: true}
	if _, _, e := f2.execute(false, "deps", "sync", "--locked", "--yes", "--accept-security-risk"); e == nil {
		t.Fatal("scanner failure bypassed")
	}
	f2.app.SecurityEvaluator = dependencyTestScanner{}
	run(f2, "deps", "sync", "--locked", "--yes", "--format", "json")
	status = run(f2, "deps", "status", "--format", "json")
	if strings.Contains(status, "mismatch") || strings.Contains(status, "not_synced") {
		t.Fatal(status)
	}
	if e := os.WriteFile(mp, []byte("schema_version = 1\n[targets]\n"), 0600); e != nil {
		t.Fatal(e)
	}
	run(f, "deps", "lock", "--format", "json")
	if _, _, err := f.execute(false, "deps", "sync", "--locked", "--yes", "--format", "json"); err == nil {
		t.Fatal("removal without external uninstall attestation reported success")
	}
	beforeRemoval, err := f.store.Load()
	if err != nil || len(beforeRemoval.EnvironmentReceipts) != 1 {
		t.Fatalf("incomplete removal dropped owner: %+v %v", beforeRemoval, err)
	}
	if _, err := os.Stat(managed); err != nil {
		t.Fatal("incomplete removal deleted projection")
	}
	run(f, "deps", "sync", "--locked", "--yes", "--external-uninstalled", "--format", "json")
	state, e = f.store.Load()
	if e != nil || len(state.EnvironmentReceipts) != 0 {
		t.Fatalf("terminal removal retained owner: %+v %v", state, e)
	}
	for _, installed := range state.Installations {
		for _, binding := range installed.Clients {
			if installed.InstallationID == id {
				t.Fatalf("terminal removal retained live binding: %+v", binding)
			}
		}
	}
	if _, err := os.Stat(managed); !os.IsNotExist(err) {
		t.Fatalf("removed projection still exists: %v", err)
	}
	if err := os.WriteFile(mp, manifest, 0600); err != nil {
		t.Fatal(err)
	}
	run(f, "deps", "lock", "--format", "json")
	run(f, "deps", "sync", "--locked", "--yes", "--format", "json")
	// A different immutable commit can contain identical bytes. Provenance must
	// advance even when the shared content-addressed store entry is reused.
	git("commit", "--allow-empty", "-m", "test: identical tree new commit")
	run(f, "deps", "update", "demo", "--format", "json")
	run(f, "deps", "sync", "--locked", "--yes", "--format", "json")
	status = run(f, "deps", "status", "--format", "json")
	if strings.Contains(status, "not_synced") {
		t.Fatal(status)
	}
	// Add another target without moving the source pin, then interrupt a real
	// per-binding update after one commit and retry from fresh inspection.
	multi := append(append([]byte{}, manifest...), []byte("claude = [\"demo\"]\n")...)
	if err := os.WriteFile(mp, multi, 0600); err != nil {
		t.Fatal(err)
	}
	beforeLookups = source.lookups
	run(f, "deps", "lock", "--format", "json")
	if source.lookups != beforeLookups {
		t.Fatal("target-only change moved pin")
	}
	run(f, "deps", "sync", "--locked", "--yes", "--format", "json")
	state, e = f.store.Load()
	if e != nil || len(state.EnvironmentReceipts) != 2 {
		t.Fatalf("multi-target receipts: %+v %v", state, e)
	}
	if err := os.WriteFile(filepath.Join(repo, "retry.txt"), []byte("next revision"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-m", "test: partial retry revision")
	run(f, "deps", "update", "demo", "--format", "json")
	reqs, e := readDependencyManifest(mp)
	if e != nil {
		t.Fatal(e)
	}
	locked, e := readDependencyLock(lp, reqs)
	if e != nil {
		t.Fatal(e)
	}
	view, e = f.app.Installer.Inspect(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	actions, e := dependencyActions(view, reqs, locked)
	if e != nil {
		t.Fatal(e)
	}
	detectedClients, e := f.app.Detector.Detect(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	detected := map[domain.ClientID]domain.DetectedClient{}
	for _, client := range detectedClients {
		detected[client.ClientID] = client
	}
	adapter := &dependencyInstaller{app: f.app, opts: &options{scope: "user", format: "json"}, cmd: &cobra.Command{}, detected: detected}
	interrupted := &dependencyInterruptedMutator{dependencyInstaller: adapter}
	results, e := (environment.Coordinator{Mutator: interrupted}).ApplySync(context.Background(), environment.PreparedSync{Bindings: actions}, true)
	if e == nil || len(results) != 2 || results[0].Outcome != installer.OutcomeCompleted {
		t.Fatalf("partial update: %+v %v", results, e)
	}
	run(f, "deps", "sync", "--locked", "--yes", "--format", "json")
	status = run(f, "deps", "status", "--format", "json")
	if strings.Contains(status, "mismatch") || strings.Contains(status, "not_synced") {
		t.Fatal(status)
	}

	// Editing the managed bytes after SDK Prepare must invalidate the handle.
	if err := os.WriteFile(filepath.Join(repo, "stale.txt"), []byte("next pin"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-m", "test: stale plan revision")
	run(f, "deps", "update", "demo", "--format", "json")
	locked, e = readDependencyLock(lp, reqs)
	if e != nil {
		t.Fatal(e)
	}
	view, e = f.app.Installer.Inspect(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	actions, e = dependencyActions(view, reqs, locked)
	if e != nil {
		t.Fatal(e)
	}
	handle, e := adapter.Prepare(context.Background(), actions[0])
	if e != nil {
		t.Fatal(e)
	}
	edited := filepath.Join(handle.Plan().TargetPath, "late-edit.txt")
	if err := os.WriteFile(edited, []byte("user edit"), 0600); err != nil {
		t.Fatal(err)
	}
	_, e = adapter.Apply(context.Background(), handle)
	_ = handle.Close()
	if e == nil {
		t.Fatal("stale prepared plan overwrote edited projection")
	}
	if body, err := os.ReadFile(edited); err != nil || string(body) != "user edit" {
		t.Fatal("late edit was overwritten")
	}
	if err := os.Remove(edited); err != nil {
		t.Fatal(err)
	}
	run(f, "deps", "sync", "--locked", "--yes", "--format", "json")

}

type dependencyInterruptedMutator struct {
	*dependencyInstaller
	applied int
}

func (m *dependencyInterruptedMutator) Apply(ctx context.Context, prepared *installer.PreparedOperation) (installer.Result, error) {
	m.applied++
	if m.applied == 2 {
		return installer.Result{}, errors.New("injected stop before second target effects")
	}
	return m.dependencyInstaller.Apply(ctx, prepared)
}

// The public process-runner seam supplies exact disposable Claude listing
// evidence. No external agent is launched by this integration test.
type dependencyClaudeListing struct {
	root             string
	codexID          string
	failNextCodexAdd bool
}

func (r *dependencyClaudeListing) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	for _, value := range command.Env {
		if strings.HasPrefix(value, "CODEX_HOME=") {
			args := command.Argv[1:]
			if len(args) >= 3 && args[0] == "plugin" && args[1] == "add" {
				if r.failNextCodexAdd {
					r.failNextCodexAdd = false
					return legacyports.CommandResult{ExitCode: 1, Stderr: []byte("injected activation failure")}, nil
				}
				r.codexID = args[2]
			}
			if len(args) >= 2 && args[0] == "plugin" && args[1] == "list" {
				installed := []map[string]any{}
				if r.codexID != "" {
					name, market, _ := strings.Cut(r.codexID, "@")
					installed = append(installed, map[string]any{"pluginId": r.codexID, "name": name, "marketplaceName": market, "installed": true, "enabled": true})
				}
				body, err := json.Marshal(map[string]any{"installed": installed})
				return legacyports.CommandResult{Stdout: body}, err
			}
			return legacyports.CommandResult{Stdout: []byte(`{"ok":true}`)}, nil
		}
	}
	if strings.Join(command.Argv[1:], " ") != "plugin list --json" {
		return legacyports.CommandResult{}, fmt.Errorf("unexpected fixture command: %v", command.Argv[1:])
	}
	entries, err := os.ReadDir(filepath.Join(r.root, "skills"))
	if os.IsNotExist(err) {
		return legacyports.CommandResult{Stdout: []byte("[]")}, nil
	}
	if err != nil {
		return legacyports.CommandResult{}, err
	}
	listed := []map[string]any{}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(r.root, "skills", entry.Name())
		body, err := os.ReadFile(filepath.Join(path, ".claude-plugin", "plugin.json"))
		if err != nil {
			continue
		}
		var manifest map[string]any
		if err := json.Unmarshal(body, &manifest); err != nil {
			return legacyports.CommandResult{}, err
		}
		listed = append(listed, map[string]any{"id": manifest["name"].(string) + "@skills-dir", "version": manifest["version"], "scope": "user", "enabled": true, "installPath": path})
	}
	body, err := json.Marshal(listed)
	return legacyports.CommandResult{Stdout: body}, err
}

func (s *dependencyTestSource) ResolveGitHubRef(ctx context.Context, repo, ref string) (sourceacquisition.RefResolution, error) {
	s.lookups++
	return s.Acquirer.ResolveGitHubRef(ctx, repo, ref)
}
