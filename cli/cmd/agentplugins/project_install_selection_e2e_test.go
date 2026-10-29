package main

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
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
