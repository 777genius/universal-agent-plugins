package agentpluginscli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Only source transport is injected. The CLI, strict loader, staged snapshots
// and immutable store remain real; no native Git or agent process is needed.
type dependencyLockRaceSource struct {
	SourceAcquirer
	root           string
	beforeSnapshot func() error
}

func (source *dependencyLockRaceSource) AcquireGitHub(ctx context.Context, _, _, _ string) (domain.PackageSnapshot, error) {
	if source.beforeSnapshot != nil {
		if err := source.beforeSnapshot(); err != nil {
			return domain.PackageSnapshot{}, err
		}
	}
	return (packagedigest.Builder{}).Snapshot(ctx, source.root, domain.SourceIdentity{})
}

func (source *dependencyLockRaceSource) AcquireGitHubVerified(ctx context.Context, repo, revision, subpath, digest string) (domain.PackageSnapshot, error) {
	snapshot, err := source.AcquireGitHub(ctx, repo, revision, subpath)
	if err == nil && snapshot.TreeDigest != digest {
		_ = packagedigest.Remove(snapshot)
		return domain.PackageSnapshot{}, errors.New("fixture immutable digest mismatch")
	}
	return snapshot, err
}

func dependencyLockRaceFixture(t *testing.T) (cliFixture, *dependencyLockRaceSource) {
	t.Helper()
	root := t.TempDir()
	aliasGraphPackage(t, root, "demo")
	fixture, _ := newDependencyHarness(t, "", []domain.DetectedClient{fixtureClient(t, domain.ClientGemini)}, installer.Config{})
	source := &dependencyLockRaceSource{root: root}
	fixture.app.SourceAcquirer = source
	manifest, _ := fixture.app.dependencyPaths()
	dependencyWrite(t, manifest, "schema_version = 1\n[dependencies.demo]\ngit = \"acme/demo\"\ncommit = \""+strings.Repeat("1", 40)+"\"\n[targets]\ngemini = [\"demo\"]\n")
	return fixture, source
}

func TestDependencyLockPreservesExternalEdits(t *testing.T) {
	for _, scenario := range []string{"created lock", "created empty lock", "changed lock", "unchanged cold lock", "changed manifest"} {
		t.Run(scenario, func(t *testing.T) {
			fixture, source := dependencyLockRaceFixture(t)
			manifestPath, lockPath := fixture.app.dependencyPaths()
			if scenario == "changed lock" || scenario == "unchanged cold lock" {
				dependencyRun(t, fixture, "deps", "lock", "--format", "json")
				if err := os.RemoveAll(fixture.app.EnvironmentStoreRoot); err != nil {
					t.Fatal(err)
				}
				if scenario == "changed lock" {
					dependencyWrite(t, manifestPath, strings.ReplaceAll(dependencyRead(t, manifestPath), strings.Repeat("1", 40), strings.Repeat("2", 40)))
				}
			}
			externalPath, externalBytes := lockPath, "external lock edit\n"
			if scenario == "created empty lock" {
				externalBytes = ""
			}
			if scenario == "changed manifest" {
				externalPath, externalBytes = manifestPath, dependencyRead(t, manifestPath)+"# external edit\n"
			}
			// Acquisition happens after the command has read its agreement. This
			// deterministic callback models an editor writing while resolution runs.
			source.beforeSnapshot = func() error { return os.WriteFile(externalPath, []byte(externalBytes), 0600) }
			_, _, err := fixture.execute(false, "deps", "lock", "--format", "json")
			if err == nil {
				t.Fatal("lock accepted an external agreement edit during resolution")
			}
			if got := dependencyRead(t, externalPath); got != externalBytes {
				t.Fatalf("external edit overwritten: got %q, want %q", got, externalBytes)
			}
			if _, err := os.Stat(fixture.app.EnvironmentStoreRoot); !os.IsNotExist(err) {
				t.Fatalf("agreement changed before publication, but store was published: %v", err)
			}
			if _, err := os.Stat(fixture.store.Path); !os.IsNotExist(err) {
				t.Fatalf("lock-only refusal wrote client state: %v", err)
			}
		})
	}
}

func TestDependencyLockInputCheckDistinguishesAbsentAndEmpty(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "uap.toml")
	lockPath := filepath.Join(dir, "uap.lock")
	manifestBytes := []byte("schema_version = 1\n")
	dependencyWrite(t, manifestPath, string(manifestBytes))
	if err := checkDependencyLockInputs(manifestPath, lockPath, manifestBytes, nil); err != nil {
		t.Fatalf("absent lock did not preserve an absent preimage: %v", err)
	}
	dependencyWrite(t, lockPath, "")
	if err := checkDependencyLockInputs(manifestPath, lockPath, manifestBytes, nil); !errors.Is(err, installer.ErrPlanChanged) {
		t.Fatalf("existing empty lock was confused with an absent preimage: %v", err)
	}
	if err := checkDependencyLockInputs(manifestPath, lockPath, manifestBytes, []byte{}); err != nil {
		t.Fatalf("existing empty lock did not match an empty preimage: %v", err)
	}
}

type dependencyLockEditScanner struct{ edit func() error }

func (scanner dependencyLockEditScanner) Evaluate(context.Context, domain.SecurityEvaluationInput) (domain.SecurityAssessment, error) {
	return domain.SecurityAssessment{}, scanner.edit()
}

