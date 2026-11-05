package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// Exercise inferred selection through the production composer and installer,
// with a single inert Gemini project fixture and real Git transport.
func TestProjectBareInstallCreatesAndReplaysExactAgreement(t *testing.T) {
	transport := newProjectJourneyTransport(t, "git")
	journey := newProjectJourney(t, transport)
	projectJourneyWrite(t, filepath.Join(journey.cache, "uap.toml"), "schema_version = 1\n[dependencies]\n[targets]\n")
	userManifest := projectJourneyRead(t, filepath.Join(journey.cache, "uap.toml"))
	userNative := securityFixtureTree(t, journey.nativeRoot(""))
	projectJourneyWrite(t, filepath.Join(journey.a, "uap.toml"), fmt.Sprintf("schema_version = 1\n[dependencies.docs]\ngit = %q\nref = \"main\"\n[targets]\ngemini = [\"docs\"]\n", transport.endpoint))
	projectJourneyAbsent(t, filepath.Join(journey.a, "uap.lock"))
	t.Chdir(journey.a)
	// An empty project argument deliberately appends no scope/root flags.
	journey.run(t, "", "install", "--yes")
	journey.assertOwnedContext(t, journey.a)
	pair := journey.assertPortablePair(t, journey.a)
	projectBefore := securityFixtureTree(t, journey.a)
	gitBefore := transport.diagnostic.countSnapshot()
	if transport.diagnostic.counts["ls-remote"] == 0 {
		t.Fatal("first install did not exercise Git acquisition")
	}
	journey.run(t, "", "install", "--yes")
	if pair != journey.assertPortablePair(t, journey.a) || !reflect.DeepEqual(projectBefore, securityFixtureTree(t, journey.a)) {
		t.Fatal("repeat bare install changed project pins, agreement or owned bytes")
	}
	if gitBefore != transport.diagnostic.countSnapshot() {
		t.Fatal("repeat bare install performed Git lookup or acquisition")
	}
	if userManifest != projectJourneyRead(t, filepath.Join(journey.cache, "uap.toml")) || !reflect.DeepEqual(userNative, securityFixtureTree(t, journey.nativeRoot(""))) {
		t.Fatal("inferred project install changed user agreement or native profile")
	}
	projectJourneyAbsent(t, filepath.Join(journey.cache, "uap.lock"), filepath.Join(journey.cache, "state-v2.json"))
	// Explicit user scope must bypass the cwd project and create only its empty
	// user lock. The shared immutable content cache is intentionally scope-neutral.
	journey.run(t, "", "install", "--scope", "user", "--yes")
	journey.assertPortablePair(t, journey.cache)
	if !reflect.DeepEqual(projectBefore, securityFixtureTree(t, journey.a)) || !reflect.DeepEqual(userNative, securityFixtureTree(t, journey.nativeRoot(""))) {
		t.Fatal("explicit user override changed project state or native profiles")
	}
}
// Reuse the production Git/CLI/project composition fixture for a required MCP
// child. Git acquisition, lock, transactions and receipts are real; the Gemini
// detector is a fixture and this does not claim a native MCP tools/call.
func TestProjectRequiredChildColdRestoreAndScopedRemoval(t *testing.T) {
	transport := newProjectJourneyTransport(t, "git")
	projectJourneyWrite(t, filepath.Join(transport.repo, "child", "plugin.json"),
		"{\"$schema\":\"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json\",\"name\":\"project-child\",\"version\":\"1.0.0\"}")
	projectJourneyWrite(t, filepath.Join(transport.repo, "child", "mcp.json"),
		"{\"$schema\":\"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json\",\"mcpServers\":{\"child\":{\"type\":\"stdio\",\"command\":\"go\",\"args\":[\"version\"]}}}")
	projectJourneyWrite(t, filepath.Join(transport.repo, "uap.toml"), fmt.Sprintf(
		"schema_version = 1\n[dependencies.child]\ngit = %q\nref = \"main\"\npath = \"child\"\n", transport.endpoint))
	projectJourneyGit(t, transport.repo, "add", ".")
	projectJourneyGit(t, transport.repo, "commit", "-m", "add required MCP child")
	projectJourneyGit(t, transport.repo, "push", "--quiet", transport.remote, "main")

	journey := newProjectJourney(t, transport)
	manifest := fmt.Sprintf("schema_version = 1\n[dependencies.parent]\ngit = %q\nref = \"main\"\n[targets]\ngemini = [\"parent\"]\n", transport.endpoint)
	scopes := []struct {
		root string
		pair string
	}{
		{"", journey.cache},
		{journey.a, journey.a},
		{journey.b, journey.b},
	}
	var pinned string
	for _, scope := range scopes {
		projectJourneyWrite(t, filepath.Join(scope.pair, "uap.toml"), manifest)
		journey.run(t, scope.root, "deps", "lock")
		journey.run(t, scope.root, "deps", "sync", "--locked", "--yes")
		state := journey.state(t, scope.root)
		if len(state.Installations) != 2 || len(state.EnvironmentReceipts) != 2 {
			t.Fatalf("scope %q did not install root plus required child: %+v", scope.root, state)
		}
		var lock environment.Lock
		body := projectJourneyRead(t, filepath.Join(scope.pair, "uap.lock"))
		if err := json.Unmarshal([]byte(body), &lock); err != nil || len(lock.Packages) != 2 {
			t.Fatalf("scope %q did not pin both packages: %+v %v", scope.root, lock, err)
		}
		foundChild := false
		for _, pkg := range lock.Packages {
			if pkg.Source.Git == nil || len(pkg.Targets) != 1 || pkg.Targets[0] != "gemini" {
				t.Fatalf("non-Git or cross-client graph pin: %+v", pkg)
			}
			if len(pkg.RequiredBy) > 0 {
				foundChild = len(pkg.RequiredBy) == 1 && pkg.RequiredBy[0].Alias == "parent"
			}
		}
		if !foundChild {
			t.Fatalf("required child edge missing from %s lock: %+v", scope.pair, lock.Packages)
		}
		if pinned != "" && pinned != body {
			t.Fatal("same graph produced scope-dependent portable locks")
		}
		pinned = body
		settings := projectJourneyRead(t, filepath.Join(journey.nativeRoot(scope.root), "settings.json"))
		for _, name := range []string{"\"docs\"", "\"child\"", "foreign.test"} {
			if !strings.Contains(settings, name) {
				t.Fatalf("scope %q did not deliver both owned MCP entries or preserve foreign: %s", scope.root, settings)
			}
		}
	}

	userState := projectJourneyRead(t, filepath.Join(journey.cache, "state-v2.json"))
	bState := projectJourneyRead(t, filepath.Join(journey.namespace(journey.b), "state-v2.json"))
	userNative := securityFixtureTree(t, journey.nativeRoot(""))
	bNative := securityFixtureTree(t, journey.nativeRoot(journey.b))
	aStatePath := filepath.Join(journey.namespace(journey.a), "state-v2.json")
	aState := projectJourneyRead(t, aStatePath)
	beforeGit := transport.diagnostic.countSnapshot()
	journey.run(t, journey.a, "deps", "sync", "--locked", "--yes")
	if aState != projectJourneyRead(t, aStatePath) || beforeGit != transport.diagnostic.countSnapshot() {
		t.Fatal("repeat locked closure changed receipts or fetched moving Git refs")
	}

	// Fresh project and fresh UAP store: only portable agreement files cross.
	fresh := newProjectJourney(t, transport)
	projectJourneyWrite(t, filepath.Join(fresh.a, "uap.toml"), manifest)
	projectJourneyWrite(t, filepath.Join(fresh.a, "uap.lock"), pinned)
	projectJourneyAbsent(t, filepath.Join(fresh.a, ".uap", "state-v2.json"), filepath.Join(fresh.cache, "store"))
	fresh.run(t, fresh.a, "deps", "sync", "--locked", "--yes")
	if len(fresh.state(t, fresh.a).EnvironmentReceipts) != 2 || pinned != projectJourneyRead(t, filepath.Join(fresh.a, "uap.lock")) {
		t.Fatal("cold project restore lost required child receipts or rewrote pins")
	}
	freshSettings := projectJourneyRead(t, filepath.Join(fresh.nativeRoot(fresh.a), "settings.json"))
	if !strings.Contains(freshSettings, `"docs"`) || !strings.Contains(freshSettings, `"child"`) {
		t.Fatalf("cold project restore did not materialize both owned MCP entries: %s", freshSettings)
	}

	// Remove the entire closure only from A, without touching user or B.
	projectJourneyWrite(t, filepath.Join(journey.a, "uap.toml"), "schema_version = 1\n[dependencies]\n[targets]\ngemini = []\n")
	journey.run(t, journey.a, "deps", "lock")
	journey.run(t, journey.a, "deps", "sync", "--locked", "--yes")
	if len(journey.state(t, journey.a).EnvironmentReceipts) != 0 {
		t.Fatal("project A retained removed root/child ownership")
	}
	if userState != projectJourneyRead(t, filepath.Join(journey.cache, "state-v2.json")) ||
		bState != projectJourneyRead(t, filepath.Join(journey.namespace(journey.b), "state-v2.json")) ||
		!reflect.DeepEqual(userNative, securityFixtureTree(t, journey.nativeRoot(""))) ||
		!reflect.DeepEqual(bNative, securityFixtureTree(t, journey.nativeRoot(journey.b))) {
		t.Fatal("project A closure removal mutated user or project B")
	}
	aSettings := projectJourneyRead(t, filepath.Join(journey.nativeRoot(journey.a), "settings.json"))
	if !strings.Contains(aSettings, "foreign.test") || strings.Contains(aSettings, `"docs"`) || strings.Contains(aSettings, `"child"`) {
		t.Fatalf("project A closure removal retained an owned MCP entry or discarded foreign settings: %s", aSettings)
	}
}
