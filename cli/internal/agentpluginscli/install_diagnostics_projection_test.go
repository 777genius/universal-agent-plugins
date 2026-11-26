package agentpluginscli

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestMissingLogProjectsEveryErrorBearingHumanField(t *testing.T) {
	for _, identity := range []string{"paired", "missing", "ambiguous", "completed native failure"} {
		t.Run(identity, func(t *testing.T) {
			marker := "opaque-native-result-732198"
			failed := installer.Result{InstallationID: "failed-id", Outcome: installer.OutcomeIncomplete,
				Client: installer.ClientResult{ClientID: "codex"}, Reason: marker,
				NextActions: []installer.NextAction{{Kind: "activate", Reason: marker}, {Kind: "recover", Reason: marker}}, ManualActions: []string{marker}}
			if identity == "missing" {
				failed.InstallationID, failed.Client.ClientID = "", ""
			}
			if identity == "completed native failure" {
				failed.Outcome = installer.OutcomeCompleted
			}
			healthy := installer.Result{InstallationID: "healthy-id", Outcome: installer.OutcomeCompleted,
				Client: installer.ClientResult{ClientID: "claude"}, Reason: "Healthy binding retained",
				NextActions: []installer.NextAction{{Kind: "activate", Reason: "Keep the healthy guidance"}}, ManualActions: []string{"Keep the healthy manual action"}}
			outcomes := []environment.BindingOutcome{
				{Binding: dependencyAction{Alias: "broken", ClientID: "codex"}, Status: environment.BindingFailed, Result: &failed, Err: installer.ErrTargetFactsUnavailable},
				{Binding: dependencyAction{Alias: "healthy", ClientID: "claude"}, Status: environment.BindingApplied, Result: &healthy},
			}
			items := outcomeAgentItems(outcomes)
			if identity == "ambiguous" {
				duplicate := items[0]
				duplicate.Alias, duplicate.failure = "other", describeSyncFailure(installer.ErrRecoveryRequired)
				items = append(items, duplicate)
			}
			opts := &options{format: "human", installResult: &installOutcome{Agents: reportByAgent(items), Items: items}}
			results := []installer.Result{failed, healthy}
			before, _ := json.Marshal(results)
			cmd, output, _ := retryTestCommand(t)
			if err := renderDependencyResults(cmd, opts, nil, results, installer.ErrTargetFactsUnavailable); err != nil {
				t.Fatal(err)
			}
			text := output.String()
			if strings.Contains(text, marker) || !strings.Contains(text, "Keep the healthy guidance") || !strings.Contains(text, "Keep the healthy manual action") {
				t.Fatalf("failed next/manual text leaked or healthy instructions disappeared: %q", text)
			}
			if identity == "completed native failure" && strings.Contains(text, "broken for OpenAI Codex: completed") {
				t.Fatal("failed binding borrowed a successful native status in the human view")
			}
			after, _ := json.Marshal(results)
			if !bytes.Equal(before, after) {
				t.Fatal("human presentation changed original native facts")
			}
			opts.format = "json"
			output.Reset()
			if err := renderDependencyResults(cmd, opts, nil, results, installer.ErrTargetFactsUnavailable); err != nil {
				t.Fatal(err)
			}
			if strings.Count(output.String(), marker) < 4 {
				t.Fatal("human projection changed original JSON reason/next/manual fields")
			}
		})
	}
}

func TestCommittedHandoffReasonRemainsPrivateWhenNoLogExists(t *testing.T) {
	for _, format := range []string{"human", "json"} {
		t.Run(format, func(t *testing.T) {
			marker := "opaque-committed-handoff-198237"
			cause := errors.Join(errors.New(marker), installer.ErrTargetFactsUnavailable)
			fixture, runner, commits := diagnosticDependencyFixture(t, cause)
			output, stderr, err := fixture.execute(false, "install", "--scope", "user", "--yes", "--format", format)
			if !errors.Is(err, cause) || *commits != 1 || runner.calls != 0 {
				t.Fatalf("committed handoff was replayed or lost: commits=%d native=%d err=%v", *commits, runner.calls, err)
			}
			if format == "human" {
				if strings.Contains(output+stderr+err.Error(), marker) {
					t.Fatalf("committed native next-action reason leaked through human output: %s\n%s\n%v", output, stderr, err)
				}
			} else {
				control := decodeOneDiagnosticEnvelope(t, output)
				results, ok := control["results"].([]any)
				if !ok || len(results) != 1 {
					t.Fatalf("committed handoff lost its native JSON result: %s", output)
				}
				row := results[0].(map[string]any)
				actions, ok := row["next_actions"].([]any)
				if row["reason"] != "target_facts_unavailable" || !ok || len(actions) != 1 {
					t.Fatalf("JSON changed the SDK's classified handoff facts: %s", output)
				}
				next := actions[0].(map[string]any)
				if next["kind"] != "reprepare" || next["reason"] != "target_facts_unavailable" {
					t.Fatalf("JSON changed the SDK's classified next action: %v", next)
				}
			}
		})
	}
}

func TestHealthyMissingIdentityIsNotReclassifiedWithoutFailures(t *testing.T) {
	result := dependencyResultDTO{Outcome: "completed", Reason: "Healthy result", ManualActions: []string{"Keep this action"}}
	items := []agentItem{{Alias: "healthy", ClientID: "claude", Status: "completed"}}
	opts := &options{installResult: &installOutcome{Agents: reportByAgent(items), Items: items}}
	if displayed := dependencyHumanResult(opts, result); !reflect.DeepEqual(displayed, result) {
		t.Fatal("success-only projection invented a failure")
	}
}
