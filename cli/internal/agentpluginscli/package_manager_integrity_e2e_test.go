//go:build uappm_e2e

package agentpluginscli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func pmIntegrityEmbeddedManifestAdmission(t *testing.T) {
	t.Run("dependency-commands", TestDependencyPackageManifestOversizedCLIRefuses)
	t.Run("imperative-admission", TestDependencyPackageManifestImperativeRefuses)
}

func pmIntegrityOfflineScopes(t *testing.T) {
	t.Run("user", TestDependencyCheckOfflineCLI)
	t.Run("project", TestDependencyCheckProjectScopeCLI)
}

func pmIntegrityOfflineCancellation(t *testing.T) {
	t.Run("command-entry", TestDependencyCheckCancellationAndOperationalClassification)
	t.Run("verified-snapshot-cleanup", TestDependencyCheckCancellationCleansSnapshotCLI)
}

func pmIntegrityGraphFixture(t *testing.T) cliFixture {
	t.Helper()
	f, _ := newDependencyHarness(t, "", nil, installer.Config{})
	f.app.SourceAcquirer = dependencyCheckForbiddenEffects{}
	f.app.DirectoryClient = dependencyCheckForbiddenEffects{}
	f.app.Detector = dependencyCheckForbiddenEffects{}
	f.app.SecurityEvaluator = dependencyCheckForbiddenEffects{}
	return f
}

// These portable agreements contain exact, structurally valid pin coordinates.
// Tree inspects the agreement alone: absent payloads must not trigger fetching,
// client discovery, installation or security refresh. Recorded graphs may be
// larger than the budgets used for fresh dependency solving.
func pmIntegrityRecordedGraph(shape string, count int) ([]environment.LockedPackage, []string) {
	pins := make([]environment.LockedPackage, count)
	roots := []string{}
	selector := environment.Selector{Kind: environment.SelectorCommit, Value: strings.Repeat("a", 40)}
	for i := range count {
		alias := fmt.Sprintf("node%04d", i)
		pins[i] = environment.LockedPackage{
			Alias: alias, SourceKey: "github:acme/" + alias + ":", Version: "1.0.0", Targets: []string{"codex"},
			DigestAlgorithm: domain.TreeDigestAlgorithm, TreeDigest: "sha256:" + strings.Repeat("b", 64), ManifestDigest: "sha256:" + strings.Repeat("c", 64),
			Source: environment.LockedSource{Git: &environment.LockedGitSource{Repository: "acme/" + alias, Commit: selector.Value, Requested: selector}},
		}
		if shape != "chain" || i == 0 {
			roots = append(roots, alias)
		}
		if shape == "chain" && i > 0 {
			pins[i].RequiredBy = []environment.RequiredBy{{Alias: pins[i-1].Alias, Selector: selector}}
		}
	}
	if shape == "dense-overflow" {
		// All consumers are distinct and every node is a root. This is a valid
		// historical graph beyond the fresh solver's demand budget.
		edges := 0
		for i := range pins {
			for j := range pins {
				if i == j || edges == 8193 {
					continue
				}
				pins[i].RequiredBy = append(pins[i].RequiredBy, environment.RequiredBy{Alias: pins[j].Alias, Selector: selector})
				edges++
			}
		}
	}
	return pins, roots
}

func pmIntegrityLargeRecordedTree(t *testing.T) {
	for _, tc := range []struct {
		shape string
		count int
	}{{"independent", 1024}, {"chain", 256}, {"independent", 1025}, {"dense-overflow", 92}} {
		t.Run(fmt.Sprintf("%s-%d", tc.shape, tc.count), func(t *testing.T) {
			f := pmIntegrityGraphFixture(t)
			pins, roots := pmIntegrityRecordedGraph(tc.shape, tc.count)
			writeDependencyCheckAgreement(t, f, pins, roots...)
			before := observeDependencyCheckFiles(t, f.root)
			var tree dependencyTree
			body := dependencyRun(t, f, "deps", "tree", "--format", "json")
			if err := json.Unmarshal([]byte(body), &tree); err != nil || len(tree.Packages) != tc.count || len(tree.Roots) != len(roots) {
				t.Fatalf("large recorded graph was lost: roots=%d nodes=%d err=%v", len(tree.Roots), len(tree.Packages), err)
			}
			human := dependencyRun(t, f, "deps", "tree")
			for _, line := range strings.Split(human, "\n") {
				if len(line)-len(strings.TrimLeft(line, " ")) > 64 {
					t.Fatal("deep recorded graph grew unbounded display indentation")
				}
			}
			if tc.shape == "chain" && !strings.Contains(human, "[depth 255]") {
				t.Fatal("deep tree lost its explicit depth beyond the indentation ceiling")
			}
			assertDependencyCheckFilesUnchanged(t, f.root, before)
		})
	}
}