func TestDependencyLockRechecksAfterStorePublication(t *testing.T) {
	for _, filename := range []string{"uap.toml", "uap.lock"} {
		t.Run(filename, func(t *testing.T) {
			fixture, _ := dependencyLockRaceFixture(t)
			manifestPath, lockPath := fixture.app.dependencyPaths()
			manifestBefore := []byte(dependencyRead(t, manifestPath))
			requirements, err := readDependencyManifestBytes(manifestPath, manifestBefore)
			if err != nil {
				t.Fatal(err)
			}
			app, cleanup, err := dependencyPreviewApp(fixture.app, &options{})
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			_, _, locked, err := resolveDependencyLock(context.Background(), app, &options{}, requirements, nil, "")
			if err != nil {
				t.Fatal(err)
			}
			externalPath := filepath.Join(filepath.Dir(manifestPath), filename)
			externalBytes := "external edit during store publication\n"
			scans := 0
			app.SecurityEvaluator = dependencyLockEditScanner{edit: func() error {
				scans++
				return os.WriteFile(externalPath, []byte(externalBytes), 0600)
			}}
			err = publishResolvedDependencyLock(context.Background(), app, manifestPath, lockPath, manifestBefore, nil, locked, true)
			if !errors.Is(err, installer.ErrPlanChanged) || scans == 0 {
				t.Fatalf("publication did not refuse a changed agreement after scanning: %v scans=%d", err, scans)
			}
			if got := dependencyRead(t, externalPath); got != externalBytes {
				t.Fatalf("publication overwrote external edit: %q", got)
			}
			pin := locked.Packages[0]
			if _, err := app.dependencyStore().Get(context.Background(), pin.DigestAlgorithm, pin.TreeDigest); err != nil {
				t.Fatalf("second check must retain the already verified immutable entry: %v", err)
			}
			if filename == "uap.toml" {
				if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
					t.Fatalf("changed manifest still published a lock: %v", err)
				}
			}
			if _, err := os.Stat(fixture.store.Path); !os.IsNotExist(err) {
				t.Fatalf("refused lock publication wrote client state: %v", err)
			}
		})
	}
}

func TestDependencyLockUnchangedHydratesMissingStore(t *testing.T) {
	fixture, _ := dependencyLockRaceFixture(t)
	dependencyRun(t, fixture, "deps", "lock", "--format", "json")
	_, lockPath := fixture.app.dependencyPaths()
	before := dependencyRead(t, lockPath)
	if err := os.RemoveAll(fixture.app.EnvironmentStoreRoot); err != nil {
		t.Fatal(err)
	}
	var result dependencyLockResult
	if err := json.Unmarshal([]byte(dependencyRun(t, fixture, "deps", "lock", "--format", "json")), &result); err != nil {
		t.Fatal(err)
	}
	if result.Changed || dependencyRead(t, lockPath) != before || len(result.Lock.Packages) != 1 {
		t.Fatal("unchanged lock was rewritten or reported changed during store hydration")
	}
	pin := result.Lock.Packages[0]
	if _, err := fixture.app.dependencyStore().Get(context.Background(), pin.DigestAlgorithm, pin.TreeDigest); err != nil {
		t.Fatalf("unchanged lock did not hydrate its missing immutable store entry: %v", err)
	}
}

func TestDependencyLockReadOnlyResolutionDoesNotPublish(t *testing.T) {
	for _, mode := range []string{"dry-run", "no-write"} {
		for _, edited := range []bool{false, true} {
			name := mode
			if edited {
				name += "/external-edit"
			}
			t.Run(name, func(t *testing.T) {
				fixture, source := dependencyLockRaceFixture(t)
				_, lockPath := fixture.app.dependencyPaths()
				if edited {
					source.beforeSnapshot = func() error { return os.WriteFile(lockPath, []byte("external edit"), 0600) }
				}
				result, err := runDependencyLock(context.Background(), fixture.app, &options{dryRun: mode == "dry-run"}, "", mode != "no-write")
				if edited {
					if !errors.Is(err, installer.ErrPlanChanged) || dependencyRead(t, lockPath) != "external edit" {
						t.Fatalf("read-only result hid or replaced an external edit: %v", err)
					}
				} else if err != nil || !result.Changed || len(result.Lock.Packages) != 1 {
					t.Fatalf("read-only resolution did not return its candidate: %+v %v", result, err)
				} else if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
					t.Fatalf("read-only resolution published the lock: %v", err)
				}
				if _, err := os.Stat(fixture.app.EnvironmentStoreRoot); !os.IsNotExist(err) {
					t.Fatalf("read-only resolution published the store: %v", err)
				}
			})
		}
	}
}

func TestDependencyLockRejectsEmptyExistingLock(t *testing.T) {
	fixture, source := dependencyLockRaceFixture(t)
	_, lockPath := fixture.app.dependencyPaths()
	dependencyWrite(t, lockPath, "")
	source.beforeSnapshot = func() error { return errors.New("unexpected acquisition after an empty existing lock") }
	_, _, err := fixture.execute(false, "deps", "lock", "--format", "json")
	if err == nil || !strings.Contains(err.Error(), "decode uap.lock") || dependencyRead(t, lockPath) != "" {
		t.Fatalf("empty existing lock was treated as absent: %v", err)
	}
}
