package agentpluginscli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
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
	run("install", "--format", "json")
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
	dependencyWrite(t, mp, manifest)
	tampered := strings.ReplaceAll(savedLock, lock.ManifestDigest, strings.Replace(lock.ManifestDigest, "sha256:", "sha256:0", 1))
	dependencyWrite(t, lp, tampered)
	if _, _, err := f.execute(false, "install", "--locked", "--yes", "--format", "json"); err == nil {
		t.Fatal("--locked accepted a changed lock")
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
