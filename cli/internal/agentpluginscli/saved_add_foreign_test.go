package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func foreignBindingFixture() *environment.ForeignBindingError {
	return &environment.ForeignBindingError{Alias: "child", SourceKey: "github:acme/child:", RootAlias: "root", Bindings: []environment.ForeignBinding{
		{InstallationID: "b", ClientID: "gemini", Reason: "foreign_environment", OwnerEnvironmentID: "project:other"},
		{InstallationID: "a", ClientID: "codex", Reason: "unmanaged_binding"},
		{InstallationID: "a", ClientID: "chatgpt", Reason: "unmanaged_binding"},
		{InstallationID: "a", ClientID: "codex", Reason: "unmanaged_binding"},
	}}
}

func TestEarlyForeignBindingMachineContract(t *testing.T) {
	refusal := foreignBindingFixture()
	err := earlyForeignBinding(App{}, refusal)
	var typed *environment.ForeignBindingError
	if !errors.As(err, &typed) || typed != refusal || !ShouldWriteAgentFailure(err) || ShouldWriteAgentFailure(refusal) {
		t.Fatal("only the early CLI wrapper must opt in, retaining the shared typed cause")
	}
	var out bytes.Buffer
	if err := WriteAgentFailure(&out, err); err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Result string `json:"result"`
		Data   struct {
			Error AgentFailure `json:"error"`
		} `json:"data"`
	}
	decoder := json.NewDecoder(&out)
	if err := decoder.Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		t.Fatalf("expected exactly one document: %v", err)
	}
	failure := envelope.Data.Error
	if envelope.Result != "failure" || failure.Code != "FOREIGN_BINDING" || failure.Retryable || len(failure.Conflicts) != 2 {
		t.Fatalf("unexpected contract: %+v", envelope)
	}
	first := failure.Conflicts[0]
	if first.InstallationID != "a" || first.Alias != "child" || first.SourceKey != refusal.SourceKey || first.RootAlias != "root" || first.OwnerEnvironmentID != "" || !reflect.DeepEqual(first.ClientIDs, []string{"chatgpt", "codex"}) {
		t.Fatalf("unexpected grouped conflict: %+v", first)
	}
	if failure.Conflicts[1].OwnerEnvironmentID != "project:other" || failure.Conflicts[1].Reason != "foreign_environment" {
		t.Fatalf("foreign owner lost: %+v", failure.Conflicts[1])
	}
}

func TestForeignBindingHumanGuidancePreservesScopeWithoutUnsafeCommand(t *testing.T) {
	err := earlyForeignBinding(App{Scope: domain.ScopeProject, ProjectRoot: "/project with 'quotes'"}, foreignBindingFixture())
	text := err.Error()
	for _, want := range []string{"root -> child", "another saved environment", "project with 'quotes'", "scope project"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q: %s", want, text)
		}
	}
	for _, forbidden := range []string{"Inspect: agentplugins", "--no-save", "agentplugins remove", "--purge-data", "--external-uninstalled"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("unsafe or unproved advice %q: %s", forbidden, text)
		}
	}
}

