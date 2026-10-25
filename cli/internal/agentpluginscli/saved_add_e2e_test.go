package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type savedJourneySource struct {
	sourceacquisition.Acquirer
	downloads, refs, selectors int
}

func (s *savedJourneySource) AcquireGitHub(ctx context.Context, repo, rev, path string) (domain.PackageSnapshot, error) {
	s.downloads++
	return s.Acquirer.AcquireGitHub(ctx, repo, rev, path)
}
func (s *savedJourneySource) AcquireGitHubVerified(ctx context.Context, repo, rev, path, digest string) (domain.PackageSnapshot, error) {
	s.downloads++
	return s.Acquirer.AcquireGitHubVerified(ctx, repo, rev, path, digest)
}
func (s *savedJourneySource) ResolveGitHubRef(ctx context.Context, repo, ref string) (sourceacquisition.RefResolution, error) {
	s.refs++
	return s.Acquirer.ResolveGitHubRef(ctx, repo, ref)
}
func (s *savedJourneySource) ResolveGitHubSelector(ctx context.Context, repo, selector string) (sourceacquisition.RefResolution, error) {
	s.selectors++
	return s.Acquirer.ResolveGitHubSelector(ctx, repo, selector)
}

func (s *savedJourneySource) AcquireGit(ctx context.Context, source sourcespec.Git, commit, digest string) (domain.PackageSnapshot, error) {
	s.downloads++
	return s.Acquirer.AcquireGit(ctx, source, commit, digest)
}
func (s *savedJourneySource) ResolveGit(ctx context.Context, source sourcespec.Git, selector string, exact bool) (sourceacquisition.RefResolution, error) {
	if exact {
		s.refs++
	} else {
		s.selectors++
	}
	return s.Acquirer.ResolveGit(ctx, source, selector, exact)
}

type savedJourneyRunner struct {
	inner dependencyClaudeListing
	calls int
}

func (r *savedJourneyRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	r.calls++
	return r.inner.Run(ctx, command)
}

type savedJourney struct {
	cli    cliFixture
	runner *savedJourneyRunner
	source *savedJourneySource
}

