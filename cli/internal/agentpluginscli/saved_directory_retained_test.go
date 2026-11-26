package agentpluginscli

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// The Directory bundle, acquisition, native responses and scanner use existing
// fixtures; public CLI commands and installer transactions remain real. This
// qualifies retained-state policy, not signatures, native clients or Git.
func TestSavedDirectoryAddAfterImperativeRemoveRetainsIdentityAndData(t *testing.T) {
	for _, test := range []struct {
		name, refusal string
		change        func(*rolloutDirectoryFixture)
		newerRelease  bool
	}{
		{name: "same immutable release"},
		{name: "newer legitimate release cannot replace recorded pin", refusal: "no installed targets", newerRelease: true, change: func(f *rolloutDirectoryFixture) {
			f.publishV2()
		}},
		{name: "rebound revision with identical bytes", refusal: "package-source revision", change: func(f *rolloutDirectoryFixture) {
			revision := strings.Repeat("3", 40)
			f.directory.bundle.Snapshot.Distributions[0].Releases[0].PackageSource.Revision = revision
			f.acquirer.revisionRoots[revision] = f.v1.root
		}},
		{name: "changed bytes at retained revision", refusal: "package bytes", change: func(f *rolloutDirectoryFixture) {
			release := &f.directory.bundle.Snapshot.Distributions[0].Releases[0]
			release.TreeDigest, release.ManifestDigest = f.v2.tree, f.v2.manifest
			release.PackageVersion = f.v2.version
			f.directory.bundle.Snapshot.Evidence[0].PackageTreeDigest = f.v2.tree
			f.acquirer.revisionRoots[f.v1.revision] = f.v2.root
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rollout, fixture, retained, sentinel := retainedDirectoryFixture(t)
			if test.change != nil {
				test.change(&rollout)
				rollout.directory.bundle.Snapshot.Sequence++
			}
			if test.newerRelease {
				latest, err := domain.ResolveDirectory(rollout.directory.bundle.Snapshot, domain.DirectoryResolveRequest{
					Selector: "rollout-demo", Targets: []domain.ClientID{domain.ClientGemini}, Scope: domain.ScopeUser,
					InstallerVersion: fixture.app.Version, SchemaVersion: "1.0.0",
				})
				if err != nil || latest.ReleaseSequence != 2 || latest.Source.Revision != rollout.v2.revision || latest.TreeDigest != rollout.v2.tree {
					t.Fatalf("new release must be ordinarily eligible before retained selection: %+v %v", latest, err)
				}
			}
			manifest, lock := fixture.app.dependencyPaths()
			before := freshRestoreFiles(t, manifest, lock, fixture.store.Path, filepath.Dir(sentinel))
			dataBefore := freshRestoreFiles(t, filepath.Dir(sentinel))
			if test.newerRelease {
				assertRetainedDirectoryRevisionRefusal(t, fixture, retained, sentinel, test.refusal)
				return
			}
			out, stderr, err := fixture.execute(false, "add", "rollout-demo", "--target", "gemini", "--yes", "--format", "json")
			if test.refusal != "" {
				var foreign *environment.ForeignBindingError
				if err == nil || !strings.Contains(err.Error(), test.refusal) || errors.As(err, &foreign) {
					t.Fatalf("retained Directory policy changed: err=%v stdout=%s stderr=%s", err, out, stderr)
				}
				if errors.Is(err, domain.ErrRetainedRevisionChange) {
					t.Fatalf("immutable publication refusal was replaced by retained-state advice: %v", err)
				}
				if !reflect.DeepEqual(before, freshRestoreFiles(t, manifest, lock, fixture.store.Path, filepath.Dir(sentinel))) {
					t.Fatal("refusal changed retained state, intent, lock or PLUGIN_DATA")
				}
				return
			}
			if err != nil {
				t.Fatalf("same immutable Directory release refused: %v stdout=%s stderr=%s", err, out, stderr)
			}
			assertRetainedDirectoryReinstalled(t, fixture, retained, rollout.directory.bundle.Snapshot.Sequence, sentinel)
			if !reflect.DeepEqual(dataBefore, freshRestoreFiles(t, filepath.Dir(sentinel))) || rollout.acquirer.revisionCalls[rollout.v2.revision] != 0 {
				t.Fatal("same-pin add changed retained data metadata or acquired the newer release")
			}
		})
	}
}