func TestLateForeignBindingKeepsSingleApplyResult(t *testing.T) {
	refusal := foreignBindingFixture()
	wantConflicts := []foreignBindingConflict{
		{Reason: "unmanaged_binding", Alias: "child", SourceKey: refusal.SourceKey, InstallationID: "a", ClientIDs: []string{"chatgpt", "codex"}, RootAlias: "root"},
		{Reason: "foreign_environment", Alias: "child", SourceKey: refusal.SourceKey, InstallationID: "b", ClientIDs: []string{"gemini"}, RootAlias: "root", OwnerEnvironmentID: "project:other"},
	}
	for _, scenario := range []struct {
		name  string
		saved bool
		err   error
	}{
		{name: "saved", saved: true, err: refusal},
		{name: "saved wrapped", saved: true, err: fmt.Errorf("private transport detail: %w", refusal)},
		{name: "deps", err: refusal},
		{name: "deps wrapped", err: fmt.Errorf("private transport detail: %w", refusal)},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			var out bytes.Buffer
			cmd := &cobra.Command{Use: "add"}
			cmd.SetOut(&out)
			opts := &options{format: "json"}
			if scenario.saved {
				opts.environmentOutput = map[string]any{"saved": true}
			}
			if err := renderDependencyResults(cmd, opts, nil, nil, scenario.err); err != nil {
				t.Fatal(err)
			}
			if ShouldWriteAgentFailure(scenario.err) || strings.Contains(out.String(), "private transport detail") {
				t.Fatalf("late refusal must keep one sanitized result: %s", out.String())
			}
			decoder := json.NewDecoder(&out)
			var document struct {
				Result string       `json:"result"`
				Error  AgentFailure `json:"error"`
				Data   struct {
					Error AgentFailure `json:"error"`
				} `json:"data"`
			}
			if err := decoder.Decode(&document); err != nil || document.Result != "failure" {
				t.Fatalf("missing original failure result: %v %+v", err, document)
			}
			failure := document.Error
			if scenario.saved {
				failure = document.Data.Error
			}
			if failure.Code != "FOREIGN_BINDING" || failure.Retryable || !reflect.DeepEqual(failure.Conflicts, wantConflicts) {
				t.Fatalf("late refusal lost ownership details: %+v", failure)
			}
			if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
				t.Fatalf("more than one document: %v", err)
			}
		})
	}
}

func TestCanceledSavedRootInspectionStopsBeforeStateAccess(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := inspectSavedAddRoot(ctx, App{}, &options{}, "github:acme/demo@"+strings.Repeat("a", 40))
	if !errors.Is(err, context.Canceled) || ShouldWriteAgentFailure(err) {
		t.Fatalf("cancellation was reclassified: %v", err)
	}
}

func TestSavedOwnershipPreviewDefersAliasAndSavedSelectorDecisions(t *testing.T) {
	key := "github:acme/demo:"
	snapshot := environmentSnapshot{
		Requirements: environment.Requirements{Dependencies: map[string]environment.Requirement{"claimed": {Alias: "claimed", Registry: &environment.RegistrySource{Distribution: "acme/existing", Selector: environment.Selector{Kind: environment.SelectorRelease, Value: "1"}}}}},
		Lock:         environment.Lock{Packages: []environment.LockedPackage{{Alias: "demo", SourceKey: key}}},
	}
	for _, test := range []struct {
		name, alias, derivedAlias, key string
		want                           bool
	}{
		{name: "saved selector", key: key},
		{name: "explicit alias", alias: "chosen", key: "github:acme/other:"},
		{name: "unknown identity"},
		{name: "new canonical source", key: "directory:acme/other", want: true},
		{name: "claimed derived alias", derivedAlias: "claimed", key: "directory:acme/other"},
		{name: "unclaimed derived alias", derivedAlias: "available", key: "directory:acme/other", want: true},
		{name: "same source derived alias", derivedAlias: "claimed", key: "directory:acme/existing", want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := canPreviewSavedAddOwnership(snapshot, test.alias, test.derivedAlias, test.key); got != test.want {
				t.Fatalf("preview=%v want=%v; resolver decisions must retain precedence", got, test.want)
			}
		})
	}
	// Deferral does not relax the same source's authoritative ownership rule.
	if err := environment.CheckSourceOwnership(snapshot.Lock.Packages[0], []environment.BindingObservation{{SourceKey: key, InstallationID: "existing", ClientID: "codex"}}); err == nil {
		t.Fatal("deferring the preview weakened the full ownership check")
	}
}

func TestMalformedIntentPrecedesSavedRootOwnershipAdvice(t *testing.T) {
	fixture, source := newDependencyHarness(t, t.TempDir(), nil, installer.Config{})
	manifest, _ := fixture.app.dependencyPaths()
	dependencyWrite(t, manifest, "schema_version = [invalid TOML")
	before, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	err = inspectSavedAddRoot(t.Context(), fixture.app, &options{}, "github:acme/demo@"+strings.Repeat("a", 40))
	var ownership *environment.ForeignBindingError
	if err == nil || errors.As(err, &ownership) || ShouldWriteAgentFailure(err) {
		t.Fatalf("malformed intent was replaced with ownership advice: %v", err)
	}
	after, readErr := os.ReadFile(manifest)
	if readErr != nil || !bytes.Equal(before, after) || source.downloads != 0 || source.lookups != 0 {
		t.Fatalf("refusal mutated intent or acquired source: %v", readErr)
	}
}

