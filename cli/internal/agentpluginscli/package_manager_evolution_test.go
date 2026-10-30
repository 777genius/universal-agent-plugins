//go:build uappm_e2e

package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func packageManagerEvolutionCases() map[string]func(*testing.T) {
	return map[string]func(*testing.T){
		"graph-edge-replacement":     pmEvolutionReplaceEdges,
		"graph-update-failure-retry": pmEvolutionFailedUpdate,
		"cascade-remove-drift":       pmEvolutionRemoveDrift,
		"prune-drift-after-lock":     pmEvolutionPruneDrift,
		"competing-lock-writers":     pmEvolutionCompetingWriters,
		"selective-cache-hydration":  pmEvolutionSelectiveHydration,
	}
}

func pmEvolutionSidecar(origin *pmArchiveOrigin, children ...string) string {
	body := "schema_version = 1\n"
	for _, child := range children {
		body += fmt.Sprintf("[dependencies.%s]\narchive = %q\n", child, origin.url(child))
	}
	return body
}

// Active effects and the portable pair are compared separately from immutable
// acquisition cache and advisory lock metadata. A late failed publication may
// legitimately retain verified cache objects.
func pmEvolutionFiles(t *testing.T, fixture pmArchiveEnvironment) map[string]freshRestoreStamp {
	t.Helper()
	mp, lp := fixture.app.dependencyPaths()
	files := fixture.activeFiles(t)
	for path, stamp := range freshRestoreFiles(t, mp, lp, fixture.operations, filepath.Join(fixture.root, "data", "plugin-data")) {
		files[path] = stamp
	}
	return files
}

// These remote-only packages do not allocate PLUGIN_DATA receipts. This is an
// independently authored, preexisting unrelated user-data slot in the exact
// PluginDataBase configured by newDependencyHarness, not a claimed allocation.
func pmEvolutionSeedData(t *testing.T, fixture pmArchiveEnvironment) string {
	t.Helper()
	root := filepath.Join(fixture.root, "data", "plugin-data", "foreign-user-slot")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "retained-user-data.txt")
	dependencyWrite(t, path, "independent user data survives failed and successful lifecycle changes\n")
	return path
}

func pmEvolutionSameBindings(t *testing.T, before, after map[string]installer.InspectedBinding, keys ...string) {
	t.Helper()
	for _, key := range keys {
		old, next := before[key], after[key]
		if old.BindingID == "" || old.BindingID != next.BindingID || old.TreeDigest != next.TreeDigest || old.Revision != next.Revision || old.TargetPath != next.TargetPath {
			t.Fatalf("unselected/shared binding changed %s: before=%+v after=%+v", key, old, next)
		}
	}
}

func pmEvolutionReplaceEdges(t *testing.T) {
	graph := pmArchiveGraph{"a": {"b"}, "b": {"c"}, "c": nil, "x": {"c"}, "d": {"e"}, "e": nil}
	origin := newPMArchiveOrigin(t, graph)
	fixture := newPMArchiveEnvironment(t, origin, false)
	fixture.manifest(t, origin, []string{"a", "x"}, map[string][]string{"gemini": {"a", "x"}})
	fixture.run(t, "install", "--yes", "--format", "json")
	initialExpected := pmArchiveTargets{"a": {"gemini"}, "b": {"gemini"}, "c": {"gemini"}, "x": {"gemini"}}
	fixture.assertGraph(t, origin, graph, initialExpected)
	initial := fixture.assertInstalled(t, initialExpected)
	data := pmEvolutionSeedData(t, fixture)
	dataBefore := freshRestoreFiles(t, data)
	retained := freshRestoreFiles(t, initial["c/gemini"].TargetPath, initial["x/gemini"].TargetPath)
	origin.sidecar(t, "a", pmEvolutionSidecar(origin, "d"))
	graph["a"] = []string{"d"}
	fixture.run(t, "deps", "update", "a", "--format", "json")
	expected := pmArchiveTargets{"a": {"gemini"}, "c": {"gemini"}, "d": {"gemini"}, "e": {"gemini"}, "x": {"gemini"}}
	fixture.assertGraph(t, origin, graph, expected)
	fixture.run(t, "install", "--locked", "--yes", "--format", "json")
	updated := fixture.assertInstalled(t, expected)
	pmEvolutionSameBindings(t, initial, updated, "c/gemini", "x/gemini")
	pmArchiveAssertEqual(t, "retained shared/unselected payloads", retained, freshRestoreFiles(t, initial["c/gemini"].TargetPath, initial["x/gemini"].TargetPath))
	if got := dependencyRead(t, filepath.Join(updated["a/gemini"].TargetPath, "uap.toml")); got != pmEvolutionSidecar(origin, "d") {
		t.Fatalf("updated A retained an old declaration: %q", got)
	}
	if _, err := os.Lstat(initial["b/gemini"].TargetPath); !os.IsNotExist(err) {
		t.Fatalf("unreachable B projection survived: %v", err)
	}
	fixture.run(t, "remove", "x", "--target", "gemini", "--format", "json")
	final := pmArchiveTargets{"a": {"gemini"}, "d": {"gemini"}, "e": {"gemini"}}
	fixture.assertGraph(t, origin, graph, final)
	fixture.assertInstalled(t, final)
	pmArchiveAssertEqual(t, "graph evolution retained unrelated user data", dataBefore, freshRestoreFiles(t, data))
	if _, err := os.Lstat(initial["c/gemini"].TargetPath); !os.IsNotExist(err) {
		t.Fatalf("last-consumer removal retained C projection: %v", err)
	}
}

