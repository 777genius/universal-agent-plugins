package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestInstallYesDoesNotPromptToRetryDeclaredFailure(t *testing.T) {
	client := fixtureClient(t, domain.ClientCodex)
	client.ExecutablePath = filepath.Join(t.TempDir(), "codex")
	runner := &cliRunOnlyRunner{}
	commits := 0
	fixture, _ := newDependencyHarness(t, "", []domain.DetectedClient{client}, installer.Config{
		UserHome: t.TempDir(), Runner: runner,
		OnCommittedBinding: func(context.Context, installer.BindingFacts) error {
			commits++
			return installer.ErrTargetFactsUnavailable
		},
	})
	// Seed the normal immutable store and lock without acquiring or executing a source.
	pin := seedDependencyCheckPackage(t, fixture, "demo", "")
	writeDependencyCheckAgreement(t, fixture, []environment.LockedPackage{pin}, "demo")
	fixture.app.SourceAcquirer = dependencyCheckForbiddenEffects{}
	fixture.app.DirectoryClient = dependencyCheckForbiddenEffects{}
	manifest, lock := fixture.app.dependencyPaths()
	beforeManifest, beforeLock := dependencyRead(t, manifest), dependencyRead(t, lock)
	prompts := 0
	fixture.app.Prompter = fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
		prompts++
		return prompt.ConfirmationResult{Accepted: false}, nil
	}}
	var stdout, stderr bytes.Buffer
	fixture.app.Terminal = true
	fixture.app.Output, fixture.app.ErrorOutput = &stdout, &stderr
	command := NewRoot(fixture.app)
	command.SetArgs([]string{"install", "--scope", "user", "--yes", "--format", "human"})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := command.ExecuteContext(ctx)
	if !errors.Is(err, installer.ErrTargetFactsUnavailable) || commits != 1 {
		t.Fatalf("declared install did not reach its original failed handoff: commits=%d err=%v\n%s\n%s", commits, err, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "Needs attention") || !describeSyncFailure(err).Retryable {
		t.Fatalf("the actual failed binding must remain retryable and reported: %v\n%s", err, stdout.String())
	}
	if prompts != 0 || runner.calls != 0 || strings.Contains(stderr.String(), "Retrying") {
		t.Fatalf("install --yes must return the failure without prompting or starting another effect: prompts=%d native=%d stderr=%q", prompts, runner.calls, stderr.String())
	}
	if dependencyRead(t, manifest) != beforeManifest || dependencyRead(t, lock) != beforeLock {
		t.Fatal("failed install changed its reviewed manifest or package pin")
	}
}

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

func TestReportByAgentKeepsDistinctUpdateGuidance(t *testing.T) {
	for _, retainedFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("retained-first=%t", retainedFirst), func(t *testing.T) {
			active := failedItem("active", "gemini", fmt.Errorf("private-active: %w", installer.ErrUpdateRequired))
			retained := failedItem("retained-a", "gemini", retainedRevisionTestCause())
			items := []agentItem{active, retained,
				failedItem("retained-b", "gemini", fmt.Errorf("second private cause: %w", retainedRevisionTestCause()))}
			if retainedFirst {
				items[0], items[1] = items[1], items[0]
			}
			reports := reportByAgent(items)
			if len(reports) != 1 || len(reports[0].Problems) != 2 || reports[0].Retryable() {
				t.Fatalf("different update explanations must remain separate: %+v", reports)
			}
			var out bytes.Buffer
			if err := renderAgentAttention(&out, reports, "", retryHintFor(&cobra.Command{Use: "add"}, reports)); err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{
				"It is already installed at a different version. (active)\n  Next: Run `agentplugins update` for it, then try again.",
				retainedRevisionSummary + " (retained-a, retained-b)\n  Next: " + retainedRevisionGuidance,
			} {
				if !strings.Contains(out.String(), want) {
					t.Fatalf("guidance was assigned to the wrong package; missing %q:\n%s", want, out.String())
				}
			}
			for _, forbidden := range []string{"private", "second", "--target"} {
				if strings.Contains(out.String(), forbidden) {
					t.Fatalf("private detail or impossible retry reached attention output: %s", out.String())
				}
			}
			bindings := (&installOutcome{Items: items, Agents: reports}).bindingsJSON()
			if len(bindings) != len(items) {
				t.Fatalf("public bindings collapsed with the presentation: %+v", bindings)
			}
			for index, binding := range bindings {
				if binding.Alias != items[index].Alias || binding.ClientID != "gemini" || binding.Code != "UPDATE_REQUIRED" || binding.Status != "failed" {
					t.Fatalf("existing per-binding JSON contract changed: %+v", binding)
				}
			}
		})
	}
}

func TestReportByAgentKeepsSkippedRetryPolicy(t *testing.T) {
	for _, recoveryFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("recovery-first=%t", recoveryFirst), func(t *testing.T) {
			items := []agentItem{
				{Alias: "canceled", ClientID: "gemini", Status: "not_attempted", failure: describeBindingFailure(environment.BindingOutcome{Status: environment.BindingNotAttempted, Err: context.Canceled})},
				{Alias: "recovery", ClientID: "gemini", Status: "not_attempted", failure: describeBindingFailure(environment.BindingOutcome{Status: environment.BindingNotAttempted, Err: installer.ErrRecoveryRequired})},
			}
			if recoveryFirst {
				items[0], items[1] = items[1], items[0]
			}
			reports := reportByAgent(items)
			if len(reports) != 1 || len(reports[0].Problems) != 2 || reports[0].Retryable() || retryTargetHint(reports) != "" {
				t.Fatalf("grouping lost a skipped item's non-retryable cause: %+v", reports)
			}
		})
	}
}

