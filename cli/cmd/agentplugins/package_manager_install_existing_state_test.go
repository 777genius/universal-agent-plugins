//go:build uappm_e2e

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// These cases use the public Cobra commands and production composition, HTTPS
// archive acquisition and native filesystem transactions. Host detection and
// security assessment are the existing inert installation-catalog fixtures;
// these cases do not certify a native agent process or the production scanner.
func pmInstallUnmanagedSourceRefusal(t *testing.T) {
	for _, scope := range []string{"user", "project"} {
		t.Run(scope, func(t *testing.T) {
			o := newPMInstallOrigin(t)
			f := newPMInstallJourney(t, o)
			project := ""
			if scope == "project" {
				project = f.a // Explicit root with spaces, outside the current directory.
			}
			f.run(t, project, "add", o.url(), "--no-save", "--target", "gemini")
			installation, _ := f.assertInstalled(t, project, "gemini", "v1", false)
			pmExistingMalformedIntent(t, f, project, o)
			before := pmExistingSnapshot(t, f, project)
			for _, command := range []string{"add", "install"} {
				for _, targets := range []string{"cursor", "cursor,gemini"} {
					out, err := f.execute(t, project, "", false, command, o.url(), "--alias", "selected", "--target", targets, "--yes", "--format", "json")
					if err == nil {
						t.Fatalf("%s adopted an unmanaged sibling through %s: %s", command, targets, out)
					}
					pmExistingTypedConflict(t, err, o.url(), "unmanaged_binding", "selected", installation.InstallationID, "", "")
					pmInstallEqual(t, "unmanaged refusal preserves active files and metadata", before, pmExistingSnapshot(t, f, project))
				}
			}
			pmExistingRunBoundary(t, f, project, o.url(), installation.InstallationID)
			requests, assessments := o.count(), f.security.calls
			out, err := f.execute(t, project, "", true, "add", o.url())
			pmExistingTypedConflict(t, err, o.url(), "unmanaged_binding", "", installation.InstallationID, "", "")
			if o.count() != requests || f.security.calls != assessments {
				t.Fatalf("unmanaged root reached acquisition or security before target selection: %s", out)
			}
			pmInstallEqual(t, "terminal refusal preserves active files and metadata", before, pmExistingSnapshot(t, f, project))
		})
	}
}

func pmExistingMalformedIntent(t *testing.T, f pmInstallJourney, project string, origin *pmInstallOrigin) {
	t.Helper()
	root := f.namespace(project)
	if project != "" {
		root = project
	}
	manifest := filepath.Join(root, "uap.toml")
	if _, err := os.Lstat(manifest); !os.IsNotExist(err) {
		t.Fatalf("malformed-intent fixture requires an absent manifest: %v", err)
	}
	projectJourneyWrite(t, manifest, "schema_version = [\n")
	before := pmExistingSnapshot(t, f, project)
	requests, assessments := origin.count(), f.security.calls
	out, err := f.execute(t, project, "", false, "add", origin.url(), "--target", "cursor", "--yes", "--format", "json")
	var foreign *environment.ForeignBindingError
	if err == nil || errors.As(err, &foreign) || !strings.Contains(err.Error(), manifest) {
		t.Fatalf("malformed intent lost precedence over existing foreign binding: %v\n%s", err, out)
	}
	if origin.count() != requests || f.security.calls != assessments {
		t.Fatal("malformed intent reached acquisition or security assessment")
	}
	pmInstallEqual(t, "malformed intent refusal preserves active files and metadata", before, pmExistingSnapshot(t, f, project))
	if err := os.Remove(manifest); err != nil {
		t.Fatal(err)
	}
}

