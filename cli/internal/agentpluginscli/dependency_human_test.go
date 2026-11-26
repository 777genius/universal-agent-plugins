package agentpluginscli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestSavedTargetNoticeGroupsMissingAgentsAndKeepsActionableReasons(t *testing.T) {
	for _, count := range []int{0, 1, 15} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			var skipped []targetSkip
			for i := range count {
				skipped = append(skipped, targetSkip{Client: domain.ClientID(fmt.Sprintf("absent-%d", i)), Kind: prompt.SkipNotDetected})
			}
			if count > 0 {
				skipped = append(skipped, skipped[0]) // Same fact is not another agent.
			}
			skipped = append(skipped,
				targetSkip{Client: "claude", Reason: "client configuration could not be inspected safely; correct it and retry"},
				targetSkip{Client: "codex", Reason: "use --no-save"},
				targetSkip{Client: "kiro", Reason: "manual activation required"},
				targetSkip{Client: "cursor", Kind: prompt.SkipNotDetected, Reason: "not detected; policy review required"})
			var request prompt.TargetSelectionRequest
			app := App{Prompter: fakePrompter{selectFn: func(value prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
				request = value
				return prompt.TargetSelectionResult{IDs: value.DefaultIDs}, nil
			}}}
			cmd := &cobra.Command{}
			cmd.SetContext(t.Context())
			clients := []domain.DetectedClient{{ClientID: domain.ClientClaude}, {ClientID: domain.ClientCodex}}
			if _, _, err := promptTargetChoices(cmd, app, clients, skipped, clients); err != nil {
				t.Fatal(err)
			}
			groups := prompt.GroupSkipped(request.Skipped)
			wantLines := 4
			if count > 0 {
				wantLines++
			}
			text := prompt.SkippedClientsNotice(request.Skipped)
			if len(groups) != wantLines {
				t.Fatalf("missing-agent notice still dominates: %q", text)
			}
			for _, warning := range []string{"could not be inspected safely", "use --no-save", "manual activation required", "policy review required"} {
				if !strings.Contains(text, warning) {
					t.Fatalf("actionable warning %q lost: %q", warning, text)
				}
			}
			if count > 0 {
				var missing int
				for _, group := range groups {
					if group.Detail == "not found on this computer; install or configure it first" {
						missing += len(group.Names)
					}
				}
				if missing != count {
					t.Fatalf("wrong deduplicated missing-agent count: got %d, want %d", missing, count)
				}
			}
		})
	}
}

func TestDependencyHumanReviewKeepsNonNeutralReasons(t *testing.T) {
	for _, reason := range []string{"no matching environment-owned binding", "review existing policy\nkeep explicit trust disclosure", "no matching environment-owned binding; inspect ownership"} {
		t.Run(reason, func(t *testing.T) {
			line := humanDependencyPlanLine(dependencyAction{Alias: "context7", ClientID: "claude", Action: environment.ActionInstall, Reason: reason})
			if !strings.HasPrefix(line, "Install context7 for Claude Code") {
				t.Fatalf("selected package/agent/action missing: %q", line)
			}
			if reason == "no matching environment-owned binding" {
				if strings.Contains(line, reason) {
					t.Fatal("technical fresh-install reason obscures review")
				}
			} else if !strings.Contains(line, reason) {
				t.Fatalf("non-neutral review disclosure changed: %q", line)
			}
		})
	}
	line := humanDependencyPlanLine(dependencyAction{Alias: "context7\n\x1b", ClientID: "unknown\n\x1b", Action: environment.ActionInstall})
	if strings.ContainsAny(line, "\n\x1b") {
		t.Fatal("package/agent display text retained terminal controls")
	}
}

func TestDependencyHumanResultsUsePairedIdentityAndKeepGuidance(t *testing.T) {
	result := func(id, client string) installer.Result {
		return installer.Result{InstallationID: id, Operation: installer.OpInstall, Outcome: installer.OutcomeCompleted, Client: installer.ClientResult{ClientID: client}}
	}
	a, b, c := result("id-alpha", "claude"), result("id-beta", "claude"), result("id-alpha", "codex")
	b.Outcome, b.Reason = installer.OutcomeIncomplete, "client apply failed"
	applyErr := fmt.Errorf("beta client apply failed")
	a.ManualActions = []string{"Claude has not connected this server; inspect its existing policy"}
	a.NextActions = []installer.NextAction{{Kind: "authenticate", Reason: "review existing authentication"}}
	items := outcomeAgentItems([]environment.BindingOutcome{
		{Binding: dependencyAction{Alias: "not-attempted", ClientID: "claude"}, Status: environment.BindingNotAttempted},
		{Binding: dependencyAction{Alias: "alpha", ClientID: "claude"}, Status: environment.BindingApplied, Result: &a},
		{Binding: dependencyAction{Alias: "beta", ClientID: "claude"}, Status: environment.BindingFailed, Result: &b, Err: applyErr},
		{Binding: dependencyAction{Alias: "alpha", ClientID: "codex"}, Status: environment.BindingApplied, Result: &c},
	})
	// Reorder actual results: alias lookup must use paired identity, not position.
	results := []installer.Result{b, a, c}
	cmd, out, _ := retryTestCommand(t)
	opts := &options{format: "human", installResult: &installOutcome{Items: items}}
	if err := renderDependencyResults(cmd, opts, nil, results, applyErr); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"Install beta for Claude Code: incomplete The installation did not complete for this agent.", "Install alpha for Claude Code: completed", "Install alpha for OpenAI Codex: completed", "Installation: id-beta (beta)\n", "Installation: id-alpha (alpha)\n", "Manual: " + a.ManualActions[0], "Next: authenticate review existing authentication"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing paired result/guidance %q: %q", want, text)
		}
	}
	if strings.Count(text, "id-alpha") != 1 || strings.Count(text, "id-beta") != 1 || strings.Contains(text, "not-attempted") || strings.Contains(text, "Ready") {
		t.Fatalf("identity references or readiness changed: %q", text)
	}
	for _, metadata := range []*installOutcome{nil, {Items: append(items, agentItem{Alias: "ambiguous", ClientID: "claude", installationID: "id-alpha"})}} {
		if alias := dependencyResultAlias(metadata, dependencyResultDTO{InstallationID: "id-alpha", ClientID: "claude"}); alias != "id-alpha" {
			t.Fatalf("missing/ambiguous identity guessed a package: %q", alias)
		}
		if suffix := dependencyInstallationAliasSuffix(metadata, []dependencyResultDTO{{InstallationID: "id-alpha", ClientID: "claude"}}, "id-alpha"); suffix != "" {
			t.Fatalf("missing/ambiguous identity invented a footer label: %q", suffix)
		}
	}

	// The same results serialize identically with and without human metadata.
	opts.format = "json"
	out.Reset()
	if err := renderDependencyResults(cmd, opts, nil, results, applyErr); err != nil {
		t.Fatal(err)
	}
	withMetadata := bytes.Clone(out.Bytes())
	opts.installResult = nil
	out.Reset()
	if err := renderDependencyResults(cmd, opts, nil, results, applyErr); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(withMetadata, out.Bytes()) {
		t.Fatalf("human labels changed public JSON:\n%s\n%s", withMetadata, out.Bytes())
	}
}
