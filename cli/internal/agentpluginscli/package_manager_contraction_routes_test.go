//go:build uappm_e2e

package agentpluginscli

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func pmContractionGraph() pmArchiveGraph {
	return pmArchiveGraph{"a": {"c"}, "b": {"c", "d"}, "c": nil, "d": {"e"}, "e": {"d"}}
}

func pmIntegrityContractionRoutes(t *testing.T) {
	for _, route := range []string{"remove", "lock-and-replay", "sync"} {
		t.Run(route, func(t *testing.T) {
			graph := pmContractionGraph()
			origin, fixture := pmArchiveInstallGraph(t, graph, "a", "b")
			original := fixture.assertInstalled(t, pmArchiveAll(graph, "gemini"))
			pins, requests := fixture.lock(t), origin.counts()
			if route == "remove" {
				before := pmIntegrityPersistentFiles(t, fixture.cliFixture)
				fixture.run(t, "remove", "b", "--target", "gemini", "--dry-run", "--format", "json")
				pmArchiveAssertEqual(t, "saved removal preview", before, pmIntegrityPersistentFiles(t, fixture.cliFixture))
				fixture.run(t, "remove", "b", "--target", "gemini", "--format", "json")
			} else {
				fixture.manifest(t, origin, []string{"a"}, map[string][]string{"gemini": {"a"}})
				pmAssertContractionInstallStrict(t, fixture)
				if route == "lock-and-replay" {
					fixture.run(t, "deps", "lock", "--format", "json")
					fixture.run(t, "install", "--locked", "--yes", "--format", "json")
				} else {
					fixture.run(t, "deps", "sync", "--yes", "--format", "json")
				}
			}
			expected := pmArchiveTargets{"a": {"gemini"}, "c": {"gemini"}}
			lock := fixture.assertGraph(t, origin, graph, expected)
			installed := fixture.assertInstalled(t, expected)
			for _, pin := range lock.Packages {
				old, _ := pins.Package(pin.Alias)
				if !environment.SamePin(old, pin) {
					t.Fatalf("contraction changed immutable pin %s", pin.Alias)
				}
				key := pin.Alias + "/gemini"
				if installed[key].BindingID != original[key].BindingID {
					t.Fatalf("contraction recreated retained binding %s", key)
				}
			}
			pmArchiveAssertEqual(t, "warm contraction source requests", requests, origin.counts())
			offline := fixture.cliFixture
			offline.app.SourceAcquirer = dependencyCheckForbiddenEffects{}
			offline.app.DirectoryClient = dependencyCheckForbiddenEffects{}
			offline.app.Detector = dependencyCheckForbiddenEffects{}
			offline.app.SecurityEvaluator = dependencyCheckForbiddenEffects{}
			executeDependencyCheck(t, offline, dependencyCheckValid)
		})
	}
	t.Run("cold-retained-only", pmIntegrityColdContraction)
	t.Run("corrupt-retained", pmIntegrityCorruptContraction)
	t.Run("cancel-retained-read", pmIntegrityCancelledContraction)
}

func pmAssertContractionInstallStrict(t *testing.T, fixture pmArchiveEnvironment) {
	t.Helper()
	before := pmIntegrityPersistentFiles(t, fixture.cliFixture)
	for _, locked := range []bool{false, true} {
		args := []string{"install", "--yes", "--format", "json"}
		if locked {
			args = append(args, "--locked")
		}
		_, err := fixture.executePM(t.Context(), t, "", false, args...)
		if err == nil || !strings.Contains(err.Error(), "manifest") {
			t.Fatalf("install silently reconciled edited intent (locked=%t): %v", locked, err)
		}
		pmArchiveAssertEqual(t, "stale install refusal", before, pmIntegrityPersistentFiles(t, fixture.cliFixture))
	}
}

func pmIntegrityColdContraction(t *testing.T) {
	graph := pmContractionGraph()
	origin, original := pmArchiveInstallGraph(t, graph, "a", "b")
	fixture := newPMArchiveEnvironment(t, origin, false)
	pmArchiveAgreement(t, original, fixture)
	fixture.manifest(t, origin, []string{"a"}, map[string][]string{"gemini": {"a"}})
	requests := origin.counts()
	fixture.run(t, "deps", "lock", "--format", "json")
	expected := pmArchiveTargets{"a": {"gemini"}, "c": {"gemini"}}
	fixture.assertGraph(t, origin, graph, expected)
	for _, name := range []string{"a", "b", "c", "d", "e"} {
		downloads := origin.counts()["GET /"+name+".zip"] - requests["GET /"+name+".zip"]
		retained := name == "a" || name == "c"
		if retained && downloads == 0 || !retained && downloads != 0 {
			t.Fatalf("cold contraction %s downloads=%d retained=%t", name, downloads, retained)
		}
	}
	requests = origin.counts()
	fixture.run(t, "install", "--locked", "--yes", "--format", "json")
	fixture.assertInstalled(t, expected)
	pmArchiveAssertEqual(t, "replay uses contracted warm cache", requests, origin.counts())
}

func pmIntegrityCorruptContraction(t *testing.T) {
	origin, fixture := pmArchiveInstallGraph(t, pmContractionGraph(), "a", "b")
	fixture.manifest(t, origin, []string{"a"}, map[string][]string{"gemini": {"a"}})
	// Read the previous agreement without pretending it matches the edited root.
	_, lockPath := fixture.app.dependencyPaths()
	requirements, err := readDependencyManifest(filepath.Join(fixture.app.DependenciesRoot, "uap.toml"))
	if err != nil {
		t.Fatal(err)
	}
	previous, err := readCompatibleDependencyLock(lockPath, requirements)
	if err != nil {
		t.Fatal(err)
	}
	pin, _ := previous.Package("c")
	stored, err := fixture.app.dependencyStore().Get(t.Context(), pin.DigestAlgorithm, pin.TreeDigest)
	if err != nil {
		t.Fatal(err)
	}
	dependencyWrite(t, filepath.Join(stored.Root, "marker.txt"), "corrupt retained bytes\n")
	before, requests := pmIntegrityPersistentFiles(t, fixture.cliFixture), origin.counts()
	_, err = fixture.executePM(t.Context(), t, "", false, "deps", "lock", "--format", "json")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "store") {
		t.Fatalf("corrupt retained cache was accepted or replaced: %v", err)
	}
	pmArchiveAssertEqual(t, "corrupt contraction files", before, pmIntegrityPersistentFiles(t, fixture.cliFixture))
	pmArchiveAssertEqual(t, "corrupt contraction source requests", requests, origin.counts())
}

func pmIntegrityCancelledContraction(t *testing.T) {
	origin, fixture := pmArchiveInstallGraph(t, pmContractionGraph(), "a", "b")
	fixture.manifest(t, origin, []string{"a"}, map[string][]string{"gemini": {"a"}})
	before, requests := pmIntegrityPersistentFiles(t, fixture.cliFixture), origin.counts()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	fixture.app.PackageLoader = dependencyCheckHookLoader{delegate: fixture.app.PackageLoader, hook: func(domain.LoadInput) { cancel() }}
	_, err := fixture.executePM(ctx, t, "", false, "deps", "lock", "--format", "json")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("retained read cancellation was not terminal: %v", err)
	}
	pmArchiveAssertEqual(t, "cancelled contraction files", before, pmIntegrityPersistentFiles(t, fixture.cliFixture))
	pmArchiveAssertEqual(t, "cancelled contraction source requests", requests, origin.counts())
}
