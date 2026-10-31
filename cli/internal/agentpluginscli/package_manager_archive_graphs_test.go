//go:build uappm_e2e

package agentpluginscli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func packageManagerArchiveCases() map[string]func(*testing.T) {
	return map[string]func(*testing.T){
		"depth32":                  pmArchiveDepth,
		"fanout64":                 pmArchiveFanout,
		"dense-shared-dag":         pmArchiveDense,
		"self-cycle":               pmArchiveSelfCycle,
		"three-cycle-removal":      pmArchiveCycleRemoval,
		"deterministic-order":      pmArchiveDeterministic,
		"shared-root-lifecycle":    pmArchiveSharedLifecycle,
		"mixed-target-inheritance": pmArchiveMixedTargets,
		"locked-replay":            pmArchiveReplay,
		"archive-refusals":         pmArchiveRefusals,
		"lock-refusals":            pmArchiveLockRefusals,
		"preview-no-effects":       pmArchivePreviews,
	}
}

func pmArchiveInstallGraph(t *testing.T, graph pmArchiveGraph, roots ...string) (*pmArchiveOrigin, pmArchiveEnvironment) {
	t.Helper()
	origin := newPMArchiveOrigin(t, graph)
	fixture := newPMArchiveEnvironment(t, origin, false)
	fixture.manifest(t, origin, roots, map[string][]string{"gemini": roots})
	fixture.run(t, "install", "--yes", "--format", "json")
	fixture.assertGraph(t, origin, graph, pmArchiveAll(graph, "gemini"))
	fixture.assertInstalled(t, pmArchiveAll(graph, "gemini"))
	return origin, fixture
}

func pmArchiveDepth(t *testing.T) {
	graph := pmArchiveGraph{}
	for index := range 32 {
		name := fmt.Sprintf("n%02d", index)
		graph[name] = nil
		if index != 31 {
			graph[name] = []string{fmt.Sprintf("n%02d", index+1)}
		}
	}
	_, fixture := pmArchiveInstallGraph(t, graph, "n00")
	pmArchiveTree(t, fixture, []string{"n00"}, len(graph))
}

func pmArchiveFanout(t *testing.T) {
	graph := pmArchiveGraph{"root": {}}
	for index := range 64 {
		name := fmt.Sprintf("leaf%02d", index)
		graph[name] = nil
		graph["root"] = append(graph["root"], name)
	}
	pmArchiveInstallGraph(t, graph, "root")
}

func pmArchiveDense(t *testing.T) {
	// Nineteen nodes and 78 independently declared edges, including 36 shared
	// leaf demands. This checks edge retention rather than just package counts.
	graph := pmArchiveGraph{"root": {}}
	for layer := range 3 {
		for index := range 6 {
			name := fmt.Sprintf("%c%02d", 'a'+layer, index)
			graph[name] = nil
			if layer == 0 {
				graph["root"] = append(graph["root"], name)
			}
			if layer < 2 {
				for child := range 6 {
					graph[name] = append(graph[name], fmt.Sprintf("%c%02d", 'b'+layer, child))
				}
			}
		}
	}
	pmArchiveInstallGraph(t, graph, "root")
}

func pmArchiveTree(t *testing.T, fixture pmArchiveEnvironment, roots []string, nodes int) {
	t.Helper()
	before := fixture.activeFiles(t)
	human := fixture.run(t, "deps", "tree")
	if len(human) > 4096+nodes*1024 {
		t.Fatalf("tree display expanded a cycle/shared DAG: %d bytes", len(human))
	}
	var tree dependencyTree
	if err := json.Unmarshal([]byte(fixture.run(t, "deps", "tree", "--format", "json")), &tree); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(tree.Roots, roots) || len(tree.Packages) != nodes {
		t.Fatalf("tree roots/nodes: %+v", tree)
	}
	pmArchiveAssertEqual(t, "read-only tree active files", before, fixture.activeFiles(t))
}

func pmArchiveSelfCycle(t *testing.T) {
	_, fixture := pmArchiveInstallGraph(t, pmArchiveGraph{"self": {"self"}}, "self")
	pmArchiveTree(t, fixture, []string{"self"}, 1)
}

func pmArchiveCycleRemoval(t *testing.T) {
	graph := pmArchiveGraph{"a": {"b"}, "b": {"c"}, "c": {"a"}}
	origin, fixture := pmArchiveInstallGraph(t, graph, "a", "b")
	initial := fixture.assertInstalled(t, pmArchiveAll(graph, "gemini"))
	fixture.run(t, "remove", "a", "--target", "gemini", "--format", "json")
	fixture.assertGraph(t, origin, graph, pmArchiveAll(graph, "gemini"))
	remaining := fixture.assertInstalled(t, pmArchiveAll(graph, "gemini"))
	for key, binding := range initial {
		if remaining[key].BindingID != binding.BindingID {
			t.Fatalf("remaining root changed cycle binding %s", key)
		}
	}
	pmArchiveTree(t, fixture, []string{"b"}, 3)
	fixture.run(t, "remove", "b", "--target", "gemini", "--format", "json")
	fixture.assertGraph(t, origin, graph, pmArchiveTargets{})
	fixture.assertInstalled(t, pmArchiveTargets{})
	pmArchiveTree(t, fixture, []string{}, 0)
}