func pmEvolutionFailedUpdate(t *testing.T) {
	graph := pmArchiveGraph{"a": {"b"}, "b": nil, "d": {"e"}, "e": nil}
	origin := newPMArchiveOrigin(t, graph)
	fixture := newPMArchiveEnvironment(t, origin, false)
	fixture.manifest(t, origin, []string{"a"}, map[string][]string{"gemini": {"a"}})
	fixture.run(t, "install", "--yes", "--format", "json")
	initialExpected := pmArchiveTargets{"a": {"gemini"}, "b": {"gemini"}}
	initial := fixture.assertInstalled(t, initialExpected)
	data := pmEvolutionSeedData(t, fixture)
	dataBefore := freshRestoreFiles(t, data)
	before, requests := pmEvolutionFiles(t, fixture), origin.counts()
	origin.sidecar(t, "a", pmEvolutionSidecar(origin, "d"))
	origin.mu.Lock()
	savedE := origin.payloads["/e.zip"]
	delete(origin.payloads, "/e.zip")
	origin.mu.Unlock()
	_, err := fixture.executePM(t.Context(), t, "", false, "deps", "update", "a", "--format", "json")
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("new missing graph leaf did not refuse acquisition: %v", err)
	}
	for _, name := range []string{"a", "d", "e"} {
		if key := "GET /" + name + ".zip"; origin.counts()[key] <= requests[key] {
			t.Fatalf("failed update never reached expected source %s", key)
		}
	}
	pmArchiveAssertEqual(t, "failed graph update active effects/agreement", before, pmEvolutionFiles(t, fixture))
	fixture.assertInstalled(t, initialExpected)
	origin.mu.Lock()
	origin.payloads["/e.zip"] = savedE
	origin.mu.Unlock()
	fixture.run(t, "deps", "update", "a", "--format", "json")
	fixture.run(t, "install", "--locked", "--yes", "--format", "json")
	graph["a"] = []string{"d"}
	expected := pmArchiveTargets{"a": {"gemini"}, "d": {"gemini"}, "e": {"gemini"}}
	fixture.assertGraph(t, origin, graph, expected)
	bindings := fixture.assertInstalled(t, expected)
	pmArchiveAssertEqual(t, "failed update and retry retained unrelated user data", dataBefore, freshRestoreFiles(t, data))
	if _, err := os.Lstat(initial["b/gemini"].TargetPath); !os.IsNotExist(err) {
		t.Fatalf("recovered update retained unreachable B projection: %v", err)
	}
	if dependencyRead(t, filepath.Join(bindings["a/gemini"].TargetPath, "uap.toml")) != pmEvolutionSidecar(origin, "d") {
		t.Fatal("recovered update installed old A payload")
	}
}