func assertRetainedDirectoryRevisionRefusal(t *testing.T, fixture cliFixture, retained domain.Installation, sentinel, refusal string) {
	t.Helper()
	manifest, lock := fixture.app.dependencyPaths()
	before := freshRestoreFiles(t, manifest, lock, fixture.store.Path, filepath.Dir(sentinel))
	// The former advice really is unavailable: update has no installed target
	// after imperative remove-all. This is not a saved-remove intent fixture.
	stdout, stderr, err := fixture.execute(false, "update", retained.InstallationID, "--target", "gemini", "--yes", "--format", "json")
	if err == nil || !strings.Contains(err.Error(), `not installed for target "gemini" in user scope`) || errors.Is(err, domain.ErrRetainedRevisionChange) {
		t.Fatalf("empty installation unexpectedly has an ordinary update path: err=%v stdout=%s stderr=%s", err, stdout, stderr)
	}
	if !reflect.DeepEqual(before, freshRestoreFiles(t, manifest, lock, fixture.store.Path, filepath.Dir(sentinel))) {
		t.Fatal("refused update changed retained state, intent, lock or PLUGIN_DATA")
	}
	for _, command := range []string{"add", "install"} {
		for _, format := range []string{"human", "json"} {
			t.Run(command+"/"+format, func(t *testing.T) {
				args := []string{command, "rollout-demo", "--target", "gemini", "--yes", "--format", format}
				out, stderr, err := fixture.execute(false, args...)
				var preflight *dependencyPreflightError
				var foreign *environment.ForeignBindingError
				if !errors.Is(err, installer.ErrUpdateRequired) || !errors.Is(err, domain.ErrRetainedRevisionChange) || !errors.As(err, &preflight) || preflight.Alias != "demo" || preflight.ClientID != "gemini" || errors.As(err, &foreign) {
					t.Fatalf("retained revision refusal lost authoritative typing: err=%v stdout=%s stderr=%s", err, out, stderr)
				}
				text := ErrorText(args, nil, err)
				if !strings.Contains(text, refusal) || !strings.Contains(text, "not supported") || ShouldWriteAgentFailure(err) || format == "json" && strings.TrimSpace(out) != "" {
					t.Fatalf("retained refusal lost its diagnosis or changed its JSON contract: error=%s stdout=%s", text, out)
				}
				assertNoRetainedDirectoryUpdateAdvice(t, text+"\n"+out+"\n"+stderr)
				for _, failure := range []*installFailure{
					describeSyncFailure(err),
					describeBindingFailure(environment.BindingOutcome{Status: environment.BindingFailed, Err: err}),
				} {
					if failure.Kind != failureUpdateRequired || failure.Retryable {
						t.Fatalf("retained refusal changed the existing failure classification: %+v", failure)
					}
					assertNoRetainedDirectoryUpdateAdvice(t, failure.Summary+"\n"+failure.Next)
				}
				if !reflect.DeepEqual(before, freshRestoreFiles(t, manifest, lock, fixture.store.Path, filepath.Dir(sentinel))) {
					t.Fatal("refusal changed retained state, intent, lock or PLUGIN_DATA")
				}
			})
		}
	}
}

func assertNoRetainedDirectoryUpdateAdvice(t *testing.T, text string) {
	t.Helper()
	for _, forbidden := range []string{"use update", "run update separately", "agentplugins update", "--no-save", "--purge-data"} {
		if strings.Contains(strings.ToLower(text), forbidden) {
			t.Fatalf("retained empty installation offered an unavailable or destructive next step %q: %s", forbidden, text)
		}
	}
}

func retainedDirectoryFixture(t *testing.T) (rolloutDirectoryFixture, cliFixture, domain.Installation, string) {
	t.Helper()
	rollout := newRolloutDirectoryFixture(t, []domain.ClientID{domain.ClientGemini}, []domain.ClientID{domain.ClientGemini})
	retainedDirectoryDataPackages(t, &rollout)
	fixture, _, _ := dependencySpecHarness(t, rollout.v1.root, dependencyClientSpec{id: domain.ClientGemini, activation: "active"})
	fixture.app.DirectoryClient, fixture.app.SourceAcquirer = rollout.directory, rollout.acquirer
	dependencyRun(t, fixture, "add", "--no-save", "rollout-demo", "--target", "gemini", "--yes", "--format", "json")
	installed, err := fixture.store.Load()
	if err != nil || len(installed.Installations) != 1 || len(installed.EnvironmentReceipts) != 0 {
		t.Fatalf("imperative Directory installation: %+v %v", installed, err)
	}
	identity := installed.Installations[0]
	if identity.OriginMode != domain.OriginModeDirectory || identity.Directory == nil || identity.Directory.DistributionID != "owner/rollout" || identity.Source.ResolvedRevision != rollout.v1.revision || identity.Source.TreeDigest != rollout.v1.tree || len(identity.Clients) != 1 || len(identity.DataReceipts) != 1 {
		t.Fatal("imperative fixture lacks actual Directory pin, binding or PLUGIN_DATA", identity)
	}
	var sentinel string
	for _, receipt := range identity.DataReceipts {
		sentinel = filepath.Join(receipt.Locator, "retained-user-data.txt")
		dependencyWrite(t, sentinel, "retain these user bytes")
	}
	dataBefore := freshRestoreFiles(t, filepath.Dir(sentinel))
	dependencyRun(t, fixture, "remove", identity.InstallationID, "--target", "gemini", "--yes", "--format", "json")
	removed, err := fixture.store.Load()
	if err != nil || len(removed.Installations) != 1 || len(removed.EnvironmentReceipts) != 0 {
		t.Fatalf("imperative remove-all: %+v %v", removed, err)
	}
	retained := removed.Installations[0]
	if len(retained.Clients) != 0 || !retained.DataRetained || retained.InstallationID != identity.InstallationID || !reflect.DeepEqual(retained.Source, identity.Source) || !reflect.DeepEqual(retained.Directory, identity.Directory) || !reflect.DeepEqual(retained.Package, identity.Package) || !reflect.DeepEqual(retained.DataReceipts, identity.DataReceipts) || !reflect.DeepEqual(dataBefore, freshRestoreFiles(t, filepath.Dir(sentinel))) {
		t.Fatal("remove-all lost retained Directory identity, pin or data", retained)
	}
	manifest, lock := fixture.app.dependencyPaths()
	for _, path := range []string{manifest, lock} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("imperative lifecycle unexpectedly saved dependency intent", path, err)
		}
	}
	return rollout, fixture, retained, sentinel
}