func pmInstallForeignEnvironmentRefusal(t *testing.T) {
	o := newPMInstallOrigin(t)
	f := newPMInstallJourney(t, o)
	f.run(t, f.a, "add", o.url(), "--alias", "rich", "--target", "gemini")
	installation, _ := f.assertInstalled(t, f.a, "gemini", "v1", false)
	app, err := f.app.ForScope(domain.ScopeProject, f.a)
	if err != nil {
		t.Fatal(err)
	}
	state := f.state(t, f.a)
	if len(state.EnvironmentReceipts) != 1 {
		t.Fatalf("expected exactly one real receipt: %+v", state.EnvironmentReceipts)
	}
	// Retain the real installed identity and bytes; only its recorded owner is
	// the adversarial existing-state input for this refusal journey.
	const owner = "another-environment"
	state.EnvironmentReceipts[0].EnvironmentID = owner
	if err := app.StateStore.Save(state); err != nil {
		t.Fatal(err)
	}
	before := pmExistingSnapshot(t, f, f.a)
	for _, args := range [][]string{
		{"add", o.url(), "--alias", "rich", "--target", "cursor", "--yes", "--format", "json"},
		{"deps", "sync", "--locked", "--yes", "--format", "json"},
	} {
		out, err := f.execute(t, f.a, "", false, args...)
		if err == nil {
			t.Fatalf("foreign environment binding was adopted: %v\n%s", args, out)
		}
		pmExistingTypedConflict(t, err, o.url(), "foreign_environment", "rich", installation.InstallationID, "", owner)
		pmInstallEqual(t, "foreign receipt refusal preserves active files and metadata", before, pmExistingSnapshot(t, f, f.a))
	}
	out, err := f.execute(t, f.a, "", false, "add", o.url(), "--alias", "rich", "--target", "cursor")
	if err == nil || strings.Contains(out, "agentplugins remove ") || strings.Contains(out, "--no-save") {
		t.Fatalf("foreign receipt guidance suggested an ownership bypass: %v\n%s", err, out)
	}
	pmInstallEqual(t, "foreign receipt human refusal preserves metadata", before, pmExistingSnapshot(t, f, f.a))
}

func pmInstallDependencyOwnershipRefusal(t *testing.T) {
	child, root := newPMInstallOrigin(t), newPMInstallOrigin(t)
	root.set(t, map[string]string{
		"plugin.json":          `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"existing-state-root","version":"1.0.0"}`,
		"skills/root/SKILL.md": pmInstallSkill("root", "v1"),
		"uap.toml":             fmt.Sprintf("schema_version = 1\n[dependencies.child]\narchive = %q\nsha256 = %q\n", child.url(), child.digest()),
	})
	f := newPMInstallJourney(t, root)
	f.run(t, f.a, "add", child.url(), "--no-save", "--target", "gemini")
	installation, binding := f.assertInstalled(t, f.a, "gemini", "v1", false)
	before := pmExistingSnapshot(t, f, f.a)
	out, err := f.execute(t, f.a, "", false, "add", root.url(), "--alias", "parent", "--target", "cursor", "--yes", "--format", "json")
	if err == nil {
		t.Fatalf("saved root adopted an unmanaged dependency: %s", out)
	}
	pmExistingTypedConflict(t, err, child.url(), "unmanaged_binding", "install-rich", installation.InstallationID, "parent", "")
	pmInstallEqual(t, "dependency refusal preserves whole active environment", before, pmExistingSnapshot(t, f, f.a))
	out, err = f.execute(t, f.a, "", false, "add", root.url(), "--no-save", "--target", "cursor", "--yes")
	if err == nil || !strings.Contains(err.Error(), "declares UAP requirements") {
		t.Fatalf("no-save discarded the required graph: %v\n%s", err, out)
	}
	pmInstallEqual(t, "no-save dependency refusal preserves whole environment", before, pmExistingSnapshot(t, f, f.a))
	if child.count() == 0 || root.count() == 0 {
		t.Fatal("dependency ownership journey did not acquire both real HTTPS archives")
	}
	// A different source with distinct component identities remains installable;
	// merely having an unmanaged package must not block the whole environment.
	gemini := pmInstallStableTree(t, f.native(f.a, "gemini"))
	managed := pmInstallStableTree(t, binding.TargetLocator)
	root.set(t, map[string]string{
		"plugin.json":          `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"existing-state-root","version":"1.0.0"}`,
		"skills/root/SKILL.md": pmInstallSkill("root", "v1"),
	})
	f.run(t, f.a, "add", root.url(), "--alias", "parent", "--target", "cursor", "--yes")
	after := f.state(t, f.a)
	if len(after.Installations) != 2 || len(after.EnvironmentReceipts) != 1 || after.EnvironmentReceipts[0].ClientID != "cursor" || after.EnvironmentReceipts[0].Alias != "parent" {
		t.Fatalf("unrelated saved source lost its own target or ownership: %+v", after)
	}
	found := false
	for _, retained := range after.Installations {
		if retained.InstallationID == installation.InstallationID {
			found = true
			pmInstallEqual(t, "unrelated unmanaged installation identity", installation, retained)
		}
	}
	if !found {
		t.Fatal("unrelated saved add removed the unmanaged installation")
	}
	pmInstallEqual(t, "unrelated unmanaged native bytes", gemini, pmInstallStableTree(t, f.native(f.a, "gemini")))
	pmInstallEqual(t, "unrelated unmanaged managed bytes", managed, pmInstallStableTree(t, binding.TargetLocator))
}

