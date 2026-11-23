package agentpluginscli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environmentapp"
)

func TestSavedPublicationNoticeUsesObservedEffects(t *testing.T) {
	for _, test := range []struct {
		name    string
		applied environmentapp.Result
		want    string
	}{
		{name: "no publication", want: ""},
		{name: "changed or unreadable", applied: environmentapp.Result{Published: true}, want: "intent may have changed"},
		{name: "binding failure", applied: environmentapp.Result{Published: true, Outcomes: []environment.BindingOutcome{{Status: environment.BindingFailed}}}, want: "requested targets, not successful installations"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			cmd := &cobra.Command{Use: "add"}
			cmd.SetOut(&out)
			request := dependencySyncRequest{intent: environmentapp.Intent{ManifestPath: "/selected project/uap.toml", LockPath: "/selected project/uap.lock"}}
			opts := &options{format: "human", scope: "project", environmentOutput: map[string]any{"saved": true}}
			app := App{Scope: "project", ProjectRoot: "/selected project"}
			if err := renderDependencyAppliedResults(cmd, app, opts, request, test.applied, errors.New("failed")); err != nil {
				t.Fatal(err)
			}
			text := out.String()
			if test.want == "" && text != "" || test.want != "" && !strings.Contains(text, test.want) {
				t.Fatalf("notice %q, want %q", text, test.want)
			}
			if test.want != "" && (!strings.Contains(text, "Scope: project") || !strings.Contains(text, request.intent.ManifestPath) || !strings.Contains(text, request.intent.LockPath)) {
				t.Fatalf("selected environment context lost: %s", text)
			}
			for _, forbidden := range []string{"nothing changed", "intent saved", "install --locked", "deps sync --target"} {
				if strings.Contains(text, forbidden) {
					t.Fatalf("unsupported publication claim or retry: %s", text)
				}
			}
		})
	}
}

func TestRetryHintDistinguishesSourceInstallFromEnvironmentConvergence(t *testing.T) {
	reports := reportByAgent([]agentItem{failedItem("pdf", "chatgpt", errors.New("native command failed"))})
	for _, test := range []struct {
		verb string
		args []string
		want string
	}{
		{verb: "add", args: []string{"source"}, want: "--target chatgptweb"},
		{verb: "install", args: []string{"source"}, want: "--target chatgptweb"},
		{verb: "install", want: "complete saved environment"},
		{verb: "sync", want: "complete saved environment"},
	} {
		cmd := &cobra.Command{Use: test.verb}
		if err := cmd.ParseFlags(test.args); err != nil {
			t.Fatal(err)
		}
		hint := retryHintFor(cmd, reports)
		if !strings.Contains(hint, test.want) {
			t.Fatalf("%s %v: %q lacks %q", test.verb, test.args, hint, test.want)
		}
		if len(test.args) == 0 && strings.Contains(hint, "--target") {
			t.Fatalf("environment convergence acquired unsupported target flag: %s", hint)
		}
	}
}

func TestSavedAddPartialResultExplainsOnlyPublishedBindingOutcomes(t *testing.T) {
	for _, scenario := range []struct {
		name                          string
		saved, outcomes, failed, json bool
		wantExplanation               bool
	}{
		{name: "saved-partial", saved: true, outcomes: true, failed: true, wantExplanation: true},
		{name: "publication-refused", saved: true, failed: true},
		{name: "published-without-binding-outcomes", saved: true, failed: true},
		{name: "ordinary-deps-partial", outcomes: true, failed: true},
		{name: "saved-completed", saved: true, outcomes: true},
		{name: "saved-json-partial", saved: true, outcomes: true, failed: true, json: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			var output bytes.Buffer
			cmd := &cobra.Command{Use: "add"}
			cmd.SetOut(&output)
			opts := &options{format: "human", scope: "user"}
			if scenario.saved {
				opts.environmentOutput = map[string]any{"saved": true}
			}
			if scenario.json {
				opts.format = "json"
			}
			applied := environmentapp.Result{}
			if scenario.outcomes {
				applied.Outcomes = []environment.BindingOutcome{{Binding: dependencyAction{Alias: "demo", ClientID: "codex"}, Status: environment.BindingFailed, Err: errors.New("client failed")}}
				applied.Published = true
			}
			if scenario.name == "published-without-binding-outcomes" {
				applied.Published = true
			}
			var applyErr error
			if scenario.failed {
				applyErr = errors.New("apply refused or partly failed")
			}
			if err := renderDependencyAppliedResults(cmd, App{Scope: "user"}, opts, dependencySyncRequest{}, applied, applyErr); err != nil {
				t.Fatal(err)
			}
			explanation := "The dependency list records requested targets, not successful installations."
			if strings.Contains(output.String(), explanation) != scenario.wantExplanation {
				t.Fatalf("misleading saved outcome: %q", output.String())
			}
			if scenario.wantExplanation && (!strings.Contains(output.String(), "per-agent results") || !strings.Contains(output.String(), "resolve their next actions")) {
				t.Fatalf("missing actionable recovery: %q", output.String())
			}
			if scenario.wantExplanation && !strings.Contains(output.String(), "Scope: user") {
				t.Fatalf("recovery lost user scope: %q", output.String())
			}
			if scenario.json && !json.Valid(output.Bytes()) {
				t.Fatalf("invalid JSON: %q", output.String())
			}
		})
	}
}

func TestSavedAddPartialResultPreservesProjectScopeGuidance(t *testing.T) {
	var output bytes.Buffer
	cmd := &cobra.Command{Use: "add"}
	cmd.SetOut(&output)
	opts := &options{format: "human", scope: "project", projectRoot: t.TempDir(), environmentOutput: map[string]any{"saved": true}}
	applied := environmentapp.Result{Outcomes: []environment.BindingOutcome{{Binding: dependencyAction{Alias: "demo", ClientID: "codex"}, Status: environment.BindingFailed, Err: errors.New("client failed")}}}
	if err := renderDependencyAppliedResults(cmd, App{Scope: "project", ProjectRoot: opts.projectRoot}, opts, dependencySyncRequest{}, applied, errors.New("partly failed")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Scope: project") || strings.Contains(output.String(), "--scope user") {
		t.Fatalf("recovery lost project authority: %q", output.String())
	}
}