func pmIntegrityGraphLimitRefusal(t *testing.T) {
	f := pmIntegrityGraphFixture(t)
	pins, roots := pmIntegrityRecordedGraph("independent", 1025)
	writeDependencyCheckAgreement(t, f, pins, roots...)
	_, lock := f.app.dependencyPaths()
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	before := pmIntegrityPersistentFiles(t, f)
	out, stderr, err := f.execute(false, "deps", "lock", "--format", "json")
	if !errors.Is(err, environment.ErrResolutionLimit) {
		t.Fatalf("fresh oversized roots escaped solving admission: %v stdout=%s stderr=%s", err, out, stderr)
	}
	pmArchiveAssertEqual(t, "refused solving persistent files", before, pmIntegrityPersistentFiles(t, f))
}

func pmIntegrityRecordedGraphCompatibility(t *testing.T) {
	for _, tc := range []struct {
		shape string
		count int
	}{{"independent", 1025}, {"dense-overflow", 92}} {
		for _, operation := range []string{"check", "import", "shrink"} {
			t.Run(tc.shape+"/"+operation, func(t *testing.T) {
				f := pmIntegrityGraphFixture(t)
				pins, roots := pmIntegrityRecordedGraph(tc.shape, tc.count)
				writeDependencyCheckAgreement(t, f, pins, roots...)
				manifest, lock := f.app.dependencyPaths()
				unrelated := freshRestoreFiles(t, f.app.EnvironmentStoreRoot, f.store.Path, f.app.ManagedRoot, f.operations, f.app.UserHome)
				switch operation {
				case "check":
					before := observeDependencyCheckFiles(t, f.root)
					// The graph is valid; bytes were intentionally not cached.
					// Offline check must admit the lock and report unavailable
					// packages, with no source acquisition or synthetic cap error.
					report, _ := executeDependencyCheck(t, f, dependencyCheckUnavailable)
					packages, validLock := 0, false
					for _, check := range report.Checks {
						validLock = validLock || check.Stage == "lock" && check.Outcome == dependencyCheckValid
						if check.Stage == "package" && check.Code == "package_unavailable" {
							packages++
						}
					}
					if !validLock || packages != len(pins) {
						t.Fatalf("recorded graph was not fully admitted: lock=%t packages=%d", validLock, packages)
					}
					assertDependencyCheckFilesUnchanged(t, f.root, before)
				case "import":
					exchange := filepath.Join(f.root, "exchange")
					if err := os.Mkdir(exchange, 0o700); err != nil {
						t.Fatal(err)
					}
					manifestBytes, lockBytes := dependencyRead(t, manifest), dependencyRead(t, lock)
					dependencyWrite(t, filepath.Join(exchange, "uap.toml"), manifestBytes)
					dependencyWrite(t, filepath.Join(exchange, "uap.lock"), lockBytes)
					writeDependencyCheckAgreement(t, f, nil)
					dependencyRun(t, f, "deps", "import", exchange, "--yes", "--format", "json")
					if dependencyRead(t, manifest) != manifestBytes || dependencyRead(t, lock) != lockBytes {
						t.Fatal("historical agreement changed during import")
					}
				case "shrink":
					// Keep the original lock while explicitly reducing intent.
					// New solver limits must not trap users in an old graph.
					dependencyWrite(t, manifest, "schema_version = 1\n")
					dependencyRun(t, f, "deps", "lock", "--format", "json")
					requirements, err := readDependencyManifest(manifest)
					if err != nil {
						t.Fatal(err)
					}
					reduced, err := readDependencyLock(lock, requirements)
					if err != nil || len(reduced.Packages) != 0 {
						t.Fatalf("could not reduce historical graph: packages=%d error=%v", len(reduced.Packages), err)
					}
				}
				pmArchiveAssertEqual(t, "unrelated persistent files", unrelated, freshRestoreFiles(t, f.app.EnvironmentStoreRoot, f.store.Path, f.app.ManagedRoot, f.operations, f.app.UserHome))
			})
		}
	}
}

func pmIntegrityPersistentFiles(t *testing.T, f cliFixture) map[string]freshRestoreStamp {
	t.Helper()
	manifest, lock := f.app.dependencyPaths()
	return freshRestoreFiles(t, manifest, lock, f.app.EnvironmentStoreRoot, f.store.Path, f.app.ManagedRoot, f.operations, f.app.UserHome)
}
