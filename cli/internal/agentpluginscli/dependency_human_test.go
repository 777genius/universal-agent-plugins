package agentpluginscli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestSavedTargetNoticeCompactsOnlyNeutralMissingAgents(t *testing.T) {
	for _, count := range []int{0, 1, 15} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			var skipped []targetSkip
			for i := range count {
				skipped = append(skipped, targetSkip{Client: domain.ClientID(fmt.Sprintf("absent-%d", i)), Reason: undetectedAutomaticInstallReason})
			}
			if count > 0 {
				skipped = append(skipped, skipped[0]) // Same fact is not another agent.
			}
			skipped = append(skipped,
				targetSkip{Client: "claude", Reason: "client configuration could not be inspected safely; correct it and retry"},
				targetSkip{Client: "codex", Reason: "use --no-save"},
				targetSkip{Client: "kiro", Reason: "manual activation required"},
				targetSkip{Client: "cursor", Reason: undetectedAutomaticInstallReason + "; policy review required"})
			labels := humanSkippedTargetLabels(skipped)
			wantLines := 4
			if count > 0 {
				wantLines++
			}
			text := strings.Join(labels, "\n")
			if len(labels) != wantLines || strings.Contains(text, "absent-") {
				t.Fatalf("missing-agent notice still dominates: %q", text)
			}
			for _, warning := range []string{"could not be inspected safely", "use --no-save", "manual activation required", "policy review required"} {
				if !strings.Contains(text, warning) {
					t.Fatalf("actionable warning %q lost: %q", warning, text)
				}
			}
			if count > 0 && !strings.Contains(text, fmt.Sprintf("%d agent", count)) {
				t.Fatalf("wrong missing-agent count: %q", text)
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
	a.ManualActions = []string{"Claude has not connected this server; inspect its existing policy"}
	a.NextActions = []installer.NextAction{{Kind: "authenticate", Reason: "review existing authentication"}}
	items := outcomeAgentItems([]environment.BindingOutcome{
		{Binding: dependencyAction{Alias: "not-attempted", ClientID: "claude"}, Status: environment.BindingNotAttempted},
		{Binding: dependencyAction{Alias: "alpha", ClientID: "claude"}, Result: &a},
		{Binding: dependencyAction{Alias: "beta", ClientID: "claude"}, Result: &b},
		{Binding: dependencyAction{Alias: "alpha", ClientID: "codex"}, Result: &c},
	})
	// Reorder actual results: alias lookup must use paired identity, not position.
	results := []installer.Result{b, a, c}
	cmd, out, _ := retryTestCommand(t)
	opts := &options{format: "human", installResult: &installOutcome{Items: items}}
	if err := renderDependencyResults(cmd, opts, nil, results, nil); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"Install beta for Claude Code: completed", "Install alpha for Claude Code: completed", "Install alpha for OpenAI Codex: completed", "Manual: " + a.ManualActions[0], "Next: authenticate review existing authentication"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing paired result/guidance %q: %q", want, text)
		}
	}
	if strings.Count(text, "Installation: id-alpha") != 1 || strings.Count(text, "Installation: id-beta") != 1 || strings.Contains(text, "not-attempted") || strings.Contains(text, "Ready") {
		t.Fatalf("identity references or readiness changed: %q", text)
	}
	for _, metadata := range []*installOutcome{nil, {Items: append(items, agentItem{Alias: "ambiguous", ClientID: "claude", installationID: "id-alpha"})}} {
		if alias := dependencyResultAlias(metadata, dependencyResultDTO{InstallationID: "id-alpha", ClientID: "claude"}); alias != "id-alpha" {
			t.Fatalf("missing/ambiguous identity guessed a package: %q", alias)
		}
	}

	// The same results serialize identically with and without human metadata.
	opts.format = "json"
	out.Reset()
	if err := renderDependencyResults(cmd, opts, nil, results, nil); err != nil {
		t.Fatal(err)
	}
	withMetadata := bytes.Clone(out.Bytes())
	opts.installResult = nil
	out.Reset()
	if err := renderDependencyResults(cmd, opts, nil, results, nil); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(withMetadata, out.Bytes()) {
		t.Fatalf("human labels changed public JSON:\n%s\n%s", withMetadata, out.Bytes())
	}
}