func pmEvolutionRemoveDrift(t *testing.T) {
	graph := pmArchiveGraph{"a": {"b"}, "b": {"c"}, "c": nil, "x": nil}
	origin, fixture := pmArchiveInstallGraph(t, graph, "a", "x")
	initial := fixture.assertInstalled(t, pmArchiveAll(graph, "gemini"))
	data := pmEvolutionSeedData(t, fixture)
	dataBefore := freshRestoreFiles(t, data)
	edited := filepath.Join(initial["b/gemini"].TargetPath, "user-edit.txt")
	dependencyWrite(t, edited, "preserve B's user change before cascading removal\n")
	before, requests := pmEvolutionFiles(t, fixture), origin.counts()
	_, err := fixture.executePM(t.Context(), t, "", false, "remove", "a", "--target", "gemini", "--format", "json")
	if err == nil {
		t.Fatal("cascading removal erased an edited transitive projection")
	}
	pmArchiveAssertEqual(t, "refused cascading removal", before, pmEvolutionFiles(t, fixture))
	pmArchiveAssertEqual(t, "refused removal acquisition", requests, origin.counts())
	if err := os.Remove(edited); err != nil {
		t.Fatal(err)
	}
	fixture.run(t, "remove", "a", "--target", "gemini", "--format", "json")
	expected := pmArchiveTargets{"x": {"gemini"}}
	fixture.assertGraph(t, origin, graph, expected)
	remaining := fixture.assertInstalled(t, expected)
	pmArchiveAssertEqual(t, "cascading removal retained unrelated user data", dataBefore, freshRestoreFiles(t, data))
	pmEvolutionSameBindings(t, initial, remaining, "x/gemini")
	for _, name := range []string{"a", "b", "c"} {
		if _, err := os.Lstat(initial[name+"/gemini"].TargetPath); !os.IsNotExist(err) {
			t.Fatalf("removed closure member %s survived: %v", name, err)
		}
	}
}

func pmEvolutionPruneDrift(t *testing.T) {
	graph := pmArchiveGraph{"a": {"b"}, "b": nil}
	origin, fixture := pmArchiveInstallGraph(t, graph, "a")
	initial := fixture.assertInstalled(t, pmArchiveAll(graph, "gemini"))
	data := pmEvolutionSeedData(t, fixture)
	dataBefore := freshRestoreFiles(t, data)
	edited := filepath.Join(initial["b/gemini"].TargetPath, "user-edit.txt")
	dependencyWrite(t, edited, "preserve unreachable B until its drift is resolved\n")
	origin.sidecar(t, "a", pmEvolutionSidecar(origin))
	fixture.run(t, "deps", "update", "a", "--format", "json")
	graph["a"] = nil
	expected := pmArchiveTargets{"a": {"gemini"}}
	fixture.assertGraph(t, origin, graph, expected)
	// The desired lock has already advanced. A failed sync preserves that pair
	// and all active old bytes; it does not promise to roll desired intent back.
	before, requests := pmEvolutionFiles(t, fixture), origin.counts()
	_, err := fixture.executePM(t.Context(), t, "", false, "deps", "sync", "--locked", "--yes", "--format", "json")
	if err == nil {
		t.Fatal("sync pruned a drifted old dependency")
	}
	pmArchiveAssertEqual(t, "failed prune current agreement and old active effects", before, pmEvolutionFiles(t, fixture))
	pmArchiveAssertEqual(t, "failed prune source requests", requests, origin.counts())
	if err := os.Remove(edited); err != nil {
		t.Fatal(err)
	}
	fixture.run(t, "deps", "sync", "--locked", "--yes", "--format", "json")
	updated := fixture.assertInstalled(t, expected)
	pmArchiveAssertEqual(t, "prune retained unrelated user data", dataBefore, freshRestoreFiles(t, data))
	if dependencyRead(t, filepath.Join(updated["a/gemini"].TargetPath, "uap.toml")) != pmEvolutionSidecar(origin) {
		t.Fatal("prune retry failed to install new A declaration")
	}
	if _, err := os.Lstat(initial["b/gemini"].TargetPath); !os.IsNotExist(err) {
		t.Fatalf("prune retry retained B: %v", err)
	}
}

