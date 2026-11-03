package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func productionProjectCheckCommand(t *testing.T, app agentpluginscli.App, project string) (*cobra.Command, *bytes.Buffer, map[string][]byte) {
	t.Helper()
	requirements := environment.Requirements{SchemaVersion: 1, Dependencies: map[string]environment.Requirement{}, Targets: map[string][]string{}}
	digest, err := environment.ManifestDigest(requirements)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(environment.Lock{LockVersion: 1, ManifestDigest: digest, Packages: []environment.LockedPackage{}})
	if err != nil {
		t.Fatal(err)
	}
	agreement := map[string][]byte{
		filepath.Join(project, "uap.toml"): []byte("schema_version = 1\n"),
		filepath.Join(project, "uap.lock"): body,
	}
	for path, body := range agreement {
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	output := &bytes.Buffer{}
	app.Output, app.ErrorOutput = output, output
	command := agentpluginscli.NewRoot(app)
	command.SetArgs([]string{"deps", "check", "--scope", "project", "--project-root", project, "--format", "json"})
	return command, output, agreement
}

func assertProductionProjectCheckUnchanged(t *testing.T, output *bytes.Buffer, agreement map[string][]byte, state string, before os.FileInfo) {
	t.Helper()
	var report struct{ Outcome, Scope string }
	if err := json.Unmarshal(output.Bytes(), &report); err != nil || report.Outcome != "valid" || report.Scope != "project" {
		t.Fatalf("production project check failed: %s, %v", output.String(), err)
	}
	for path, want := range agreement {
		if actual, err := os.ReadFile(path); err != nil || !bytes.Equal(actual, want) {
			t.Fatalf("check changed agreement %s: %v", path, err)
		}
	}
	after, err := os.Lstat(state)
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("check changed unrelated installed state: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(state))
	if err != nil || len(entries) != 1 || entries[0].Name() != "state-v2.json" {
		t.Fatalf("check changed installed-state namespace: %+v, %v", entries, err)
	}
}

func TestProductionDependencyCheckIgnoresUnrelatedInvalidState(t *testing.T) {
	app, _, project, _ := projectCompositionFixture(t)
	state := filepath.Join(project, ".uap", "state-v2.json")
	if err := os.MkdirAll(filepath.Dir(state), 0700); err != nil {
		t.Fatal(err)
	}
	original := []byte("invalid unrelated installed state: preserve\n")
	if err := os.WriteFile(state, original, 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(state)
	if err != nil {
		t.Fatal(err)
	}
	command, output, agreement := productionProjectCheckCommand(t, app, project)
	if err := command.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	assertProductionProjectCheckUnchanged(t, output, agreement, state, before)
	if actual, err := os.ReadFile(state); err != nil || !bytes.Equal(actual, original) {
		t.Fatalf("check changed invalid state bytes: %v", err)
	}
	// Existing installation/status commands retain the full state preflight.
	command = agentpluginscli.NewRoot(app)
	command.SetArgs([]string{"deps", "status", "--scope", "project", "--project-root", project, "--format", "json"})
	if err := command.ExecuteContext(t.Context()); err == nil {
		t.Fatal("status lost its installed-state preflight")
	}
}
