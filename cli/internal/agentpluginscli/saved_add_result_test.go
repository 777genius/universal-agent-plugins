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
			if err := renderDependencyAppliedResults(cmd, opts, nil, applied, applyErr); err != nil {
				t.Fatal(err)
			}
			explanation := "The dependency list records requested targets, not successful installations."
			if strings.Contains(output.String(), explanation) != scenario.wantExplanation {
				t.Fatalf("misleading saved outcome: %q", output.String())
			}
			if scenario.wantExplanation && (!strings.Contains(output.String(), "agentplugins deps status") || !strings.Contains(output.String(), "agentplugins install --locked")) {
				t.Fatalf("missing actionable recovery: %q", output.String())
			}
			if scenario.wantExplanation && !strings.Contains(output.String(), "Use --scope user for both commands.") {
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
	if err := renderDependencyAppliedResults(cmd, opts, nil, applied, errors.New("partly failed")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Use --scope project and the same --project-root for both commands.") || strings.Contains(output.String(), "--scope user") {
		t.Fatalf("recovery lost project authority: %q", output.String())
	}
}
