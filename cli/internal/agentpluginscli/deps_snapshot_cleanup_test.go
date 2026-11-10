package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

type dependencyCleanupAcquirer struct {
	dependencyCheckForbiddenEffects
	snapshot domain.PackageSnapshot
	calls    int
}

func (acquirer *dependencyCleanupAcquirer) AcquireGitHubVerified(context.Context, string, string, string, string) (domain.PackageSnapshot, error) {
	acquirer.calls++
	return acquirer.snapshot, nil
}

// Keep Builder-verified bytes intact while making the existing cleanup ownership
// guard refuse the snapshot. No privilege-dependent permission failure is used.
func refuseDependencySnapshotCleanup(t *testing.T, snapshot domain.PackageSnapshot) domain.PackageSnapshot {
	t.Helper()
	relocated := filepath.Join(t.TempDir(), "unrecognized-parent")
	if err := os.Rename(filepath.Dir(snapshot.Root), relocated); err != nil {
		t.Fatal(err)
	}
	snapshot.Root = filepath.Join(relocated, "root")
	t.Cleanup(func() {
		if err := os.Chmod(snapshot.Root, 0o700); err != nil {
			t.Error(err)
		}
	})
	if err := packagedigest.Remove(snapshot); err == nil {
		t.Fatal("snapshot cleanup refusal was not established")
	}
	return snapshot
}

func coldDependencyCleanupFixture(t *testing.T, refuse bool) (cliFixture, environment.LockedPackage, *dependencyCleanupAcquirer) {
	t.Helper()
	f := newDependencyCheckFixture(t)
	pin := seedDependencyCheckPackage(t, f, "demo", "")
	snapshot, err := f.app.dependencyStore().Snapshot(t.Context(), pin.DigestAlgorithm, pin.TreeDigest)
	if err != nil {
		t.Fatal(err)
	}
	if refuse {
		snapshot = refuseDependencySnapshotCleanup(t, snapshot)
	} else {
		t.Cleanup(func() { _ = packagedigest.Remove(snapshot) })
	}
	f.app.EnvironmentStoreRoot = filepath.Join(f.root, "cold-store")
	f.app.SecurityEvaluator = nil
	acquirer := &dependencyCleanupAcquirer{snapshot: snapshot}
	f.app.SourceAcquirer = acquirer
	return f, pin, acquirer
}

func dependencyCleanupRegistryPin(pin environment.LockedPackage) environment.LockedPackage {
	git := pin.Source.Git
	pin.SourceKey = "directory:acme/demo"
	pin.Source = environment.LockedSource{Registry: &environment.LockedRegistrySource{
		Distribution: "acme/demo", DistributionID: "acme/demo", ProductID: "demo", ReleaseSequence: 1,
		Requested: environment.Selector{Kind: environment.SelectorRelease, Value: "1"}, Repository: git.Repository, Path: git.Path, Commit: git.Commit,
	}}
	return pin
}

func TestDependencyColdReplayCleanup(t *testing.T) {
	for _, mode := range []string{"success", "cleanup_refusal", "pin_mismatch", "loader_failure"} {
		t.Run(mode, func(t *testing.T) {
			f, pin, acquirer := coldDependencyCleanupFixture(t, mode != "success")
			var original error
			switch mode {
			case "pin_mismatch":
				pin.Version = "9.0.0"
				original = errDependencyPinMismatch
			case "loader_failure":
				original = errors.New("loader unavailable")
				f.app.PackageLoader = &recordingPackageLoader{err: original}
			}
			before := observeDependencyCheckFiles(t, f.root)
			resolved, err := readReplayDependency(t.Context(), f.app, pin)
			if acquirer.calls != 1 {
				t.Fatalf("cold acquisition calls=%d", acquirer.calls)
			}
			if mode == "success" {
				if err != nil || resolved.Package.Alias != pin.Alias {
					t.Fatalf("cold replay: %+v %v", resolved, err)
				}
				if _, err := os.Stat(acquirer.snapshot.Root); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("successful cold replay retained its snapshot: %v", err)
				}
			} else {
				if !errors.Is(err, errDependencySnapshotCleanup) || original != nil && !errors.Is(err, original) {
					t.Fatalf("cleanup lost original or cleanup identity: %v", err)
				}
				if !reflect.DeepEqual(resolved, environment.ResolvedPackage{}) {
					t.Fatalf("cleanup failure returned a usable package: %+v", resolved)
				}
			}
			assertDependencyCheckFilesUnchanged(t, f.root, before)
		})
	}
}

func TestDependencyStoredSnapshotFailurePreservesCleanup(t *testing.T) {
	for _, mode := range []string{"pin_mismatch", "loader_failure", "directory_refusal"} {
		t.Run(mode, func(t *testing.T) {
			f, pin, acquirer := coldDependencyCleanupFixture(t, true)
			original := errDependencyPinMismatch
			switch mode {
			case "pin_mismatch":
				pin.Version = "9.0.0"
			case "loader_failure":
				original = errors.New("stored loader unavailable")
				f.app.PackageLoader = &recordingPackageLoader{err: original}
			case "directory_refusal":
				original = domain.ErrDirectoryIneligible
				pin = dependencyCleanupRegistryPin(pin)
				f.app.DirectoryClient = &fixedDirectoryClient{err: original}
			}
			before := observeDependencyCheckFiles(t, f.root)
			loaded, err := loadStoredDependencySnapshot(t.Context(), f.app, acquirer.snapshot, pin, nil)
			if !errors.Is(err, original) || !errors.Is(err, errDependencySnapshotCleanup) {
				t.Fatalf("stored failure lost original or cleanup identity: %v", err)
			}
			if loaded.envelope.SnapshotRoot != "" || loaded.cleanup != nil || acquirer.calls != 0 {
				t.Fatalf("failed stored snapshot remained usable or acquired a source: %+v calls=%d", loaded, acquirer.calls)
			}
			assertDependencyCheckFilesUnchanged(t, f.root, before)
		})
	}
}

func TestDependencyEligibilityPreservesOperationalPolicyFailure(t *testing.T) {
	for _, mode := range []string{"pure_policy", "cleanup", "canceled", "deadline"} {
		t.Run(mode, func(t *testing.T) {
			f, pin, acquirer := coldDependencyCleanupFixture(t, mode == "cleanup")
			pin = dependencyCleanupRegistryPin(pin)
			original := domain.ErrDirectoryIneligible
			var terminal error
			switch mode {
			case "cleanup":
				terminal = errDependencySnapshotCleanup
			case "canceled":
				terminal = context.Canceled
			case "deadline":
				terminal = context.DeadlineExceeded
			}
			if terminal != nil && mode != "cleanup" {
				original = errors.Join(original, terminal)
			}
			f.app.DirectoryClient = &fixedDirectoryClient{err: original}
			before := observeDependencyCheckFiles(t, f.root)
			eligible, err := (dependencyResolver{app: f.app}).Eligible(t.Context(), pin, []string{"codex"})
			if eligible || acquirer.calls != 1 {
				t.Fatalf("policy failure acquired=%d eligible=%t", acquirer.calls, eligible)
			}
			if terminal == nil {
				if err != nil {
					t.Fatalf("pure policy refusal became terminal: %v", err)
				}
			} else if !errors.Is(err, terminal) || !errors.Is(err, domain.ErrDirectoryIneligible) {
				t.Fatalf("operational failure became candidate ineligibility: %v", err)
			}
			assertDependencyCheckFilesUnchanged(t, f.root, before)
		})
	}
}