func newSavedJourney(t *testing.T, repo string) savedJourney {
	t.Helper()
	codex, claude := fixtureClient(t, domain.ClientCodex), fixtureClient(t, domain.ClientClaude)
	codex.ExecutablePath = filepath.Join(t.TempDir(), "codex")
	claude.ExecutablePath = filepath.Join(t.TempDir(), "claude")
	f := newCLIFixture(t, []domain.DetectedClient{codex, claude})
	root := filepath.Join(f.root, "data")
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	runner := &savedJourneyRunner{inner: dependencyClaudeListing{root: claude.ConfigRoot}}
	f.app.Installer, err = installer.New(installer.Config{StateRoot: root, StateFile: f.store.Path, LockFile: filepath.Join(root, "mutation.lock"), OperationsDir: f.operations, PluginDataBase: filepath.Join(root, "plugin-data"), ManagedRoot: f.app.ManagedRoot, TempRoot: filepath.Join(root, "installer-tmp"), Registry: f.app.ClientRegistry, HelperExecutable: helper, Runner: runner})
	if err != nil {
		t.Fatal(err)
	}
	f.app.DependenciesRoot = root
	f.app.EnvironmentLockPath = filepath.Join(root, "environment.lock")
	f.app.EnvironmentStoreRoot = filepath.Join(root, "store")
	f.app.SecurityEvaluator = dependencyTestScanner{}
	source := &savedJourneySource{Acquirer: sourceacquisition.Acquirer{TempRoot: t.TempDir(), URLForRepo: func(string) string { return repo }}}
	diagnostic := &dependencyGitDiagnosticRunner{delegate: sourceacquisition.OSRunner{}, report: func(phase, class, status string) {
		t.Logf("saved-source Git diagnostic: phase=%s class=%s exit_status=%s", phase, class, status)
	}}
	source.Runner = diagnostic
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("saved-source Git diagnostic: delegated_failures=%d", diagnostic.failures.Load())
		}
	})
	f.app.SourceAcquirer = source
	return savedJourney{cli: f, runner: runner, source: source}
}
func (j savedJourney) run(t *testing.T, args ...string) string {
	t.Helper()
	out, stderr, err := j.cli.execute(false, args...)
	if err != nil {
		t.Fatalf("%v: %v\n%s\n%s", args, err, out, stderr)
	}
	return out
}
func savedJourneyGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", append([]string{"-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=iliya", "GIT_AUTHOR_EMAIL=iliyazelenkog@gmail.com", "GIT_COMMITTER_NAME=iliya", "GIT_COMMITTER_EMAIL=iliyazelenkog@gmail.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s %v", args, out, err)
	}
	return strings.TrimSpace(string(out))
}
func savedJourneyRepository(t *testing.T) (string, string) {
	t.Helper()
	repo := writeCLIPlugin(t)
	writeCLIMCP(t, repo)
	savedJourneyGit(t, repo, "init", "-b", "main")
	savedJourneyGit(t, repo, "add", ".")
	savedJourneyGit(t, repo, "commit", "-m", "test: disposable saved dependency")
	return repo, savedJourneyGit(t, repo, "rev-parse", "HEAD")
}
func (j savedJourney) state(t *testing.T) domain.StateFileV2 {
	t.Helper()
	state, err := j.cli.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	return state
}
func (j savedJourney) intent(t *testing.T) (environment.Requirements, environment.Lock) {
	t.Helper()
	mp, lp := j.cli.app.dependencyPaths()
	r, err := readDependencyManifest(mp)
	if err != nil {
		t.Fatal(err)
	}
	l, err := readDependencyLock(lp, r)
	if err != nil {
		t.Fatal(err)
	}
	return r, l
}
func savedJourneyBinding(t *testing.T, state domain.StateFileV2, client string) domain.ClientBinding {
	t.Helper()
	for _, i := range state.Installations {
		for _, b := range i.Clients {
			if b.ClientID == client {
				return b
			}
		}
	}
	t.Fatalf("missing %s binding", client)
	return domain.ClientBinding{}
}

type savedJourneyFile struct {
	Body     string
	Mode     fs.FileMode
	Modified time.Time
}