func TestRecoveryRequiredIsNeverOfferedForRetry(t *testing.T) {
	reports := reportByAgent([]agentItem{
		failedItem("pdf", "codex", installer.ErrRecoveryRequired),
		failedItem("pdf", "gemini", errors.New("native command failed")),
	})
	retryable := retryableAgents(reports)
	if len(retryable) != 0 {
		t.Fatalf("recovery must block retries throughout the selected namespace: %+v", retryable)
	}
}

func TestRecoveryAttentionKeepsPerAgentFactsWithoutRetryTargets(t *testing.T) {
	reports := reportByAgent([]agentItem{
		failedItem("pdf", "codex", fs.ErrPermission),
		failedItem("pdf", "claude", installer.ErrRecoveryRequired),
	})
	var out bytes.Buffer
	if err := renderAgentAttention(&out, reports, "", retryHintFor(&cobra.Command{Use: "add"}, reports)); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"Codex", "Claude", "permission", "recovery", "agentplugins doctor"} {
		if !strings.Contains(text, want) {
			t.Fatalf("lost per-agent fact %q: %s", want, text)
		}
	}
	if strings.Contains(text, "--target") || strings.Contains(text, "To retry only") {
		t.Fatalf("recovery permits a misleading ordinary retry: %s", text)
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

func retainedRevisionTestCause() error {
	return fmt.Errorf("preflight private-retained-secret: %w", &domain.TypedError{
		Text: "private-retained-secret: run update separately",
		Kind: installer.ErrUpdateRequired, Cause: domain.ErrRetainedRevisionChange,
	})
}

func TestRetainedRevisionFailureGuidanceKeepsInternalCause(t *testing.T) {
	cause := retainedRevisionTestCause()
	for name, failure := range map[string]*installFailure{
		"binding": describeBindingFailure(environment.BindingOutcome{Status: environment.BindingFailed, Err: cause}),
		"sync":    describeSyncFailure(cause),
	} {
		t.Run(name, func(t *testing.T) {
			if failure.Kind != failureUpdateRequired || failure.Retryable || failure.Detail != cause.Error() {
				t.Fatalf("retained refusal lost its code, stop policy or internal detail: %+v", failure)
			}
			public := failure.Summary + " " + failure.Next
			for _, want := range []string{"no installed targets", "not supported", "recorded revision is kept"} {
				if !strings.Contains(public, want) {
					t.Fatalf("missing retained-state fact %q: %s", want, public)
				}
			}
			assertNoRetainedRevisionRetry(t, public)
		})
	}
	var out bytes.Buffer
	reports := reportByAgent([]agentItem{failedItem("demo", "gemini", cause)})
	if err := renderAgentAttention(&out, reports, "", retryHintFor(&cobra.Command{Use: "add"}, reports)); err != nil {
		t.Fatal(err)
	}
	assertNoRetainedRevisionRetry(t, out.String())
}

func assertNoRetainedRevisionRetry(t *testing.T, public string) {
	t.Helper()
	for _, forbidden := range []string{"private-retained-secret", "agentplugins update", "run update", "use update", "retry", "--target"} {
		if strings.Contains(strings.ToLower(public), forbidden) {
			t.Fatalf("unsafe retained-state advice %q: %s", forbidden, public)
		}
	}
}

func TestRetainedRevisionSkippedFailureKeepsSkippedSummary(t *testing.T) {
	failure := describeBindingFailure(environment.BindingOutcome{
		Status: environment.BindingNotAttempted, Err: retainedRevisionTestCause(),
	})
	if failure.Kind != failureSkipped || failure.Retryable || failure.Summary != "Not attempted because an earlier problem stopped the installation." {
		t.Fatalf("retained refusal changed skipped status or offered retry: %+v", failure)
	}
	if !strings.Contains(failure.Next, "not supported") || !strings.Contains(failure.Next, "recorded revision is kept") {
		t.Fatalf("skipped binding lost the actual stop cause: %+v", failure)
	}
	assertNoRetainedRevisionRetry(t, failure.Summary+" "+failure.Next)
}

func TestRetainedRevisionFailurePreservesRecoveryPriority(t *testing.T) {
	cause := errors.Join(installer.ErrRecoveryRequired, retainedRevisionTestCause())
	for name, failure := range map[string]*installFailure{
		"binding": describeBindingFailure(environment.BindingOutcome{Status: environment.BindingFailed, Err: cause}),
		"sync":    describeSyncFailure(cause),
	} {
		t.Run(name, func(t *testing.T) {
			if failure.Kind != failureRecovery || failure.Retryable || !strings.Contains(failure.Summary, "recovery") || !strings.Contains(failure.Next, "agentplugins doctor") {
				t.Fatalf("retained guidance overrode recovery: %+v", failure)
			}
			if public := failure.Summary + failure.Next; strings.Contains(public, "private-retained-secret") || strings.Contains(public, "empty installation") || strings.Contains(public, "agentplugins update") {
				t.Fatalf("recovery mixed in unsafe or lower-priority guidance: %s", public)
			}
		})
	}
}

func TestActiveRevisionFailureKeepsUpdateGuidance(t *testing.T) {
	for _, failure := range []*installFailure{
		describeBindingFailure(environment.BindingOutcome{Status: environment.BindingFailed, Err: installer.ErrUpdateRequired}),
		describeSyncFailure(installer.ErrUpdateRequired),
	} {
		if failure.Kind != failureUpdateRequired || failure.Retryable || !strings.Contains(failure.Next, "agentplugins update") || strings.Contains(failure.Summary, "no installed targets") {
			t.Fatalf("ordinary active revision lost its existing update advice: %+v", failure)
		}
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
