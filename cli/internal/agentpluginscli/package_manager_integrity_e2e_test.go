//go:build uappm_e2e

package agentpluginscli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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
// client discovery, installation or security refresh even at the graph ceiling.
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
		// All consumers are distinct and every node is a root. Refusal must be
		// the resource limit, not an incidental duplicate or unreachable edge.
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
	}{{"independent", 1024}, {"chain", 256}} {
		t.Run(tc.shape, func(t *testing.T) {
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
	for _, tc := range []struct {
		shape string
		count int
	}{{"independent", 1025}, {"dense-overflow", 92}} {
		for _, operation := range []string{"tree", "check", "import", "lock"} {
			t.Run(tc.shape+"/"+operation, func(t *testing.T) {
				f := pmIntegrityGraphFixture(t)
				pins, roots := pmIntegrityRecordedGraph(tc.shape, tc.count)
				writeDependencyCheckAgreement(t, f, pins, roots...)
				args := []string{"deps", operation, "--format", "json"}
				if operation == "import" {
					exchange := filepath.Join(f.root, "exchange")
					if err := os.Mkdir(exchange, 0o700); err != nil {
						t.Fatal(err)
					}
					mp, lp := f.app.dependencyPaths()
					dependencyWrite(t, filepath.Join(exchange, "uap.toml"), dependencyRead(t, mp))
					dependencyWrite(t, filepath.Join(exchange, "uap.lock"), dependencyRead(t, lp))
					writeDependencyCheckAgreement(t, f, nil)
					args = append(args, exchange, "--yes")
				}
				before := observeDependencyCheckFiles(t, f.root)
				persistentBefore := pmIntegrityPersistentFiles(t, f)
				if operation == "check" {
					report, _ := executeDependencyCheck(t, f, dependencyCheckError)
					requireDependencyCheckCode(t, report, "graph_limit")
				} else {
					out, stderr, err := f.execute(false, args...)
					if !errors.Is(err, environment.ErrResolutionLimit) {
						t.Fatalf("%s failed outside graph admission: %v stdout=%s stderr=%s", operation, err, out, stderr)
					}
				}
				if operation == "lock" {
					// Mutation commands reserve their coordination lock before
					// admission. The agreement, cache and installed state must
					// nevertheless remain byte-for-byte and timestamp stable.
					pmArchiveAssertEqual(t, "refused lock persistent files", persistentBefore, pmIntegrityPersistentFiles(t, f))
				} else {
					assertDependencyCheckFilesUnchanged(t, f.root, before)
				}
			})
		}
	}
	t.Run("fresh-resolution", func(t *testing.T) {
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
			t.Fatalf("fresh oversized roots escaped admission: %v stdout=%s stderr=%s", err, out, stderr)
		}
		if !reflect.DeepEqual(before, pmIntegrityPersistentFiles(t, f)) {
			t.Fatal("oversized fresh resolution published agreement, cache or installed state")
		}
	})
}

func pmIntegrityPersistentFiles(t *testing.T, f cliFixture) map[string]freshRestoreStamp {
	t.Helper()
	manifest, lock := f.app.dependencyPaths()
	return freshRestoreFiles(t, manifest, lock, f.app.EnvironmentStoreRoot, f.store.Path, f.app.ManagedRoot, f.operations, f.app.UserHome)
}