func savedJourneyTree(t *testing.T, root string) map[string]savedJourneyFile {
	t.Helper()
	out := map[string]savedJourneyFile{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if os.IsNotExist(err) {
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
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out[rel] = savedJourneyFile{Body: string(body), Mode: info.Mode(), Modified: info.ModTime()}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// This is fixture evidence: real Cobra, Git, loaders, store and SDK transaction;
// client commands and security scanning use deterministic disposable fakes.
func TestSavedAddGitLifecycleAndPortableRestore(t *testing.T) {
	repo, sha := savedJourneyRepository(t)
	j := newSavedJourney(t, repo)
	source := "github:acme/demo@" + sha
	mp, lp := j.cli.app.dependencyPaths()
	if _, err := os.Stat(j.cli.app.DependenciesRoot); !os.IsNotExist(err) {
		t.Fatal("fixture root is not cold")
	}
	out := j.run(t, "add", source, "--target", "codex", "--format", "json")
	var added struct {
		Data struct{ Environment struct{ Saved bool } }
	}
	if err := json.Unmarshal([]byte(out), &added); err != nil || !added.Data.Environment.Saved {
		t.Fatalf("saved add output: %s: %v", out, err)
	}
	j.intent(t)
	state := j.state(t)
	if len(state.EnvironmentReceipts) != 1 || j.source.downloads != 1 || j.source.refs+j.source.selectors != 0 {
		t.Fatalf("cold add: receipts=%d downloads=%d ref=%d selector=%d", len(state.EnvironmentReceipts), j.source.downloads, j.source.refs, j.source.selectors)
	}
	id := state.EnvironmentReceipts[0].InstallationID
	before := savedJourneyTree(t, j.cli.app.DependenciesRoot)
	calls := j.runner.calls
	downloads := j.source.downloads
	out = j.run(t, "add", source, "--target", "codex", "--format", "json")
	if !strings.Contains(out, "unchanged") || !reflect.DeepEqual(before, savedJourneyTree(t, j.cli.app.DependenciesRoot)) || calls != j.runner.calls || downloads != j.source.downloads || j.source.refs+j.source.selectors != 0 {
		t.Fatalf("same add changed environment: runner %d→%d downloads %d→%d refs %d selectors %d; files %s; %s", calls, j.runner.calls, downloads, j.source.downloads, j.source.refs, j.source.selectors, savedJourneyDifferences(before, savedJourneyTree(t, j.cli.app.DependenciesRoot)), out)
	}
	body, err := os.ReadFile(mp)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mp, append([]byte("# owner comment survives\n"), body...), 0600); err != nil {
		t.Fatal(err)
	}
	codexBefore := savedJourneyBinding(t, state, "codex")
	j.run(t, "add", source, "--target", "claude", "--format", "json")
	body, _ = os.ReadFile(mp)
	state = j.state(t)
	if !strings.Contains(string(body), "# owner comment survives") || !reflect.DeepEqual(codexBefore, savedJourneyBinding(t, state, "codex")) {
		t.Fatal("new target rewrote comment or existing Codex binding")
	}
	if len(state.EnvironmentReceipts) != 2 {
		t.Fatal("target add did not create exactly two owned receipts")
	}
	for _, receipt := range state.EnvironmentReceipts {
		if receipt.InstallationID != id {
			t.Fatal("target add changed owned installation identity")
		}
	}
	claudeBefore := savedJourneyBinding(t, state, "claude")
	j.run(t, "add", source, "--target", "codex", "--auth-complete", "--format", "json")
	state = j.state(t)
	codex := savedJourneyBinding(t, state, "codex")
	if codex.Authentication != domain.AuthenticationComplete || !reflect.DeepEqual(claudeBefore, savedJourneyBinding(t, state, "claude")) {
		t.Fatal("attestation did not stay scoped to Codex")
	}
	before = savedJourneyTree(t, j.cli.app.DependenciesRoot)
	calls = j.runner.calls
	j.run(t, "add", source, "--target", "codex", "--format", "json")
	if !reflect.DeepEqual(before, savedJourneyTree(t, j.cli.app.DependenciesRoot)) || calls != j.runner.calls {
		t.Fatal("repeat after attestation changed environment")
	}
	body, _ = os.ReadFile(mp)
	body = []byte(strings.Replace(string(body), `commit = "`+sha+`"`, `ref = "main"`, 1))
	if err := os.WriteFile(mp, body, 0600); err != nil {
		t.Fatal(err)
	}
	j.run(t, "deps", "sync", "--yes", "--format", "json")
	if j.source.refs == 0 || j.source.selectors != 0 {
		t.Fatal("moving ref did not use explicit ref lookup")
	}
	if err := os.WriteFile(filepath.Join(repo, "next.txt"), []byte("next pin"), 0600); err != nil {
		t.Fatal(err)
	}
	savedJourneyGit(t, repo, "add", ".")
	savedJourneyGit(t, repo, "commit", "-m", "test: next saved dependency pin")
	refs := j.source.refs
	j.run(t, "update", "demo", "--format", "json")
	if j.source.refs != refs+1 {
		t.Fatalf("update lookups=%d want %d", j.source.refs, refs+1)
	}
	_, lock := j.intent(t)
	state = j.state(t)
	for _, receipt := range state.EnvironmentReceipts {
		if receipt.PackageDigest != lock.Packages[0].TreeDigest {
			t.Fatal("target retained old pin")
		}
	}
	j.run(t, "remove", "demo", "--target", "claude", "--external-uninstalled", "--format", "json")
	requirements, _ := j.intent(t)
	if _, ok := requirements.Dependencies["demo"]; !ok {
		t.Fatal("target removal removed dependency")
	}
	if _, _, err := j.cli.execute(false, "remove", "demo", "--format", "json"); err == nil {
		t.Fatal("manual Codex removal incorrectly reported terminal success")
	}
	requirements, _ = j.intent(t)
	if _, ok := requirements.Dependencies["demo"]; ok {
		t.Fatal("last target retained intent")
	}
	if len(j.state(t).EnvironmentReceipts) != 1 {
		t.Fatal("partial last removal dropped its live receipt")
	}
	downloads, refs = j.source.downloads, j.source.refs
	j.run(t, "remove", "demo", "--external-uninstalled", "--format", "json")
	state = j.state(t)
	if len(state.EnvironmentReceipts) != 0 || downloads != j.source.downloads || refs != j.source.refs {
		t.Fatal("receipt-only removal fetched source or retained ownership")
	}
	if _, err := os.Stat(j.cli.app.EnvironmentStoreRoot); err != nil {
		t.Fatal("removal deleted store")
	}
	retained := false
	for _, i := range state.Installations {
		if i.InstallationID == id {
			retained = i.DataRetained
			for _, d := range i.DataReceipts {
				if _, err := os.Stat(d.Locator); err != nil {
					t.Fatal("removal deleted PLUGIN_DATA")
				}
			}
		}
	}
	if !retained {
		t.Fatal("removed installation did not retain data")
	}
	source = "github:acme/demo@" + savedJourneyGit(t, repo, "rev-parse", "HEAD")
	j.run(t, "add", source, "--target", "codex", "--format", "json")
	_, lock = j.intent(t)
	second := newSavedJourney(t, repo)
	mp2, lp2 := second.cli.app.dependencyPaths()
	if err := os.MkdirAll(filepath.Dir(mp2), 0700); err != nil {
		t.Fatal(err)
	}
	for src, dst := range map[string]string{mp: mp2, lp: lp2} {
		body, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	second.run(t, "deps", "sync", "--locked", "--yes", "--format", "json")
	if second.source.refs+second.source.selectors != 0 {
		t.Fatal("locked restore resolved a selector")
	}
	for _, r := range second.state(t).EnvironmentReceipts {
		if r.PackageDigest != lock.Packages[0].TreeDigest {
			t.Fatal("restore digest differs")
		}
	}
	before = savedJourneyTree(t, second.cli.app.DependenciesRoot)
	calls = second.runner.calls
	downloads = second.source.downloads
	second.run(t, "deps", "sync", "--locked", "--yes", "--format", "json")
	if !reflect.DeepEqual(before, savedJourneyTree(t, second.cli.app.DependenciesRoot)) || calls != second.runner.calls || downloads != second.source.downloads {
		t.Fatal("locked restore repeat changed environment")
	}
}

func TestSavedAddPublicationFailureRequiresReconcile(t *testing.T) {
	repo, sha := savedJourneyRepository(t)
	j := newSavedJourney(t, repo)
	mp, lp := j.cli.app.dependencyPaths()
	publish := publishEnvironmentFile
	defer func() { publishEnvironmentFile = publish }()
	publishEnvironmentFile = func(path string, body []byte) error {
		if path == lp {
			return errors.New("injected lock publication failure")
		}
		return publish(path, body)
	}
	source := "github:acme/demo@" + sha
	_, _, err := j.cli.execute(false, "add", source, "--target", "codex", "--format", "json")
	if err == nil || !strings.Contains(err.Error(), "deps lock") {
		t.Fatalf("failure lacks reconciliation action: %v", err)
	}
	if _, err := os.Stat(mp); err != nil {
		t.Fatal("manifest was not published first")
	}
	for _, path := range []string{lp, j.cli.store.Path} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("failed publication changed %s: %v", path, err)
		}
	}
	if j.runner.calls != 0 {
		t.Fatal("publication failure reached client effects")
	}
	publishEnvironmentFile = publish
	downloads := j.source.downloads
	if _, _, err := j.cli.execute(false, "add", source, "--target", "codex", "--format", "json"); err == nil || !strings.Contains(err.Error(), "deps lock") {
		t.Fatalf("unreconciled repeat: %v", err)
	}
	if downloads != j.source.downloads {
		t.Fatal("unreconciled retry acquired source")
	}
	j.run(t, "deps", "lock", "--format", "json")
	j.run(t, "add", source, "--target", "codex", "--format", "json")
	if len(j.state(t).EnvironmentReceipts) != 1 {
		t.Fatal("reconciled add did not install")
	}
}

type savedRegistrySource struct {
	*savedJourneySource
	directory SourceAcquirer
}

func (s savedRegistrySource) AcquireGitHub(ctx context.Context, repo, rev, path string) (domain.PackageSnapshot, error) {
	if repo == "owner/rollout" || repo == "second/rollout" {
		return s.directory.AcquireGitHub(ctx, repo, rev, path)
	}
	return s.savedJourneySource.AcquireGitHub(ctx, repo, rev, path)
}
func (s savedRegistrySource) AcquireGitHubVerified(ctx context.Context, repo, rev, path, digest string) (domain.PackageSnapshot, error) {
	if repo == "owner/rollout" || repo == "second/rollout" {
		return s.directory.AcquireGitHubVerified(ctx, repo, rev, path, digest)
	}
	return s.savedJourneySource.AcquireGitHubVerified(ctx, repo, rev, path, digest)
}

func TestSavedAddRegistryPinsEligibilityAndRestore(t *testing.T) {
	fixture := newRolloutDirectoryFixture(t, []domain.ClientID{domain.ClientCodex, domain.ClientClaude}, []domain.ClientID{domain.ClientCodex, domain.ClientClaude})
	repo, _ := savedJourneyRepository(t)
	// This second source has its own declared name and namespace.
	manifest := filepath.Join(repo, "plugin.json")
	body, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte(strings.Replace(string(body), `"name": "demo"`, `"name": "other"`, 1)), 0600); err != nil {
		t.Fatal(err)
	}
	savedJourneyGit(t, repo, "add", ".")
	savedJourneyGit(t, repo, "commit", "-m", "test: distinct second saved dependency")
	sha := savedJourneyGit(t, repo, "rev-parse", "HEAD")
	setup := func() savedJourney {
		j := newSavedJourney(t, repo)
		j.cli.app.DirectoryClient = fixture.directory
		j.cli.app.SourceAcquirer = savedRegistrySource{savedJourneySource: j.source, directory: fixture.acquirer}
		return j
	}
	j := setup()
	j.run(t, "add", "rollout-demo", "--alias", "docs", "--target", "codex", "--format", "json")
	requirements, lock := j.intent(t)
	if requirements.Dependencies["docs"].Registry.Distribution != "owner/rollout" || requirements.Dependencies["docs"].Registry.Selector.Value != "1" || lock.Packages[0].Source.Registry.ReleaseSequence != 1 {
		t.Fatal("registry pin was not canonical release 1")
	}
	before := savedJourneyTree(t, j.cli.app.DependenciesRoot)
	directoryCalls, acquisitions, runnerCalls := fixture.directory.calls, fixture.acquirer.verifiedCalls, j.runner.calls
	j.run(t, "add", "rollout-demo", "--target", "codex", "--format", "json")
	if directoryCalls != fixture.directory.calls || acquisitions != fixture.acquirer.verifiedCalls || runnerCalls != j.runner.calls || !reflect.DeepEqual(before, savedJourneyTree(t, j.cli.app.DependenciesRoot)) {
		t.Fatalf("registry repeat changed environment: directory %d→%d acquisition %d→%d runner %d→%d files %s", directoryCalls, fixture.directory.calls, acquisitions, fixture.acquirer.verifiedCalls, runnerCalls, j.runner.calls, savedJourneyDifferences(before, savedJourneyTree(t, j.cli.app.DependenciesRoot)))
	}
	// A second saved distribution shares the product and basename. Short input
	// must fail deterministically, even though each canonical source is valid.
	alternativeBytes, err := json.Marshal(fixture.directory.bundle.Snapshot.Distributions[0])
	if err != nil {
		t.Fatal(err)
	}
	var alternative domain.DirectoryDistribution
	if err := json.Unmarshal(alternativeBytes, &alternative); err != nil {
		t.Fatal(err)
	}
	alternative.ID = "second/rollout"
	alternative.ProductID = "alternate-product"
	alternative.Releases = alternative.Releases[:1]
	alternative.ReleasePolicies = alternative.ReleasePolicies[:1]
	alternateRoot := writeCLIPlugin(t)
	alternateManifest := filepath.Join(alternateRoot, "plugin.json")
	alternateBody, err := os.ReadFile(alternateManifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(alternateManifest, []byte(strings.Replace(string(alternateBody), `"name": "demo"`, `"name": "alternate-demo"`, 1)), 0600); err != nil {
		t.Fatal(err)
	}
	alternateLoaded, err := fixture.cli.app.acquireLocal(context.Background(), alternateRoot)
	if err != nil {
		t.Fatal(err)
	}
	alternateRevision := strings.Repeat("3", 40)
	fixture.acquirer.revisionRoots[alternateRevision] = alternateRoot
	alternative.Releases[0].PackageSource.Repository = "second/rollout"
	alternative.Releases[0].PackageSource.Revision = alternateRevision
	alternative.Releases[0].ManifestName = "alternate-demo"
	alternative.Releases[0].TreeDigest = alternateLoaded.envelope.TreeDigest
	alternative.Releases[0].ManifestDigest = alternateLoaded.envelope.ManifestDigest
	if err := alternateLoaded.cleanup(); err != nil {
		t.Fatal(err)
	}
	for i := range alternative.ReleasePolicies {
		policy := &alternative.ReleasePolicies[i]
		for k, oldID := range policy.CurrentEvidence {
			newID := "alternative/" + oldID
			policy.CurrentEvidence[k] = newID
			for _, evidence := range fixture.directory.bundle.Snapshot.Evidence {
				if evidence.ID != oldID {
					continue
				}
				evidence.ID = newID
				evidence.DistributionID = alternative.ID
				evidence.PackageTreeDigest = alternative.Releases[0].TreeDigest
				fixture.directory.bundle.Snapshot.Evidence = append(fixture.directory.bundle.Snapshot.Evidence, evidence)
				break
			}
		}
	}
	fixture.directory.bundle.Snapshot.Distributions = append(fixture.directory.bundle.Snapshot.Distributions, alternative)
	alternateProduct := fixture.directory.bundle.Snapshot.Products[0]
	alternateProduct.ID = "alternate-product"
	alternateProduct.ManifestName = "alternate-demo"
	alternateProduct.DefaultDistribution = alternative.ID
	alternateProduct.Distributions = []string{alternative.ID}
	alternateProduct.Aliases = []string{"alternate-product"}
	alternateProduct.ReservedAliases = []string{"alternate-product"}
	fixture.directory.bundle.Snapshot.Products = append(fixture.directory.bundle.Snapshot.Products, alternateProduct)
	j.run(t, "add", "second/rollout", "--alias", "alternate", "--target", "claude", "--format", "json")
	directoryCalls = fixture.directory.calls
	_, _, err = j.cli.execute(false, "add", "rollout", "--target", "codex", "--format", "json")
	if err == nil || !strings.Contains(err.Error(), "docs") || !strings.Contains(err.Error(), "alternate") || directoryCalls != fixture.directory.calls {
		t.Fatalf("ambiguous registry short input: %v", err)
	}
	j.run(t, "remove", "alternate", "--external-uninstalled", "--format", "json")
	j.run(t, "add", "github:acme/other@"+sha, "--alias", "other", "--target", "claude", "--format", "json")
	_, lock = j.intent(t)
	other, _ := lock.Package("other")
	otherBytes, _ := json.Marshal(other)
	fixture.publishV2()
	fixture.directory.bundle.Snapshot.Distributions[0].ReleasePolicies[0].Status = domain.ReleaseSuperseded
	j.run(t, "deps", "lock", "--format", "json")
	j.run(t, "update", "other", "--format", "json")
	j.run(t, "deps", "sync", "--yes", "--format", "json")
	mp, lp := j.cli.app.dependencyPaths()
	oldManifest, err := os.ReadFile(mp)
	if err != nil {
		t.Fatal(err)
	}
	oldLock, err := os.ReadFile(lp)
	if err != nil {
		t.Fatal(err)
	}
	fresh := setup()
	freshMP, freshLP := fresh.cli.app.dependencyPaths()
	if err := os.MkdirAll(filepath.Dir(freshMP), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(freshMP, oldManifest, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(freshLP, oldLock, 0600); err != nil {
		t.Fatal(err)
	}
	fresh.run(t, "deps", "sync", "--locked", "--yes", "--format", "json")
	_, freshLock := fresh.intent(t)
	var copiedLock environment.Lock
	if err := json.Unmarshal(oldLock, &copiedLock); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(freshLock, copiedLock) || !bytes.Equal(oldManifest, []byte(dependencyRead(t, freshMP))) || !bytes.Equal(oldLock, []byte(dependencyRead(t, freshLP))) {
		t.Fatal("copied superseded lock restore moved pins or rewrote the portable agreement")
	}
	if len(fresh.state(t).EnvironmentReceipts) != len(copiedLock.Packages) || fresh.source.refs+fresh.source.selectors != 0 {
		t.Fatal("copied superseded lock restore omitted bindings or resolved refs")
	}
	j.run(t, "update", "docs", "--format", "json")
	requirements, lock = j.intent(t)
	updated, _ := lock.Package("docs")
	afterOther, _ := lock.Package("other")
	afterOtherBytes, _ := json.Marshal(afterOther)
	if updated.Source.Registry.ReleaseSequence != 2 || requirements.Dependencies["docs"].Registry.Selector.Value != "2" || !bytes.Equal(otherBytes, afterOtherBytes) {
		t.Fatal("registry update did not preserve other pin")
	}
	restored := setup()
	restoredMP, restoredLP := restored.cli.app.dependencyPaths()
	if err := os.MkdirAll(filepath.Dir(restoredMP), 0700); err != nil {
		t.Fatal(err)
	}
	for src, dst := range map[string]string{mp: restoredMP, lp: restoredLP} {
		body, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	restored.run(t, "deps", "sync", "--locked", "--yes", "--format", "json")
	if restored.source.refs+restored.source.selectors != 0 {
		t.Fatal("registry restore resolved Git refs")
	}
	restoredState := restored.state(t)
	if len(restoredState.EnvironmentReceipts) != 2 {
		t.Fatal("registry restore missing receipts")
	}
	for _, receipt := range restoredState.EnvironmentReceipts {
		pkg, ok := lock.Package(receipt.Alias)
		if !ok || receipt.PackageDigest != pkg.TreeDigest {
			t.Fatal("restored receipt differs from locked digest")
		}
	}
}

func savedJourneyDifferences(before, after map[string]savedJourneyFile) string {
	var changes []string
	for path, file := range before {
		if got, ok := after[path]; !ok {
			changes = append(changes, path+" removed")
		} else if file != got {
			changes = append(changes, fmt.Sprintf("%s body_changed=%v modified=%s→%s", path, file.Body != got.Body, file.Modified, got.Modified))
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			changes = append(changes, path+" created")
		}
	}
	return strings.Join(changes, "; ")
}
