package agentpluginscli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func TestDirectSourcesWithSameTreeKeepSelectedLockProvenance(t *testing.T) {
	for _, order := range [][]string{{"a", "b"}, {"b", "a"}} {
		t.Run(strings.Join(order, "-"), func(t *testing.T) {
			transport := newDirectTransportFixture(t, "zip", false)
			sharedStore := filepath.Join(t.TempDir(), "shared-store")
			coldStore := filepath.Join(t.TempDir(), "shared-cold-store")
			var previousDigest, previousKey, previousSHA string
			for _, name := range order {
				transport.payload = directArchiveBytes(t, transport.work, "zip", "provider-"+name+"/")
				fixture := directCLIEnvironment(t, transport)
				fixture.app.EnvironmentStoreRoot = sharedStore
				endpoint := transport.server.URL + "/" + name + ".zip"
				dependencyRun(t, fixture, "add", endpoint, "--alias", name, "--target", "gemini", "--format", "json")
				pin := directLockedPin(t, fixture)
				if previousDigest != "" && (previousDigest != pin.TreeDigest || previousKey == pin.SourceKey || previousSHA == pin.Source.Archive.SHA256) {
					t.Fatal("byte-distinct, same-tree archive source facts collapsed")
				}
				previousDigest, previousKey = pin.TreeDigest, pin.SourceKey
				previousSHA = pin.Source.Archive.SHA256
				loaded, err := materializeLockedPackage(context.Background(), fixture.app, pin, false, nil)
				if err != nil {
					t.Fatal(err)
				}
				if loaded.envelope.Source != identityFromLocked(pin) {
					t.Fatalf("store supplied first-writer identity: %+v", loaded.envelope.Source)
				}
				replay, err := describePinnedDependency(context.Background(), fixture.app, pin)
				if err != nil || replay.Package.SourceKey != pin.SourceKey {
					t.Fatalf("replay provenance changed: %+v %v", replay, err)
				}
				state, err := fixture.store.Load()
				if err != nil {
					t.Fatal(err)
				}
				if state.Installations[0].Source.SourceURL != endpoint {
					t.Fatal("installation adopted another endpoint's provenance")
				}
				directColdRestoreToStore(t, fixture, transport, pin, coldStore)
			}
		})
	}
}

func directColdRestoreToStore(t *testing.T, original cliFixture, transport *directTransportFixture, pin environment.LockedPackage, store string) {
	t.Helper()
	fresh := directCLIEnvironment(t, transport)
	fresh.app.EnvironmentStoreRoot = store
	oldManifest, oldLock := original.app.dependencyPaths()
	newManifest, newLock := fresh.app.dependencyPaths()
	dependencyWrite(t, newManifest, dependencyRead(t, oldManifest))
	dependencyWrite(t, newLock, dependencyRead(t, oldLock))
	dependencyRun(t, fresh, "deps", "sync", "--locked", "--yes", "--format", "json")
	loaded, err := materializeLockedPackage(context.Background(), fresh.app, pin, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.envelope.Source != identityFromLocked(pin) {
		t.Fatal("cold restore reused another endpoint's provenance")
	}
}

func TestDirectSavedPreviewDoesNotPublishStoreOrIntent(t *testing.T) {
	for _, mode := range []string{"dry-run", "decline"} {
		t.Run(mode, func(t *testing.T) {
			transport := newDirectTransportFixture(t, "zip", false)
			fixture := directCLIEnvironment(t, transport)
			dependencyRun(t, fixture, "deps", "init", "--format", "json")
			manifestPath, lockPath := fixture.app.dependencyPaths()
			before := dependencyRead(t, manifestPath)
			args := []string{"add", transport.server.URL + "/package.zip"}
			if mode == "dry-run" {
				args = append(args, "--target", "gemini", "--dry-run", "--format", "json")
			}
			_, _, err := fixture.executeInput(mode == "decline", "n\n", args...)
			if err != nil {
				t.Fatal(err)
			}
			if dependencyRead(t, manifestPath) != before {
				t.Fatal("unconfirmed preview changed intent")
			}
			for _, path := range []string{lockPath, fixture.app.EnvironmentStoreRoot} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("unconfirmed preview published %s: %v", filepath.Base(path), err)
				}
			}
			state, err := fixture.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if len(state.Installations) != 0 || len(state.EnvironmentReceipts) != 0 {
				t.Fatal("unconfirmed preview changed ownership")
			}
		})
	}
}

func TestLegacyGitHubLockIdentitySurvivesNativeTransport(t *testing.T) {
	repo := dependencyRepo(t)
	fixture, source, _ := dependencySpecHarness(t, repo, dependencyClientSpec{id: domain.ClientGemini, activation: "active"})
	commit := savedJourneyGit(t, repo, "rev-parse", "HEAD")
	raw := githubIdentity("acme/demo", commit, "")
	dependencyRun(t, fixture, "add", raw, "--target", "gemini", "--format", "json")
	pin := directLockedPin(t, fixture)
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	before := state.Installations[0]
	expected := legacyGitIdentity("acme/demo", commit, "", "uap-dependency-git")
	if before.Source.SourceBindingID != domain.ComputeSourceBindingID(expected) || before.Source.SourceKind != "" || before.Source.SourceURL != "" {
		t.Fatalf("legacy identity changed: %+v", before.Source)
	}
	native, err := source.AcquireGit(context.Background(), gitSourceSpec(*pin.Requirement().Git), commit, pin.TreeDigest)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = packagedigest.Remove(native) }()
	// Store metadata can originate from the generic transport. The lock still
	// supplies the historical identity on every replay and acquired-cache path.
	fixture.app.EnvironmentStoreRoot = filepath.Join(t.TempDir(), "different-first-writer-store")
	if _, err := fixture.app.dependencyStore().Put(context.Background(), native); err != nil {
		t.Fatal(err)
	}
	dependencyRun(t, fixture, "repair", before.InstallationID, "--target", "gemini", "--format", "json")
	dependencyRun(t, fixture, "deps", "sync", "--locked", "--yes", "--format", "json")
	after, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if after.Installations[0].InstallationID != before.InstallationID || after.Installations[0].Source != before.Source {
		t.Fatalf("legacy binding changed: %+v", after.Installations[0].Source)
	}
}
