//go:build uappm_e2e

package agentpluginscli

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func packageManagerRegistryCases() map[string]func(*testing.T) {
	return map[string]func(*testing.T){
		"compatible-ranges":           packageManagerCompatibleRanges,
		"incompatible-ranges":         packageManagerIncompatibleRanges,
		"forced-backtracking":         packageManagerForcedBacktracking,
		"candidate-integrity-failure": packageManagerCandidateIntegrityFailure,
		"candidate-ineligibility":     TestDependencySelectionSkipsHostAndLoadedMCPIneligibility,
		"partial-commit-locked-retry": TestDependencyEnvironmentCLIEndToEnd,
		"mixed-source-target-union":   TestDependencyTOMLMixedSourceLifecycleAndRestore,
		"portable-cold-restore":       TestDependencyFreshRestoreFromPortableFiles,
	}
}

// The verified Directory and Codex responses are fixtures. Its package releases
// are independently authored Git commits and pass the real immutable acquirer,
// standard loader, graph solver, store and installer transaction paths.
func packageManagerRegistryRelease(t *testing.T, fixture freshRestoreEnvironment, repo, alias, version string, sequence uint64, dependencies map[string]string) domain.DirectoryRelease {
	t.Helper()
	release := freshRestorePackage(t, fixture, repo, alias, version, alias+"-"+version, "", sequence, true)
	if len(dependencies) == 0 {
		return release
	}
	return dependencyReviewReviseRelease(t, fixture, repo, release, func(root string) {
		keys := make([]string, 0, len(dependencies))
		for key := range dependencies {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var body strings.Builder
		body.WriteString("schema_version = 1\n[dependencies]\n")
		for _, key := range keys {
			fmt.Fprintf(&body, "%q = %q\n", "acme/"+key, dependencies[key])
		}
		dependencyWrite(t, filepath.Join(root, "uap.toml"), body.String())
	})
}

func packageManagerRegistryIntent(t *testing.T, fixture freshRestoreEnvironment, a, b string) string {
	t.Helper()
	manifest := fmt.Sprintf("schema_version = 1\n[dependencies]\na = %q\nb = %q\n[targets]\ncodex = [\"a\", \"b\"]\n", a, b)
	mp, _ := fixture.app.dependencyPaths()
	dependencyWrite(t, mp, manifest)
	return manifest
}

func packageManagerCompatibleRanges(t *testing.T) {
	fixture, repo, _ := dependencyReviewFixture(t)
	a := packageManagerRegistryRelease(t, fixture, repo, "a", "1.0.0", 1, map[string]string{"c": "^1.0"})
	b := packageManagerRegistryRelease(t, fixture, repo, "b", "1.0.0", 1, map[string]string{"c": "^1.1"})
	packageManagerRegistryRelease(t, fixture, repo, "c", "1.0.0", 1, nil)
	c := packageManagerRegistryRelease(t, fixture, repo, "c", "1.1.0", 2, nil)
	packageManagerRegistryRelease(t, fixture, repo, "c", "2.0.0", 3, nil)
	packageManagerRegistryIntent(t, fixture, "1.0.0", "1.0.0")
	dependencyRun(t, fixture.cliFixture, "install", "--yes", "--format", "json")
	packageManagerAssertRegistryGraph(t, fixture, map[string]domain.DirectoryRelease{"a": a, "b": b, "c": c}, []environment.RequiredBy{
		{Alias: "a", Selector: environment.Selector{Kind: environment.SelectorRange, Value: "^1.0"}},
		{Alias: "b", Selector: environment.Selector{Kind: environment.SelectorRange, Value: "^1.1"}},
	})
}

func packageManagerIncompatibleRanges(t *testing.T) {
	fixture, repo, _ := dependencyReviewFixture(t)
	packageManagerRegistryRelease(t, fixture, repo, "a", "1.0.0", 1, map[string]string{"c": "^1.0"})
	packageManagerRegistryRelease(t, fixture, repo, "b", "1.0.0", 1, map[string]string{"c": "^2.0"})
	packageManagerRegistryRelease(t, fixture, repo, "c", "1.0.0", 1, nil)
	packageManagerRegistryRelease(t, fixture, repo, "c", "2.0.0", 2, nil)
	manifest := packageManagerRegistryIntent(t, fixture, "1.0.0", "1.0.0")
	_, _, err := fixture.execute(false, "install", "--yes", "--format", "json")
	if err == nil || !strings.Contains(err.Error(), "dependency conflict for directory:acme/c") {
		t.Fatalf("incompatible shared ranges were not refused as a graph conflict: %v", err)
	}
	packageManagerAssertRegistryRefusal(t, fixture, manifest)
}

func packageManagerForcedBacktracking(t *testing.T) {
	fixture, repo, _ := dependencyReviewFixture(t)
	firstA := packageManagerRegistryRelease(t, fixture, repo, "a", "1.0.0", 1, map[string]string{"c": "^1.0", "d": "^1.0"})
	a := packageManagerRegistryRelease(t, fixture, repo, "a", "1.1.0", 2, map[string]string{"c": "^2.0"})
	b := packageManagerRegistryRelease(t, fixture, repo, "b", "1.0.0", 1, map[string]string{"c": "^2.0"})
	packageManagerRegistryRelease(t, fixture, repo, "c", "1.0.0", 1, nil)
	c := packageManagerRegistryRelease(t, fixture, repo, "c", "2.0.0", 2, nil)
	packageManagerRegistryRelease(t, fixture, repo, "d", "1.0.0", 1, nil)
	packageManagerRegistryIntent(t, fixture, "^1.0", "1.0.0")
	dependencyRun(t, fixture.cliFixture, "install", "--yes", "--format", "json")
	// Prove that this is backtracking, rather than a fixture that only makes the
	// final candidate available. Both independently authored A releases were read.
	seen := map[string]bool{}
	for _, fetch := range fixture.source.verified {
		seen[fetch.commit+"/"+fetch.path] = true
	}
	for _, release := range []domain.DirectoryRelease{firstA, a} {
		if !seen[release.PackageSource.Revision+"/"+release.PackageSource.Path] {
			t.Fatalf("backtracking did not acquire A %s: %+v", release.PackageVersion, fixture.source.verified)
		}
	}
	packageManagerAssertRegistryGraph(t, fixture, map[string]domain.DirectoryRelease{"a": a, "b": b, "c": c}, []environment.RequiredBy{
		{Alias: "a", Selector: environment.Selector{Kind: environment.SelectorRange, Value: "^2.0"}},
		{Alias: "b", Selector: environment.Selector{Kind: environment.SelectorRange, Value: "^2.0"}},
	})
}

func packageManagerCandidateIntegrityFailure(t *testing.T) {
	fixture, repo, directory := dependencyReviewFixture(t)
	first := packageManagerRegistryRelease(t, fixture, repo, "a", "1.0.0", 1, nil)
	packageManagerRegistryRelease(t, fixture, repo, "a", "1.1.0", 2, nil)
	// Metadata admits the first release, but its authenticated package identity
	// disagrees with the real bytes. Integrity errors must abort candidate search.
	wrong := "sha256:" + strings.Repeat("0", 64)
	directory.bundle.Snapshot.Distributions[0].Releases[0].TreeDigest = wrong
	directory.bundle.Snapshot.Evidence[0].PackageTreeDigest = wrong
	mp, _ := fixture.app.dependencyPaths()
	manifest := "schema_version = 1\n[dependencies]\na = \"^1.0\"\n[targets]\ncodex = [\"a\"]\n"
	dependencyWrite(t, mp, manifest)
	_, _, err := fixture.execute(false, "deps", "lock", "--format", "json")
	if err == nil || !strings.Contains(err.Error(), "acquired package tree digest") || !strings.Contains(err.Error(), "does not match expected") {
		t.Fatalf("candidate integrity failure was masked or did not reach byte verification: %v", err)
	}
	if len(fixture.source.verified) != 1 || fixture.source.verified[0].path != first.PackageSource.Path || fixture.source.verified[0].digest != wrong {
		t.Fatalf("integrity failure fell back to another candidate: %+v", fixture.source.verified)
	}
	packageManagerAssertRegistryRefusal(t, fixture, manifest)
}

func packageManagerAssertRegistryRefusal(t *testing.T, fixture freshRestoreEnvironment, manifest string) {
	t.Helper()
	mp, lp := fixture.app.dependencyPaths()
	if dependencyRead(t, mp) != manifest || fixture.runner.calls != 0 {
		t.Fatal("resolution refusal changed intent or launched a native client")
	}
	for _, path := range []string{lp, fixture.store.Path, fixture.app.ManagedRoot, fixture.client.ConfigRoot} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("resolution refusal published installed effects %s: %v", path, err)
		}
	}
}

