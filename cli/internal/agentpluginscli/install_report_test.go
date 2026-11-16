package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func failedItem(alias, client string, err error) agentItem {
	outcome := environment.BindingOutcome{Status: environment.BindingFailed, Err: err}
	status, reason := outcomeStatusReason(outcome)
	_ = reason
	return agentItem{Alias: alias, ClientID: client, Status: status, failure: describeBindingFailure(outcome)}
}

func TestReportByAgentIsolatesFailuresAndGroupsSharedCauses(t *testing.T) {
	items := []agentItem{
		{Alias: "pdf", ClientID: "claude", Status: "completed"},
		failedItem("pdf", "codex", installer.ErrTargetFactsUnavailable),
		failedItem("docx", "codex", installer.ErrTargetFactsUnavailable),
		{Alias: "docx", ClientID: "claude", Status: "completed"},
		failedItem("pdf", "gemini", installer.ErrForeignSkillDestination),
	}
	reports := reportByAgent(items)
	if len(reports) != 3 || reports[0].Failed() || !reports[1].Failed() || !reports[2].Failed() {
		t.Fatalf("one agent's failure must not taint another: %+v", reports)
	}
	if len(reports[1].Problems) != 1 || len(reports[1].Problems[0].Items) != 2 {
		t.Fatalf("items with the same cause must be grouped: %+v", reports[1].Problems)
	}
	if got := retryTargetHint(failedAgents(reports)); got != "To retry only these agents, run the same command with --target codex,gemini" {
		t.Fatalf("retry flag %q", got)
	}
}

func TestRecoveryRequiredIsNeverOfferedForRetry(t *testing.T) {
	reports := reportByAgent([]agentItem{
		failedItem("pdf", "codex", installer.ErrRecoveryRequired),
		failedItem("pdf", "gemini", errors.New("native command failed")),
	})
	retryable := retryableAgents(reports)
	if len(retryable) != 1 || retryable[0].ClientID != "gemini" {
		t.Fatalf("an agent that needs recovery must not be retried blindly: %+v", retryable)
	}
}

func TestAttentionSummaryShowsPlainCauseAndLogPathButNoTechnicalDetail(t *testing.T) {
	secret := "exec: \"codex\": exit status 7 at /private/tmp/very/technical/path token=abc123456"
	reports := reportByAgent([]agentItem{
		{Alias: "pdf", ClientID: "claude", Status: "completed"},
		failedItem("pdf", "codex", errors.New(secret)),
	})
	var out bytes.Buffer
	if err := renderAgentAttention(&out, reports, "/logs/install-1.jsonl", retryTargetHint(reports)); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, leaked := range []string{"exit status 7", "/private/tmp/very", "abc123456"} {
		if strings.Contains(text, leaked) {
			t.Fatalf("technical detail %q reached the terminal:\n%s", leaked, text)
		}
	}
	for _, want := range []string{"Needs attention", "Codex", "did not complete", "--target codex", "Technical details: /logs/install-1.jsonl"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Claude") {
		t.Fatalf("a healthy agent must not appear under Needs attention:\n%s", text)
	}
	if reports[1].Problems[0].Failure.Detail != secret {
		t.Fatal("the full technical chain must be kept for the log")
	}
}

func TestRetryReviewCoverageOnlyAcceptsAlreadyReviewedDestinations(t *testing.T) {
	reviewed := []SkillsAction{
		{Alias: "pdf", ClientID: "claude", TargetPath: "/a", Action: "install"},
		{Alias: "pdf", ClientID: "codex", TargetPath: "/b", Action: "install"},
	}
	if !coveredByReview(reviewed, []SkillsAction{{Alias: "pdf", ClientID: "codex", TargetPath: "/b", Action: "update"}}) {
		t.Fatal("an install that became an update of the same destination is still reviewed")
	}
	if !coveredByReview(reviewed, []SkillsAction{{Alias: "x", ClientID: "other", TargetPath: "/z", Action: "unchanged"}}) {
		t.Fatal("an unchanged action does nothing and needs no review")
	}
	for _, widened := range []SkillsAction{
		{Alias: "pdf", ClientID: "codex", TargetPath: "/elsewhere", Action: "install"},
		{Alias: "docx", ClientID: "codex", TargetPath: "/b", Action: "install"},
		{Alias: "pdf", ClientID: "gemini", TargetPath: "/b", Action: "install"},
	} {
		if coveredByReview(reviewed, []SkillsAction{widened}) {
			t.Fatalf("a retry that changes alias, agent or destination must be re-confirmed: %+v", widened)
		}
	}
}

func TestSkippedAgentIsRetryableOnlyWhenItsStopCauseIs(t *testing.T) {
	stopped := func(cause error) *installFailure {
		return describeBindingFailure(environment.BindingOutcome{Status: environment.BindingNotAttempted, Err: cause})
	}
	if failure := stopped(installer.ErrRecoveryRequired); failure.Retryable || failure.Kind != failureSkipped {
		t.Fatalf("a run stopped by recovery must not offer a blind retry: %+v", failure)
	}
	if failure := stopped(context.Canceled); !failure.Retryable {
		t.Fatalf("a canceled run can be retried: %+v", failure)
	}
}

func TestBindingsJSONNamesEveryPackageAndAgent(t *testing.T) {
	outcome := &installOutcome{Items: []agentItem{
		{Alias: "pdf", ClientID: "claude", Status: "completed"},
		failedItem("pdf", "codex", installer.ErrTargetFactsUnavailable),
	}}
	got := outcome.bindingsJSON()
	if len(got) != 2 || got[0].Code != "" || got[1].Code != string(failureTargetUnavailable) || got[1].Alias != "pdf" || got[1].ClientID != "codex" || got[1].Status != "failed" {
		t.Fatalf("%+v", got)
	}
}

// A successful result keeps its established JSON shape; per-binding detail and
// the log path appear only when something failed.
func TestDependencyResultsJSONAddsBindingsOnlyOnFailure(t *testing.T) {
	render := func(outcome *installOutcome) map[string]any {
		cmd, out, _ := retryTestCommand(t)
		opts := &options{format: "json", scope: "user", installResult: outcome}
		if err := renderDependencyResults(cmd, opts, nil, nil, nil); err != nil {
			t.Fatal(err)
		}
		var data map[string]any
		if err := json.Unmarshal(out.Bytes(), &data); err != nil {
			t.Fatalf("%v: %s", err, out.String())
		}
		return data
	}
	ok := &installOutcome{Items: []agentItem{{Alias: "demo", ClientID: "claude", Status: "completed"}}}
	ok.Agents = reportByAgent(ok.Items)
	if _, present := render(ok)["bindings"]; present {
		t.Fatal("a successful result must keep its established shape")
	}
	failed := &installOutcome{LogFile: "/logs/x.jsonl", Items: []agentItem{failedItem("demo", "codex", errors.New("boom"))}}
	failed.Agents = reportByAgent(failed.Items)
	data := render(failed)
	if data["bindings"] == nil || data["log_file"] != "/logs/x.jsonl" {
		t.Fatalf("a failure must carry per-binding detail and the log path: %v", data)
	}
}
