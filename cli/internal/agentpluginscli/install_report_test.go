package agentpluginscli

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