func pmExistingTypedConflict(t *testing.T, err error, source, reason, alias, installation, root, owner string) {
	t.Helper()
	var refusal *environment.ForeignBindingError
	if !errors.As(err, &refusal) {
		t.Fatalf("refusal lost its typed ownership cause: %v", err)
	}
	wantKey, keyErr := (environment.Requirement{Alias: "candidate", Archive: &environment.ArchiveSource{URL: source}}).SourceKey()
	if keyErr != nil {
		t.Fatal(keyErr)
	}
	if refusal.Alias != alias || refusal.SourceKey != wantKey || refusal.RootAlias != root {
		t.Fatalf("ownership conflict lost exact source or graph identity: %#v", refusal)
	}
	pmInstallEqual(t, "unselected conflicting binding", []environment.ForeignBinding{{InstallationID: installation, ClientID: "gemini", Reason: reason, OwnerEnvironmentID: owner}}, refusal.Bindings)
}

// Exercise run's actual failure-output boundary after setup through the catalog
// fixture. The known archive key must refuse without acquisition, scanner or
// native execution; no formatter is called directly by this test.
func pmExistingRunBoundary(t *testing.T, f pmInstallJourney, project, source, installation string) {
	t.Helper()
	t.Setenv("HOME", f.home)
	t.Setenv("USERPROFILE", f.home)
	t.Setenv("AGENTPLUGINS_HOME", f.cache)
	t.Setenv("GEMINI_CLI_HOME", f.home)
	t.Setenv("PATH", t.TempDir())
	oldArgs, oldOut, oldErr, oldTransport := os.Args, os.Stdout, os.Stderr, http.DefaultTransport
	defer func() { os.Args, os.Stdout, os.Stderr, http.DefaultTransport = oldArgs, oldOut, oldErr, oldTransport }()
	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer stdout.Close()
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()
	transport := &offlineSecurityTransport{}
	http.DefaultTransport = transport
	os.Stdout, os.Stderr = stdout, stderr
	os.Args = []string{"agentplugins", "add", source, "--target", "cursor", "--yes", "--format", "json"}
	if project != "" {
		os.Args = append(os.Args, "--scope", "project", "--project-root", project)
	} else {
		os.Args = append(os.Args, "--scope", "user")
	}
	err = run()
	pmExistingTypedConflict(t, err, source, "unmanaged_binding", "", installation, "", "")
	if len(transport.urls) != 0 {
		t.Fatalf("early refusal performed network operations: %v", transport.urls)
	}
	body, err := os.ReadFile(stdout.Name())
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		SchemaVersion int    `json:"schema_version"`
		Result        string `json:"result"`
		Data          struct {
			Error struct {
				Code      string `json:"code"`
				Retryable bool   `json:"retryable"`
				Conflicts []struct {
					Reason         string   `json:"reason"`
					Alias          string   `json:"alias"`
					SourceKey      string   `json:"source_key"`
					InstallationID string   `json:"installation_id"`
					ClientIDs      []string `json:"client_ids"`
				} `json:"conflicts"`
			} `json:"error"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatalf("run did not emit exactly one JSON document: %v\n%s", err, body)
	}
	failure := document.Data.Error
	if document.SchemaVersion != 1 || document.Result != "failure" || failure.Code != "FOREIGN_BINDING" || failure.Retryable || len(failure.Conflicts) != 1 {
		t.Fatalf("unexpected run ownership JSON envelope: %s", body)
	}
	wantKey, err := (environment.Requirement{Alias: "candidate", Archive: &environment.ArchiveSource{URL: source}}).SourceKey()
	if err != nil {
		t.Fatal(err)
	}
	conflict := failure.Conflicts[0]
	if conflict.Reason != "unmanaged_binding" || conflict.Alias != "" || conflict.InstallationID != installation || conflict.SourceKey != wantKey {
		t.Fatalf("run lost ownership conflict details: %s", body)
	}
	pmInstallEqual(t, "run conflict client IDs", []string{"gemini"}, conflict.ClientIDs)
}

type pmExistingFile struct {
	Bytes    string
	Mode     os.FileMode
	Modified time.Time
}

type pmExistingState struct {
	Trees map[string]map[string]string
	Files map[string]pmExistingFile
}

func pmExistingSnapshot(t *testing.T, f pmInstallJourney, project string) pmExistingState {
	t.Helper()
	snapshot := pmExistingState{Trees: f.activeSnapshot(t, project), Files: map[string]pmExistingFile{}}
	for root, entries := range snapshot.Trees {
		for name := range entries {
			path := filepath.Join(root, name)
			info, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			if !info.Mode().IsRegular() {
				continue
			}
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			snapshot.Files[path] = pmExistingFile{Bytes: string(body), Mode: info.Mode(), Modified: info.ModTime()}
		}
	}
	return snapshot
}