func pmArchiveDeterministic(t *testing.T) {
	graph := pmArchiveGraph{"a": {"c", "d"}, "b": {"d", "c"}, "c": {"e"}, "d": {"e"}, "e": nil}
	origin := newPMArchiveOrigin(t, graph)
	var previous any
	for _, order := range [][]string{{"a", "b"}, {"b", "a"}} {
		fixture := newPMArchiveEnvironment(t, origin, false)
		fixture.manifest(t, origin, order, map[string][]string{"gemini": order})
		fixture.run(t, "install", "--yes", "--format", "json")
		lock := fixture.assertGraph(t, origin, graph, pmArchiveAll(graph, "gemini"))
		fixture.assertInstalled(t, pmArchiveAll(graph, "gemini"))
		if previous != nil {
			pmArchiveAssertEqual(t, "same-origin equivalent order lock", previous, lock)
		}
		previous = lock
	}
}

func pmArchiveSharedLifecycle(t *testing.T) {
	graph := pmArchiveGraph{"a": {"c"}, "b": {"c"}, "c": nil}
	origin, fixture := pmArchiveInstallGraph(t, graph, "a", "b")
	initial := fixture.assertInstalled(t, pmArchiveAll(graph, "gemini"))["c/gemini"]
	requests := origin.counts()
	fixture.run(t, "add", origin.url("c"), "--alias", "c", "--target", "gemini", "--format", "json")
	pmArchiveTree(t, fixture, []string{"a", "b", "c"}, 3)
	fixture.run(t, "remove", "c", "--target", "gemini", "--format", "json")
	fixture.assertGraph(t, origin, graph, pmArchiveAll(graph, "gemini"))
	shared := fixture.assertInstalled(t, pmArchiveAll(graph, "gemini"))["c/gemini"]
	if initial.BindingID != shared.BindingID || !reflect.DeepEqual(initial.Owner, shared.Owner) {
		t.Fatal("promotion/demotion changed shared owner identity")
	}
	fixture.run(t, "remove", "a", "--target", "gemini", "--format", "json")
	expected := pmArchiveTargets{"b": {"gemini"}, "c": {"gemini"}}
	fixture.assertGraph(t, origin, graph, expected)
	if got := fixture.assertInstalled(t, expected)["c/gemini"]; got.BindingID != initial.BindingID {
		t.Fatal("shared removal recreated C")
	}
	pmArchiveTree(t, fixture, []string{"b"}, 2)
	pmArchiveAssertEqual(t, "promotion/demotion/removal source requests", requests, origin.counts())
	fixture.run(t, "remove", "b", "--target", "gemini", "--format", "json")
	fixture.assertGraph(t, origin, graph, pmArchiveTargets{})
	fixture.assertInstalled(t, pmArchiveTargets{})
}

func pmArchiveMixedTargets(t *testing.T) {
	graph := pmArchiveGraph{"a": {"c"}, "b": {"c"}, "c": {"d"}, "d": nil}
	origin := newPMArchiveOrigin(t, graph)
	fixture := newPMArchiveEnvironment(t, origin, true)
	fixture.manifest(t, origin, []string{"a", "b"}, map[string][]string{"gemini": {"a"}, "codex": {"b"}})
	fixture.run(t, "install", "--yes", "--format", "json")
	expected := pmArchiveTargets{"a": {"gemini"}, "b": {"codex"}, "c": {"codex", "gemini"}, "d": {"codex", "gemini"}}
	fixture.assertGraph(t, origin, graph, expected)
	bindings := fixture.assertInstalled(t, expected)
	// A foreign setting survives the second consumer's inherited target union.
	settings := dependencyRead(t, filepath.Join(fixture.clients["gemini"].ConfigRoot, "settings.json"))
	if !strings.Contains(settings, "https://example.test/pm/d") {
		t.Fatal("transitive target union omitted native Gemini configuration")
	}
	if bindings["a/codex"].BindingID != "" || bindings["b/gemini"].BindingID != "" {
		t.Fatal("target selection leaked to unrelated root")
	}
	fixture.run(t, "remove", "a", "--target", "gemini", "--format", "json")
	remaining := pmArchiveTargets{"b": {"codex"}, "c": {"codex"}, "d": {"codex"}}
	fixture.assertGraph(t, origin, graph, remaining)
	shrunk := fixture.assertInstalled(t, remaining)
	for _, name := range []string{"c", "d"} {
		key := name + "/codex"
		if shrunk[key].BindingID != bindings[key].BindingID {
			t.Fatalf("removing Gemini consumer recreated shared Codex binding %s", name)
		}
	}
}
