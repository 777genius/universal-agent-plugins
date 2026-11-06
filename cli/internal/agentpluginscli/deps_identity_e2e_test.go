package agentpluginscli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// These fixtures use real Git modes, acquisition, loading, immutable storage and
// installer transactions. Native Codex responses and the process runner for the
// disposable local repository are fixtures; containment has separate tests.
func newDependencyIdentityEnvironment(t *testing.T, repo string) freshRestoreEnvironment {
	t.Helper()
	fixture := newFreshRestoreEnvironment(t, repo, nil)
	fixture.source.Runner = directGitTestRunner{}
	return fixture
}

func dependencyIdentityAgreement(t *testing.T) (repo, manifest, lock string) {
	t.Helper()
	repo = writeCLIPlugin(t)
	writeCLIMCP(t, repo)
	if err := os.Mkdir(filepath.Join(repo, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alpha", "omega"} {
		dependencyWrite(t, filepath.Join(repo, "bin", name), "#!/bin/sh\n# immutable executable fixture; never run\n")
	}
	dependencyGit(t, repo, "init", "-b", "main")
	dependencyGit(t, repo, "add", ".")
	// Explicit index modes work on Windows as well as POSIX hosts.
	dependencyGit(t, repo, "update-index", "--chmod=+x", "bin/alpha", "bin/omega")
	dependencyGit(t, repo, "commit", "-m", "test: exact executable inventory")
	command := exec.CommandContext(t.Context(), "git", "rev-parse", "HEAD")
	command.Dir = repo
	commit, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	original := newDependencyIdentityEnvironment(t, repo)
	mp, lp := original.app.dependencyPaths()
	manifest = fmt.Sprintf("schema_version = 1\n[dependencies.demo]\ngit = \"acme/identity\"\ncommit = %q\n[targets]\ncodex = [\"demo\"]\n", strings.TrimSpace(string(commit)))
	dependencyWrite(t, mp, manifest)
	dependencyRun(t, original.cliFixture, "deps", "lock", "--format", "json")
	pin := freshRestoreReadLock(t, original.cliFixture).Packages[0]
	if !reflect.DeepEqual(pin.ExecutableFiles, []string{"bin/alpha", "bin/omega"}) {
		t.Fatalf("fixture lost immutable executable modes: %v", pin.ExecutableFiles)
	}
	return repo, manifest, dependencyRead(t, lp)
}

func dependencyIdentityEditedLock(t *testing.T, original string, edit func(*environment.LockedPackage)) string {
	t.Helper()
	var lock environment.Lock
	if err := json.Unmarshal([]byte(original), &lock); err != nil {
		t.Fatal(err)
	}
	edit(&lock.Packages[0])
	body, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(body) + "\n"
}

func TestDependencyPinInventoryTamperingWarmAndColdCLI(t *testing.T) {
	repo, manifest, original := dependencyIdentityAgreement(t)
	for _, tc := range []struct {
		name string
		edit func(*environment.LockedPackage)
	}{
		{"missing", func(pin *environment.LockedPackage) { pin.ExecutableFiles = nil }},
		{"extra", func(pin *environment.LockedPackage) { pin.ExecutableFiles = append(pin.ExecutableFiles, "bin/ghost") }},
		{"duplicate", func(pin *environment.LockedPackage) { pin.ExecutableFiles = append(pin.ExecutableFiles, "bin/alpha") }},
		{"escaping", func(pin *environment.LockedPackage) { pin.ExecutableFiles = append(pin.ExecutableFiles, "../outside") }},
		{"version", func(pin *environment.LockedPackage) { pin.Version = "9.9.9" }},
	} {
		for _, warm := range []bool{true, false} {
			for _, operation := range []string{"lock", "install"} {
				name := fmt.Sprintf("%s/warm=%t/%s", tc.name, warm, operation)
				t.Run(name, func(t *testing.T) {
					fixture := newDependencyIdentityEnvironment(t, repo)
					freshRestoreCopyAgreement(t, fixture, manifest, original)
					if warm {
						dependencyRun(t, fixture.cliFixture, "install", "--locked", "--yes", "--format", "json")
					}
					mp, lp := fixture.app.dependencyPaths()
					dependencyWrite(t, lp, dependencyIdentityEditedLock(t, original, tc.edit))
					before := freshRestoreFiles(t, mp, lp, fixture.store.Path, fixture.app.EnvironmentStoreRoot, fixture.app.ManagedRoot, fixture.client.ConfigRoot)
					lookups, mutations := fixture.source.lookups, fixture.runner.mutations
					args := []string{"deps", "lock", "--format", "json"}
					if operation == "install" {
						args = []string{"install", "--locked", "--yes", "--format", "json"}
					}
					if _, _, err := fixture.execute(false, args...); err == nil {
						t.Fatalf("%s accepted inconsistent immutable lock metadata", operation)
					}
					if fixture.source.lookups != lookups || fixture.runner.mutations != mutations || !reflect.DeepEqual(before, freshRestoreFiles(t, mp, lp, fixture.store.Path, fixture.app.EnvironmentStoreRoot, fixture.app.ManagedRoot, fixture.client.ConfigRoot)) {
						t.Fatal("immutable pin refusal changed agreement/store/installation or resolved a moving ref")
					}
				})
			}
		}
	}
}

func TestDependencyPinUnsortedInventoryPreservesAgreementCLI(t *testing.T) {
	repo, manifest, original := dependencyIdentityAgreement(t)
	unsorted := dependencyIdentityEditedLock(t, original, func(pin *environment.LockedPackage) {
		pin.ExecutableFiles = []string{"bin/omega", "bin/alpha"}
	})
	for _, warm := range []bool{false, true} {
		t.Run(fmt.Sprintf("warm=%t", warm), func(t *testing.T) {
			fixture := newDependencyIdentityEnvironment(t, repo)
			freshRestoreCopyAgreement(t, fixture, manifest, original)
			if warm {
				dependencyRun(t, fixture.cliFixture, "install", "--locked", "--yes", "--format", "json")
			}
			mp, lp := fixture.app.dependencyPaths()
			dependencyWrite(t, lp, unsorted)
			agreement := freshRestoreFiles(t, mp, lp)
			dependencyRun(t, fixture.cliFixture, "install", "--locked", "--yes", "--format", "json")
			dependencyRun(t, fixture.cliFixture, "deps", "lock", "--format", "json")
			before := freshRestoreFiles(t, mp, lp, fixture.store.Path, fixture.app.EnvironmentStoreRoot, fixture.app.ManagedRoot, fixture.client.ConfigRoot)
			downloads, mutations := fixture.source.downloads, fixture.runner.mutations
			dependencyRun(t, fixture.cliFixture, "install", "--locked", "--yes", "--format", "json")
			dependencyRun(t, fixture.cliFixture, "deps", "lock", "--format", "json")
			if !reflect.DeepEqual(agreement, freshRestoreFiles(t, mp, lp)) || !reflect.DeepEqual(before, freshRestoreFiles(t, mp, lp, fixture.store.Path, fixture.app.EnvironmentStoreRoot, fixture.app.ManagedRoot, fixture.client.ConfigRoot)) || fixture.source.downloads != downloads || fixture.runner.mutations != mutations || fixture.source.lookups != 0 {
				t.Fatal("historical inventory order caused lock churn, acquisition or repeated deployment")
			}
		})
	}
}

func TestDependencyPinLegacyVersionHydrationWarmAndColdCLI(t *testing.T) {
	repo, manifest, original := dependencyIdentityAgreement(t)
	legacy := dependencyIdentityEditedLock(t, original, func(pin *environment.LockedPackage) { pin.Version = "" })
	for _, warm := range []bool{false, true} {
		t.Run(fmt.Sprintf("warm=%t", warm), func(t *testing.T) {
			fixture := newDependencyIdentityEnvironment(t, repo)
			freshRestoreCopyAgreement(t, fixture, manifest, original)
			if warm {
				dependencyRun(t, fixture.cliFixture, "install", "--locked", "--yes", "--format", "json")
			}
			_, lp := fixture.app.dependencyPaths()
			dependencyWrite(t, lp, legacy)
			dependencyRun(t, fixture.cliFixture, "deps", "lock", "--format", "json")
			lock := freshRestoreReadLock(t, fixture.cliFixture)
			if lock.Packages[0].Version != "1.0.0" || !reflect.DeepEqual(lock.Packages[0].ExecutableFiles, []string{"bin/alpha", "bin/omega"}) || fixture.source.lookups != 0 {
				t.Fatalf("legacy hydration changed immutable identity: %+v", lock.Packages[0])
			}
			before := freshRestoreFiles(t, lp)
			dependencyRun(t, fixture.cliFixture, "deps", "lock", "--format", "json")
			if !reflect.DeepEqual(before, freshRestoreFiles(t, lp)) {
				t.Fatal("hydrated lock changed on repeated resolution")
			}
		})
	}
}

func TestDependencyRecordedRepairHydratesInventoryWarmAndColdCLI(t *testing.T) {
	repo, manifest, original := dependencyIdentityAgreement(t)
	for _, warm := range []bool{true, false} {
		t.Run(fmt.Sprintf("warm=%t", warm), func(t *testing.T) {
			fixture := newDependencyIdentityEnvironment(t, repo)
			freshRestoreCopyAgreement(t, fixture, manifest, original)
			dependencyRun(t, fixture.cliFixture, "install", "--locked", "--yes", "--format", "json")
			state, err := fixture.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			installation := state.Installations[0]
			binding, err := selectedRepairBinding(installation, domain.ClientCodex, fixture.app.scope())
			if err != nil {
				t.Fatal(err)
			}
			if binding.PackageRevision == nil || binding.PackageRevision.Version != "1.0.0" {
				t.Fatalf("fixture has no exact recorded repair revision: %+v", binding.PackageRevision)
			}
			drift := filepath.Join(binding.TargetLocator, "identity-repair-drift.txt")
			dependencyWrite(t, drift, "explicit repair must restore the exact recorded package")
			if !warm {
				if err := os.RemoveAll(fixture.app.EnvironmentStoreRoot); err != nil {
					t.Fatal(err)
				}
			}
			mp, lp := fixture.app.dependencyPaths()
			agreement := freshRestoreFiles(t, mp, lp)
			downloads := fixture.source.downloads
			dependencyRun(t, fixture.cliFixture, "repair", installation.InstallationID, "--target", "codex", "--format", "json")
			if _, err := os.Stat(drift); !os.IsNotExist(err) {
				t.Fatalf("explicit recorded repair did not remove injected drift: %v", err)
			}
			after, err := fixture.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			repairedBinding, err := selectedRepairBinding(after.Installations[0], domain.ClientCodex, fixture.app.scope())
			if err != nil {
				t.Fatal(err)
			}
			repaired := repairedBinding.PackageRevision
			if repaired == nil || repaired.TreeDigest != binding.PackageRevision.TreeDigest || repaired.ResolvedRevision != binding.PackageRevision.ResolvedRevision || !reflect.DeepEqual(agreement, freshRestoreFiles(t, mp, lp)) || fixture.source.lookups != 0 {
				t.Fatal("receipt-only repair changed pin or desired agreement")
			}
			wantDownloads := 0
			if !warm {
				wantDownloads = 1
			}
			if fixture.source.downloads-downloads != wantDownloads {
				t.Fatalf("repair acquisitions=%d want=%d", fixture.source.downloads-downloads, wantDownloads)
			}
		})
	}
}
