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
	app := withInstallLogDir(j.cli.app)
	app.Prompter = fakePrompter{
		selectFn: func(r prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
			for _, choice := range r.Choices {
				offered = append(offered, string(choice.ID))
			}
			return prompt.TargetSelectionResult{IDs: r.DefaultIDs}, nil
		},
		confirmFn: func() (prompt.ConfirmationResult, error) { return prompt.ConfirmationResult{Accepted: true}, nil },
	}
	cmd, out, errOut := retryTestCommand(t)
	app.Terminal, app.ErrorOutput = true, errOut
	if err := applySkillsWithRecovery(cmd, app, &options{format: "human", scope: "user", plain: true}, plan, true); err != nil {
		t.Fatalf("the retry should have completed the installation: %v\n%s", err, out.String())
	}

	if strings.Join(offered, ",") != "codex" {
		t.Fatalf("only the failed agent may be offered for retry, got %v", offered)
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
	initialProgress, retryProgress, announced := strings.Cut(errOut.String(), "Retrying 1 agent(s)")
	if !announced || !strings.Contains(initialProgress, "preparing") || !strings.Contains(initialProgress, "installing") ||
		!strings.Contains(retryProgress, "preparing") || !strings.Contains(retryProgress, "installing") {
		t.Fatalf("both attempts must expose live progress:\n%s", errOut.String())
	}
	if strings.Contains(retryProgress, "Claude Code") {
		t.Fatalf("retry progress included an already successful agent:\n%s", retryProgress)
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
	app := App{Terminal: true, Prompter: fakePrompter{selectFn: func(r prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
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
	app.Prompter = fakePrompter{selectFn: func(prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
		return prompt.TargetSelectionResult{}, prompt.ErrPromptCanceled
	}}
	cmd, _, _ := retryTestCommand(t)
	boom := errors.New("failed")
	if err := retryFailedAgents(cmd, app, opts, boom, func(string, []string) error { t.Fatal("must not rerun"); return nil }); !errors.Is(err, boom) {
		t.Fatalf("declining the retry must keep the failure: %v", err)
	}
}

func TestPrepareSkillsRetryPreservesPromptErrors(t *testing.T) {
	readFailure := errors.Join(errors.New("retry terminal read failed"), prompt.ErrPromptInputClosed)
	for name, test := range map[string]struct {
		app  App
		want error
	}{
		"terminal unavailable": {app: App{}, want: prompt.ErrPromptUnavailable},
		"selection read failed": {
			app: App{Prompter: fakePrompter{selectFn: func(prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
				return prompt.TargetSelectionResult{}, readFailure
			}}},
			want: prompt.ErrPromptInputClosed,
		},
	} {
		t.Run(name, func(t *testing.T) {
			cmd, _, _ := retryTestCommand(t)
			reports := reportByAgent([]agentItem{failedItem("pdf", "codex", installer.ErrTargetFactsUnavailable)})
			// No original plan is needed: an input failure must stop before preparation.
			next, err := prepareSkillsRetry(cmd, test.app, &options{format: "human"}, nil, nil, reports)
			if next != nil || !errors.Is(err, test.want) {
				t.Fatalf("retry input failure was lost: next=%v err=%v want=%v", next, err, test.want)
			}
		})
	}
}

func TestPrepareSkillsRetryEscapeDeclinesWithoutError(t *testing.T) {
	for _, canceled := range []error{prompt.ErrPromptCanceled, errors.Join(errors.New("wrapped cancellation"), prompt.ErrPromptCanceled)} {
		t.Run(canceled.Error(), func(t *testing.T) {
			app := App{Prompter: fakePrompter{selectFn: func(prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
				return prompt.TargetSelectionResult{}, canceled
			}}}
			cmd, _, _ := retryTestCommand(t)
			reports := reportByAgent([]agentItem{failedItem("pdf", "codex", installer.ErrTargetFactsUnavailable)})
			next, err := prepareSkillsRetry(cmd, app, &options{format: "human"}, nil, nil, reports)
			if next != nil || err != nil {
				t.Fatalf("Escape must decline without replacing the original installation failure: next=%v err=%v", next, err)
			}
		})
	}
}

func TestPrepareSkillsRetryRejectsInvalidSelection(t *testing.T) {
	for name, ids := range map[string][]domain.ClientID{
		"unoffered target": {domain.ClientClaude},
		"duplicate target": {domain.ClientCodex, domain.ClientCodex},
		"empty selection":  nil,
	} {
		t.Run(name, func(t *testing.T) {
			app := App{Prompter: fakePrompter{selectFn: func(prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
				return prompt.TargetSelectionResult{IDs: ids}, nil
			}}}
			cmd, _, _ := retryTestCommand(t)
			reports := reportByAgent([]agentItem{failedItem("pdf", "codex", installer.ErrTargetFactsUnavailable)})
			next, err := prepareSkillsRetry(cmd, app, &options{format: "human"}, nil, nil, reports)
			if next != nil || err == nil {
				t.Fatalf("invalid selection was treated as a declined retry: next=%v err=%v", next, err)
			}
		})
	}
}

func TestSkillsHumanFailureBoundaryPreservesTypedCauses(t *testing.T) {
	for _, sentinel := range []error{installer.ErrRecoveryRequired, context.Canceled} {
		t.Run(sentinel.Error(), func(t *testing.T) {
			native := errors.Join(errors.New("private-native-stderr-marker"), sentinel)
			outcome := environment.BindingOutcome{Status: environment.BindingFailed, Err: native}
			syncErr := &environment.SyncError{Outcomes: []environment.BindingOutcome{outcome}}
			items := []SkillsBatchItem{{failure: describeBindingFailure(outcome)}}
			err := humanSkillsApplyError("human", items, syncErr, "fixture\n-install.log")
			var observed *environment.SyncError
			if !errors.Is(err, sentinel) || !errors.Is(err, native) || !errors.As(err, &observed) || observed != syncErr {
				t.Fatalf("human presentation lost typed installer causes: %v", err)
			}
			text := ErrorText([]string{"skills", "add", "fixture", "--no-ui"}, &bytes.Buffer{}, err)
			if strings.Contains(text, "private-native-stderr-marker") || strings.ContainsAny(text, "\n\x1b") || !strings.Contains(text, "Technical details: fixture-install.log") {
				t.Fatalf("final stderr did not preserve the safe report boundary: %q", text)
			}
			if humanSkillsApplyError("json", items, syncErr, "log") != syncErr || humanSkillsApplyError("human", nil, syncErr, "log") != syncErr || humanSkillsApplyError("human", items, nil, "log") != nil {
				t.Fatal("presentation changed an unreported, JSON, or successful result")
			}
		})
	}
}

func TestSkillsRetryClosesPreparedPreviewWhenOutputFails(t *testing.T) {
	for _, phase := range []string{"announcement", "result"} {
		t.Run(phase, func(t *testing.T) {
			j, _, sha := skillJourney(t)
			request := SkillsBatchRequest{Selections: []SkillsSelection{{Repository: "acme/tools", Commit: sha, Path: "skills/pdf", Name: "pdf", Selector: environment.Selector{Kind: environment.SelectorRef, Value: "main"}}}, TargetIDs: []string{"codex", "claude"}}
			plan, err := j.cli.app.PrepareSkills(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			defer plan.Close()
			broken := plan.request.detected[domain.ClientCodex]
			broken.Status = domain.DetectionNotDetected
			plan.request.detected[domain.ClientCodex] = broken
			cmd, _, _ := retryTestCommand(t)
			app := withInstallLogDir(j.cli.app)
			app.Prompter = fakePrompter{selectFn: func(r prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
				return prompt.TargetSelectionResult{IDs: r.DefaultIDs}, nil
			}, confirmFn: func() (prompt.ConfirmationResult, error) { return prompt.ConfirmationResult{Accepted: true}, nil }}
			writeErr := errors.New("fixture output closed")
			var roots []string
			app.SourceAcquirerForDryRun = func(root string) SourceAcquirer {
				roots = append(roots, root)
				if phase == "announcement" {
					cmd.SetErr(targetNoticeWriter{err: writeErr})
				} else {
					cmd.SetOut(targetNoticeWriter{err: writeErr})
				}
				return app.SourceAcquirer
			}
			err = applySkillsWithRecovery(cmd, app, &options{format: "human", scope: "user"}, plan, true)
			if !errors.Is(err, writeErr) || len(roots) != 1 {
				t.Fatalf("did not reach the prepared retry output failure: roots=%v err=%v", roots, err)
			}
			for _, root := range roots {
				if _, err := os.Stat(root); !os.IsNotExist(err) {
					t.Fatalf("retry retained its disposable preparation at %q: %v", root, err)
				}
			}
		})
	}
}