func pmEvolutionCompetingWriters(t *testing.T) {
	graph := pmArchiveGraph{"a": {"c"}, "c": nil}
	origin := newPMArchiveOrigin(t, graph)
	fixture := newPMArchiveEnvironment(t, origin, false)
	fixture.manifest(t, origin, []string{"a"}, map[string][]string{"gemini": {"a"}})
	started, release := make(chan struct{}, 1), make(chan struct{})
	var once sync.Once
	resume := func() { once.Do(func() { close(release) }) }
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	defer resume()
	origin.mu.Lock()
	origin.blocked, origin.started, origin.release = "/a.zip", started, release
	origin.mu.Unlock()
	done := make(chan error, 1)
	go func() {
		defer close(done)
		_, err := fixture.executePM(ctx, t, "", false, "deps", "lock", "--format", "json")
		done <- err
	}()
	defer func() {
		resume()
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("source-barrier writer did not stop during cleanup")
		}
	}()
	select {
	case <-started:
	case err := <-done:
		t.Fatalf("first writer ended before its witnessed source barrier: %v", err)
	case <-ctx.Done():
		t.Fatal("first writer did not reach source barrier")
	}
	before, requests := pmEvolutionFiles(t, fixture), origin.counts()
	_, err := fixture.executePM(ctx, t, "", false, "deps", "lock", "--format", "json")
	if !errors.Is(err, processlock.ErrActive) {
		t.Fatalf("overlapping actual lock writer did not refuse with active mutation: %v", err)
	}
	pmArchiveAssertEqual(t, "contending writer active effects/agreement", before, pmEvolutionFiles(t, fixture))
	pmArchiveAssertEqual(t, "contending writer source requests", requests, origin.counts())
	resume()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("first writer failed after release: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("first writer did not finish after release")
	}
	fixture.assertGraph(t, origin, graph, pmArchiveAll(graph, "gemini"))
	_, lp := fixture.app.dependencyPaths()
	locked := freshRestoreFiles(t, lp)
	requests = origin.counts()
	fixture.run(t, "deps", "lock", "--format", "json")
	pmArchiveAssertEqual(t, "explicit retry agreement", locked, freshRestoreFiles(t, lp))
	pmArchiveAssertEqual(t, "explicit retry source requests", requests, origin.counts())
	fixture.run(t, "install", "--locked", "--yes", "--format", "json")
	fixture.assertInstalled(t, pmArchiveAll(graph, "gemini"))
}

func pmEvolutionSelectiveHydration(t *testing.T) {
	graph := pmArchiveGraph{"a": {"c"}, "b": {"c"}, "c": nil}
	origin, fixture := pmArchiveInstallGraph(t, graph, "a", "b")
	expected := pmArchiveAll(graph, "gemini")
	initial := fixture.assertInstalled(t, expected)
	pmEvolutionSeedData(t, fixture)
	pin, _ := fixture.lock(t).Package("c")
	stored, err := fixture.app.dependencyStore().Get(t.Context(), pin.DigestAlgorithm, pin.TreeDigest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Dir(stored.Root)); err != nil {
		t.Fatal(err)
	}
	before, requests := pmEvolutionFiles(t, fixture), origin.counts()
	fixture.run(t, "install", "--locked", "--yes", "--format", "json")
	pmArchiveAssertEqual(t, "missing-only hydration active effects/agreement", before, pmEvolutionFiles(t, fixture))
	for _, name := range []string{"a", "b", "c"} {
		want := 0
		if name == "c" {
			want = 1
		}
		key := "GET /" + name + ".zip"
		if delta := origin.counts()[key] - requests[key]; delta != want {
			t.Fatalf("missing-only hydration %s requested %d times, want %d", name, delta, want)
		}
	}
	if _, err := fixture.app.dependencyStore().Get(t.Context(), pin.DigestAlgorithm, pin.TreeDigest); err != nil {
		t.Fatalf("missing pinned store object was not hydrated: %v", err)
	}
	remaining := fixture.assertInstalled(t, expected)
	pmEvolutionSameBindings(t, initial, remaining, "a/gemini", "b/gemini", "c/gemini")
	requests = origin.counts()
	fixture.run(t, "install", "--locked", "--yes", "--format", "json")
	pmArchiveAssertEqual(t, "second selective replay requests", requests, origin.counts())
}