func packageManagerAssertRegistryGraph(t *testing.T, fixture freshRestoreEnvironment, expected map[string]domain.DirectoryRelease, parents []environment.RequiredBy) {
	t.Helper()
	locked := freshRestoreReadLock(t, fixture.cliFixture)
	if len(locked.Packages) != len(expected) {
		t.Fatalf("selected graph contains omitted or rejected branch nodes: %+v", locked.Packages)
	}
	for _, pin := range locked.Packages {
		release, exists := expected[pin.Alias]
		if !exists || pin.Version != release.PackageVersion || pin.TreeDigest != release.TreeDigest || pin.ManifestDigest != release.ManifestDigest || lockedCommit(pin) != release.PackageSource.Revision || !reflect.DeepEqual(pin.Targets, []string{"codex"}) {
			t.Fatalf("selected pin differs from independently authored release: %+v want %+v", pin, release)
		}
		if pin.Source.Registry == nil || pin.Source.Registry.DistributionID != "acme/"+pin.Alias || pin.Source.Registry.ReleaseSequence != release.Sequence {
			t.Fatalf("selected pin lost its registry publication identity: %+v", pin)
		}
		if pin.Alias == "c" {
			if !reflect.DeepEqual(pin.RequiredBy, parents) {
				t.Fatalf("shared dependency lost selected parent constraints: %+v", pin.RequiredBy)
			}
		} else if len(pin.RequiredBy) != 0 {
			t.Fatalf("selected direct root retained an unexpected edge: %+v", pin)
		}
	}
	// This helper independently checks receipts, loader identity and immutable
	// source/store digests for precisely the three expected installed nodes.
	freshRestoreCheckInstalled(t, fixture, locked)
	view, err := fixture.app.Installer.Inspect(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, installation := range view.Installations {
		binding := installation.Bindings[0]
		release, exists := expected[binding.Owner.Alias]
		if !exists || !strings.Contains(dependencyRead(t, filepath.Join(binding.TargetPath, "skills", binding.Owner.Alias, "SKILL.md")), "Recorded package "+binding.Owner.Alias+" "+release.PackageVersion+".") {
			t.Fatalf("installed payload came from an omitted or rejected candidate: %+v", binding)
		}
	}
}
