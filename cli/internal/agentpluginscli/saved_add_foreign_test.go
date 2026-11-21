package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
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
	var out bytes.Buffer
	cmd := &cobra.Command{Use: "add"}
	cmd.SetOut(&out)
	opts := &options{format: "json", environmentOutput: map[string]any{"saved": true}}
	if err := renderDependencyResults(cmd, opts, nil, nil, refusal); err != nil {
		t.Fatal(err)
	}
	if ShouldWriteAgentFailure(refusal) {
		t.Fatal("late refusal would cause a second machine envelope")
	}
	decoder := json.NewDecoder(&out)
	var document map[string]any
	if err := decoder.Decode(&document); err != nil || document["result"] != "failure" {
		t.Fatalf("missing original failure result: %v %+v", err, document)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		t.Fatalf("more than one document: %v", err)
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
	snapshot := environmentSnapshot{Lock: environment.Lock{Packages: []environment.LockedPackage{{Alias: "demo", SourceKey: key}}}}
	for _, test := range []struct {
		name, alias, key string
		want             bool
	}{
		{name: "saved selector", key: key},
		{name: "explicit alias", alias: "chosen", key: "github:acme/other:"},
		{name: "unknown identity"},
		{name: "new canonical source", key: "directory:acme/other", want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := canPreviewSavedAddOwnership(snapshot, test.alias, test.key); got != test.want {
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