func TestSavedAddInputErrorsPrecedeOwnershipInspection(t *testing.T) {
	fixture, source := newDependencyHarness(t, t.TempDir(), nil, installer.Config{})
	manifest, _ := fixture.app.dependencyPaths()
	dependencyWrite(t, manifest, "schema_version = [invalid TOML")
	before, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, want string
		args       []string
	}{
		{name: "mixed manual target", args: []string{"--target", "chatgptweb,codex"}, want: "separately"},
		{name: "missing target", want: "requires --target"},
		{name: "activation attestation", args: []string{"--target", "codex,gemini", "--activation-complete"}, want: "exactly one target"},
		{name: "authentication attestation", args: []string{"--target", "codex,gemini", "--auth-complete"}, want: "exactly one target"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{"add", "github:acme/demo@" + strings.Repeat("a", 40), "--format", "json"}
			_, _, err := fixture.execute(false, append(args, tc.args...)...)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("known argument error was hidden by environment inspection: %v", err)
			}
			after, readErr := os.ReadFile(manifest)
			if readErr != nil || !bytes.Equal(before, after) || source.downloads != 0 || source.lookups != 0 {
				t.Fatalf("argument refusal mutated intent or acquired source: %v", readErr)
			}
		})
	}
}

// An unmanaged installation must remain unchanged after a saved-add refusal.
// Ownership advice names the client and inspection command without promising
// automatic ownership transfer.
func TestSavedAddNamesTheClientAndInspectionForAnUnmanagedInstallation(t *testing.T) {
	repo := dependencyRepo(t)
	f, _, _ := dependencySpecHarness(t, repo, dependencyClientSpec{id: domain.ClientCodex, executable: true})
	head := exec.CommandContext(t.Context(), "git", "rev-parse", "HEAD")
	head.Dir = repo
	rev, err := head.Output()
	if err != nil {
		t.Fatal(err)
	}
	source := githubIdentity("acme/demo", strings.TrimSpace(string(rev)), "")
	dependencyRun(t, f, "add", source, "--target", "codex", "--no-save", "--yes", "--format", "json")
	state, err := f.store.Load()
	if err != nil || len(state.Installations) != 1 || len(state.EnvironmentReceipts) != 0 {
		t.Fatalf("fixture must hold exactly one unmanaged installation: %+v %v", state, err)
	}
	id := state.Installations[0].InstallationID

	_, _, err = f.execute(false, "add", source, "--target", "codex", "--yes")
	if err == nil {
		t.Fatal("saved add adopted an unmanaged installation")
	}
	var foreign *environment.ForeignBindingError
	if !errors.As(err, &foreign) {
		t.Fatalf("typed cause lost: %v", err)
	}
	command := "agentplugins info " + id + " --scope user"
	for _, want := range []string{"existing package ownership", "Codex", "installation " + id, command, "cannot transfer ownership automatically"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("message lacks %q:\n%s", want, err)
		}
	}
	for _, forbidden := range []string{"agentplugins remove", "--no-save", "--external-uninstalled"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("unsafe ownership advice %q: %v", forbidden, err)
		}
	}
	stateAfter, loadErr := f.store.Load()
	if loadErr != nil || !reflect.DeepEqual(state, stateAfter) {
		t.Fatalf("ownership refusal changed the existing installation: %+v %v", stateAfter, loadErr)
	}

}

func TestEarlyForeignBindingLeavesOtherErrorsAlone(t *testing.T) {
	other := errors.New("drift: demo/codex")
	if got := earlyForeignBinding(App{}, other); !errors.Is(got, other) {
		t.Fatalf("unrelated error rewritten: %v", got)
	}
	if earlyForeignBinding(App{}, nil) != nil {
		t.Fatal("nil error invented")
	}
}
