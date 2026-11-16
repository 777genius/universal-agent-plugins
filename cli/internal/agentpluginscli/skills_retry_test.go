package agentpluginscli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func retryTestCommand(t *testing.T) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	return cmd, &out, &errOut
}

func skillsLogFiles(t *testing.T, root string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(root, "logs", "install-*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func withInstallLogDir(app App) App {
	app.InstallLogDir = filepath.Join(app.DependenciesRoot, "logs")
	return app
}

// One agent failing must leave the other installed, explain the failure in
// plain words, keep the technical detail in the log and let the person retry
// just the failed agent.
func TestSkillsFailureIsIsolatedExplainedLoggedAndRetriedPerAgent(t *testing.T) {
	j, _, sha := skillJourney(t)
	ctx := context.Background()
	request := SkillsBatchRequest{Selections: []SkillsSelection{{Repository: "acme/tools", Commit: sha, Path: "skills/pdf", Name: "pdf", Selector: environment.Selector{Kind: environment.SelectorRef, Value: "main"}}}, TargetIDs: []string{"codex", "claude"}}
	plan, err := j.cli.app.PrepareSkills(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	// Codex becomes unavailable after review, as if it were uninstalled meanwhile.
	broken := plan.request.detected[domain.ClientCodex]
	broken.Status = domain.DetectionNotDetected
	plan.request.detected[domain.ClientCodex] = broken

	var offered []string
	confirms := 0
	app := withInstallLogDir(j.cli.app)
	app.Prompter = fakePrompter{
		selectFn: func(r prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
			for _, choice := range r.Choices {
				offered = append(offered, string(choice.ID))
			}
			return prompt.TargetSelectionResult{IDs: r.DefaultIDs}, nil
		},
		confirmFn: func() (prompt.ConfirmationResult, error) {
			confirms++
			return prompt.ConfirmationResult{Accepted: true}, nil
		},
	}
	cmd, out, errOut := retryTestCommand(t)
	if err := applySkillsWithRecovery(cmd, app, &options{format: "human", scope: "user"}, plan, true); err != nil {
		t.Fatalf("the retry should have completed the installation: %v\n%s", err, out.String())
	}

	// One failed agent is offered as a plain yes/no, never as a one-item list.
	if confirms != 1 || len(offered) != 0 {
		t.Fatalf("the single failed agent must be offered with one confirmation: confirms=%d offered=%v", confirms, offered)
	}
	text := out.String()
	for _, want := range []string{"Needs attention", "Codex", "could not be verified", "Skills installation succeeded"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in output:\n%s", want, text)
		}
	}
	if strings.Contains(text, "not detected; nothing was changed") {
		t.Fatalf("the raw technical error must stay out of the terminal:\n%s", text)
	}
	if !strings.Contains(errOut.String(), "Retrying 1 agent(s)") {
		t.Fatalf("retry was not announced:\n%s", errOut.String())
	}

	logs := skillsLogFiles(t, app.DependenciesRoot)
	if len(logs) != 1 {
		t.Fatalf("exactly the failed first attempt must be logged, got %v", logs)
	}
	body, err := os.ReadFile(logs[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"client":"codex"`, `"status":"failed"`, `"code":"TARGET_UNAVAILABLE"`, "is not detected"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("log lacks %q:\n%s", want, body)
		}
	}
	if !strings.Contains(text, "Technical details: "+logs[0]) {
		t.Fatalf("the log location must be shown:\n%s", text)
	}

	state, err := j.cli.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, client := range []string{"claude", "codex"} {
		savedJourneyBinding(t, state, client) // fails the test when the agent has no installed binding
	}
}

func TestSkillsFailureWithoutAPersonDoesNotPromptButStillExplains(t *testing.T) {
	j, _, sha := skillJourney(t)
	ctx := context.Background()
	request := SkillsBatchRequest{Selections: []SkillsSelection{{Repository: "acme/tools", Commit: sha, Path: "skills/pdf", Name: "pdf", Selector: environment.Selector{Kind: environment.SelectorRef, Value: "main"}}}, TargetIDs: []string{"codex", "claude"}}
	plan, err := j.cli.app.PrepareSkills(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	broken := plan.request.detected[domain.ClientCodex]
	broken.Status = domain.DetectionNotDetected
	plan.request.detected[domain.ClientCodex] = broken
	app := withInstallLogDir(j.cli.app)
	app.Prompter = fakePrompter{selectFn: func(prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
		t.Fatal("a non-interactive run must never prompt")
		return prompt.TargetSelectionResult{}, nil
	}}
	cmd, out, _ := retryTestCommand(t)
	if err := applySkillsWithRecovery(cmd, app, &options{format: "human", scope: "user"}, plan, false); err == nil {
		t.Fatal("a partial installation must still report an error")
	}
	for _, want := range []string{"partial", "Needs attention", "--target codex", "Technical details:"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q:\n%s", want, out.String())
		}
	}
}

func retryAppAndOptions(t *testing.T, offered *[]string) (App, *options) {
	t.Helper()
	app := App{Terminal: true, Prompter: fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
		*offered = append(*offered, "codex")
		return prompt.ConfirmationResult{Accepted: true}, nil
	}, selectFn: func(r prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
		for _, choice := range r.Choices {
			*offered = append(*offered, string(choice.ID))
		}
		return prompt.TargetSelectionResult{IDs: r.DefaultIDs}, nil
	}}}
	opts := &options{format: "human", installResult: &installOutcome{Agents: reportByAgent([]agentItem{
		failedItem("demo", "codex", errors.New("native command failed")),
		failedItem("demo", "gemini", installer.ErrRecoveryRequired),
		{Alias: "demo", ClientID: "claude", Status: "completed"},
	})}}
	return app, opts
}

func TestAddRetryRerunsOnlyRetryableFailedAgentsAndKeepsUnresolvedOnesFailing(t *testing.T) {
	var offered []string
	app, opts := retryAppAndOptions(t, &offered)
	cmd, _, _ := retryTestCommand(t)
	var rerunTarget string
	var carried []string
	boom := errors.New("1 of 3 client installations failed")
	err := retryFailedAgents(cmd, app, opts, boom, func(target string, unresolved []string) error {
		rerunTarget, carried = target, unresolved
		return nil
	})
	if rerunTarget != "codex" || strings.Join(offered, ",") != "codex" {
		t.Fatalf("only the retryable failed agent may be retried: target=%q offered=%v", rerunTarget, offered)
	}
	// Gemini needs recovery and was never retried: a green retry must not hide it.
	if err == nil || !strings.Contains(err.Error(), "Gemini") || len(carried) != 1 {
		t.Fatalf("an unresolved agent must keep the command failing: err=%v carried=%v", err, carried)
	}
}

func TestAddRetrySucceedsOnlyWhenNothingIsLeftUnresolved(t *testing.T) {
	var offered []string
	app, opts := retryAppAndOptions(t, &offered)
	opts.installResult = &installOutcome{Agents: reportByAgent([]agentItem{failedItem("demo", "codex", errors.New("native command failed"))})}
	cmd, _, _ := retryTestCommand(t)
	if err := retryFailedAgents(cmd, app, opts, errors.New("failed"), func(string, []string) error { return nil }); err != nil {
		t.Fatalf("every failed agent was retried and fixed: %v", err)
	}
}

func TestAddRetryNeverPromptsWithoutAPersonOrAfterTheLimit(t *testing.T) {
	for name, mutate := range map[string]func(*App, *options){
		"--yes":          func(_ *App, o *options) { o.yes = true },
		"not a terminal": func(a *App, _ *options) { a.Terminal = false },
		"json output":    func(_ *App, o *options) { o.format = "json" },
		"dry run":        func(_ *App, o *options) { o.dryRun = true },
		"retry limit":    func(_ *App, o *options) { o.retryAttempt = maxSkillsRetries },
	} {
		t.Run(name, func(t *testing.T) {
			var offered []string
			app, opts := retryAppAndOptions(t, &offered)
			mutate(&app, opts)
			cmd, _, _ := retryTestCommand(t)
			boom := errors.New("failed")
			err := retryFailedAgents(cmd, app, opts, boom, func(string, []string) error { t.Fatal("must not rerun"); return nil })
			if !errors.Is(err, boom) || len(offered) != 0 {
				t.Fatalf("err=%v offered=%v", err, offered)
			}
		})
	}
}

func TestAddRetryCancelKeepsTheOriginalError(t *testing.T) {
	var offered []string
	app, opts := retryAppAndOptions(t, &offered)
	app.Prompter = fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
		return prompt.ConfirmationResult{}, prompt.ErrPromptCanceled
	}}
	cmd, _, _ := retryTestCommand(t)
	boom := errors.New("failed")
	if err := retryFailedAgents(cmd, app, opts, boom, func(string, []string) error { t.Fatal("must not rerun"); return nil }); !errors.Is(err, boom) {
		t.Fatalf("declining the retry must keep the failure: %v", err)
	}
}

// A failed one-agent add must reach the same retry step as a group: it records
// a per-agent result, writes the technical log and offers a plain yes/no.
func TestSingleAgentAddFailureIsRecordedLoggedAndRetried(t *testing.T) {
	cmd, _, errOut := retryTestCommand(t)
	app := App{Terminal: true, InstallLogDir: filepath.Join(t.TempDir(), "logs")}
	var asked []string
	app.Prompter = fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
		asked = append(asked, "retry")
		return prompt.ConfirmationResult{Accepted: true}, nil
	}}
	opts := &options{format: "human"}
	boom := installer.ErrTargetFactsUnavailable
	app.recordSingleAdd(cmd, opts, domain.ClientCodex, "demo", boom)
	if opts.installResult == nil || len(opts.installResult.failedAgents()) != 1 || opts.installResult.LogFile == "" {
		t.Fatalf("failure not recorded: %+v", opts.installResult)
	}
	if !strings.Contains(errOut.String(), "Technical details: "+opts.installResult.LogFile) {
		t.Fatalf("log path not shown: %s", errOut.String())
	}
	var rerun string
	err := retryFailedAgents(cmd, app, opts, boom, func(target string, _ []string) error { rerun = target; return nil })
	if err != nil || rerun != "codex" || len(asked) != 1 {
		t.Fatalf("rerun=%q asked=%v err=%v", rerun, asked, err)
	}
	// Success records nothing.
	clean := &options{format: "human"}
	app.recordSingleAdd(cmd, clean, domain.ClientCodex, "demo", nil)
	if clean.installResult != nil {
		t.Fatal("a successful add must not create a failure record")
	}
}

func TestConvergeRetryRerunsEverythingOutstandingAndRespectsTheGate(t *testing.T) {
	var asked []string
	app := App{Terminal: true, Prompter: fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
		asked = append(asked, "retry")
		return prompt.ConfirmationResult{Accepted: true}, nil
	}}}
	opts := &options{format: "human", installResult: &installOutcome{Agents: reportByAgent([]agentItem{
		failedItem("demo", "codex", errors.New("native command failed")),
		{Alias: "demo", ClientID: "claude", Status: "completed"},
	})}}
	cmd, _, _ := retryTestCommand(t)
	reruns := 0
	if err := retryConverge(cmd, app, opts, errors.New("failed"), func() error { reruns++; return nil }); err != nil || reruns != 1 || len(asked) != 1 || opts.retryAttempt != 1 {
		t.Fatalf("reruns=%d asked=%v attempt=%d err=%v", reruns, asked, opts.retryAttempt, err)
	}
	boom := errors.New("failed")
	opts.yes = true
	if err := retryConverge(cmd, app, opts, boom, func() error { t.Fatal("--yes must never prompt"); return nil }); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
}

func TestRetryInstallsExactlyThePackageOfTheFirstAttempt(t *testing.T) {
	first := loadedPackage{}
	first.envelope.TreeDigest = "sha256:first"
	opts := &options{}
	if err := pinRetryPackage(opts, first); err != nil || opts.firstTreeDigest != "sha256:first" {
		t.Fatalf("first attempt must pin its package: %v %q", err, opts.firstTreeDigest)
	}
	retry := *opts
	if err := pinRetryPackage(&retry, first); err != nil {
		t.Fatalf("an unchanged package may be retried: %v", err)
	}
	changed := loadedPackage{}
	changed.envelope.TreeDigest = "sha256:edited"
	if err := pinRetryPackage(&retry, changed); err == nil {
		t.Fatal("a retry must refuse a package that changed since the first attempt")
	}
	if err := pinRetryPackage(&options{}, loadedPackage{}); err != nil {
		t.Fatalf("a package without a digest cannot be pinned: %v", err)
	}
}