func retainedDirectoryDataPackages(t *testing.T, rollout *rolloutDirectoryFixture) {
	t.Helper()
	for index, revision := range []*rolloutRevision{&rollout.v1, &rollout.v2} {
		dependencyWrite(t, filepath.Join(revision.root, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"demo":{"type":"stdio","command":"go","args":["version"],"env":{"DATA":"${PLUGIN_DATA}"}}}}`)
		loaded, err := rollout.cli.app.acquireLocal(t.Context(), revision.root)
		if err != nil {
			t.Fatal(err)
		}
		revision.tree, revision.manifest = loaded.envelope.TreeDigest, loaded.envelope.ManifestDigest
		if err := loaded.cleanup(); err != nil {
			t.Fatal(err)
		}
		release := &rollout.directory.bundle.Snapshot.Distributions[0].Releases[index]
		release.TreeDigest, release.ManifestDigest, release.Components = revision.tree, revision.manifest, []string{"mcp"}
		rollout.directory.bundle.Snapshot.Evidence[index].PackageTreeDigest = revision.tree
	}
}

func assertRetainedDirectoryReinstalled(t *testing.T, fixture cliFixture, retained domain.Installation, snapshotSequence uint64, sentinel string) {
	t.Helper()
	after, err := fixture.store.Load()
	if err != nil || len(after.Installations) != 1 || len(after.EnvironmentReceipts) != 1 {
		t.Fatalf("same-pin saved installation: %+v %v", after, err)
	}
	snapshot, err := snapshotEnvironmentContext(t.Context(), fixture.app)
	if err != nil || len(snapshot.Lock.Packages) != 1 {
		t.Fatalf("saved immutable Directory pin: %+v %v", snapshot.Lock, err)
	}
	pin := snapshot.Lock.Packages[0]
	if pin.Source.Registry == nil || pin.Source.Registry.DistributionID != retained.Directory.DistributionID || pin.Source.Registry.ReleaseSequence != 1 || pin.Source.Registry.Commit != retained.Source.ResolvedRevision || pin.TreeDigest != retained.Source.TreeDigest || pin.ManifestDigest != retained.Package.ManifestDigest {
		t.Fatal("saved add silently selected another Directory pin", pin)
	}
	// Directory re-add permits its existing source-hint reconstruction. Assert
	// the exact locked identity rather than treating its new ID as migration.
	identity := identityFromLocked(pin)
	expectedSource := retained.Source
	expectedSource.SourceBindingID = domain.ComputeSourceBindingID(identity)
	expectedSource.StableSourceKey, expectedSource.RequestedSource, expectedSource.CanonicalSource = identity.StableSourceKey, identity.RequestedSource, identity.CanonicalSource
	expectedDirectory := *retained.Directory
	expectedDirectory.SnapshotSequence = snapshotSequence
	installation := after.Installations[0]
	if installation.InstallationID != retained.InstallationID || !reflect.DeepEqual(installation.Source, expectedSource) || !reflect.DeepEqual(installation.Directory, &expectedDirectory) || !reflect.DeepEqual(installation.Package, retained.Package) || !reflect.DeepEqual(installation.DataReceipts, retained.DataReceipts) || len(installation.Clients) != 1 {
		t.Fatal("saved add changed retained Directory identity, immutable pin or data ownership", installation)
	}
	binding := onlyCLIClient(installation)
	receipt := after.EnvironmentReceipts[0]
	if receipt.InstallationID != retained.InstallationID || receipt.ClientBindingID != binding.ClientBindingID || receipt.ClientID != "gemini" || receipt.Alias != "demo" || receipt.SourceKey != "directory:owner/rollout" || receipt.PackageDigest != retained.Source.TreeDigest || receipt.EnvironmentID != fixture.app.environmentID() || dependencyRead(t, sentinel) != "retain these user bytes" {
		t.Fatal("saved add failed to bind exact retained installation and data", receipt)
	}
	t.Logf("retained IID=%s distribution=%s release=%d revision=%s tree=%s; SourceBindingID %s -> %s; one saved receipt and original PLUGIN_DATA preserved", retained.InstallationID, installation.Directory.DistributionID, installation.Directory.DesiredReleaseSequence, installation.Source.ResolvedRevision, installation.Source.TreeDigest, retained.Source.SourceBindingID, installation.Source.SourceBindingID)
}
