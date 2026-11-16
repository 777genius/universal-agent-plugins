package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environmentapp"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
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
	app.Terminal, app.ErrorOutput = true, errOut
	if err := applySkillsWithRecovery(cmd, app, &options{format: "human", scope: "user", plain: true}, plan, true); err != nil {
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

// The ordinary HTTPS/SDK fixture uses no Git or native process. Each prepared
// retry owns its disposable preview, while the original belongs to the caller.
func TestSkillsRetryWriteFailureClosesOnlyOwnedPreview(t *testing.T) {
	for _, phase := range []string{"retry notice", "retry result"} {
		t.Run(phase, func(t *testing.T) {
			var roots []string
			app, plan := retryCleanupPlan(t, &roots)
			app.Prompter = fakePrompter{
				selectFn: func(r prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
					return prompt.TargetSelectionResult{IDs: r.DefaultIDs}, nil
				},
				confirmFn: func() (prompt.ConfirmationResult, error) { return prompt.ConfirmationResult{Accepted: true}, nil },
			}
			cmd, _, _ := retryTestCommand(t)
			writeErr := errors.New("retry output failed")
			writer := retryCleanupWriter{cause: writeErr, fail: func() bool { return len(roots) == 2 }}
			if phase == "retry notice" {
				cmd.SetErr(writer)
			} else {
				cmd.SetOut(writer)
			}
			err := applySkillsWithRecovery(cmd, app, &options{format: "human", scope: "user"}, plan, true)
			if !errors.Is(err, writeErr) || (phase == "retry notice" && !errors.Is(err, installer.ErrTargetFactsUnavailable)) {
				t.Fatalf("write failure lost its own or original installation cause: %v", err)
			}
			assertRetryPreviewOwnership(t, plan, roots)
		})
	}
}

func retryCleanupPlan(t *testing.T, roots *[]string) (App, *PreparedSkillsBatch) {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/agent-skills/index.json":
			_, _ = io.WriteString(w, `{"skills":[{"name":"pdf","description":"Read PDFs","files":["SKILL.md"]}]}`)
		case "/.well-known/agent-skills/pdf/SKILL.md":
			_, _ = io.WriteString(w, "---\nname: pdf\ndescription: Read PDF documents\n---\nRead PDFs.\n")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	journey := newSavedJourney(t, t.TempDir())
	app := withInstallLogDir(journey.cli.app)
	app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: t.TempDir(), HTTPClient: server.Client()}
	app.SourceAcquirerForDryRun = func(root string) SourceAcquirer {
		*roots = append(*roots, root)
		return app.SourceAcquirer
	}
	plan, err := app.PrepareSkills(context.Background(), SkillsBatchRequest{
		Selections: []SkillsSelection{{Website: server.URL, Name: "pdf"}}, TargetIDs: []string{"codex"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(plan.Close)
	broken := plan.request.detected[domain.ClientCodex]
	broken.Status = domain.DetectionNotDetected
	plan.request.detected[domain.ClientCodex] = broken
	return app, plan
}

func assertRetryPreviewOwnership(t *testing.T, original *PreparedSkillsBatch, roots []string) {
	t.Helper()
	if len(roots) != 2 || original.closed {
		t.Fatalf("retry did not retain original caller ownership: roots=%v closed=%v", roots, original.closed)
	}
	if _, err := os.Stat(roots[0]); err != nil {
		t.Fatalf("original preview was closed by the retry: %v", err)
	}
	if _, err := os.Stat(roots[1]); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("retry-owned preview survived writer failure: %v", err)
	}
	original.Close()
	if _, err := os.Stat(roots[0]); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("original caller could not close its preview: %v", err)
	}
}

type retryCleanupWriter struct {
	cause error
	fail  func() bool
}

func (writer retryCleanupWriter) Write(body []byte) (int, error) {
	if writer.fail() {
		return 0, writer.cause
	}
	return len(body), nil
}

func TestAddRetryUnclosedProbeNeverPromptsOrRetries(t *testing.T) {
	var offered []string
	app, opts := retryAppAndOptions(t, &offered)
	prompts := 0
	app.Prompter = fakePrompter{selectFn: func(prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
		prompts++
		return prompt.TargetSelectionResult{}, nil
	}}
	cmd, _, _ := retryTestCommand(t)
	cause := errors.New("family closure unconfirmed")
	failure := fmt.Errorf("install failed: %w", errors.Join(errors.New("ordinary client failed"), &domain.UnclosedClientProbe{Operation: "readonly inventory", Err: cause}))
	calls := 0
	err := retryFailedAgents(cmd, app, opts, failure, func(string, []string) error { calls++; return nil })
	if !domain.IsUnclosedClientProbe(err) || !errors.Is(err, cause) || len(offered) != 0 || prompts != 0 || calls != 0 {
		t.Fatalf("unsafe retry lost cause or prompted: %v offered=%v prompts=%d calls=%d", err, offered, prompts, calls)
	}
}
func TestGroupedAddPreservesUnclosedProbeCause(t *testing.T) {
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCodex), fixtureClient(t, domain.ClientCursor)})
	cause := errors.New("family closure unconfirmed")
	fixture.app.Lifecycle.Activator = &cliObservedActivator{err: &domain.UnclosedClientProbe{Operation: "readonly inventory", Err: cause}}
	_, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "codex,cursor", "--format", "json")
	if !domain.IsUnclosedClientProbe(err) || !errors.Is(err, cause) {
		t.Fatalf("group failure flattened unclosed probe: %v", err)
	}
}

type unclosedSkillsStore struct {
	statev2.Store
	failure error
	checks  int
}

func (store *unclosedSkillsStore) RequireMutationReady() error {
	store.checks++
	return store.failure
}
func TestSkillsUnclosedProbeNeverPromptsOrRetries(t *testing.T) {
	fixture := newCLIFixture(t, nil)
	fixture.app.DependenciesRoot = filepath.Join(fixture.root, "dependencies")
	cause := errors.New("family closure unconfirmed")
	failure := fmt.Errorf("skills apply: %w", errors.Join(errors.New("ordinary failure"), &domain.UnclosedClientProbe{Operation: "readonly inventory", Err: cause}))
	store := &unclosedSkillsStore{Store: fixture.store, failure: failure}
	fixture.app.StateStore = store
	manifest, lock := fixture.app.dependencyPaths()
	plan := &PreparedSkillsBatch{
		app:     fixture.app,
		request: dependencySyncRequest{intent: environmentapp.Intent{ManifestPath: manifest, LockPath: lock}},
		actions: []SkillsAction{{Alias: "demo", ClientID: "codex"}, {Alias: "demo", ClientID: "claude"}},
		cleanup: func() {},
	}
	defer plan.Close()
	prompts := 0
	app := fixture.app
	app.Terminal = true
	app.Prompter = fakePrompter{selectFn: func(prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
		prompts++
		return prompt.TargetSelectionResult{}, nil
	}}
	cmd, _, _ := retryTestCommand(t)
	err := applySkillsWithRecovery(cmd, app, &options{format: "human", scope: "user"}, plan, true)
	if !domain.IsUnclosedClientProbe(err) || !errors.Is(err, cause) || prompts != 0 || store.checks != 1 {
		t.Fatalf("unsafe skills retry or lost cause: %v prompts=%d checks=%d", err, prompts, store.checks)
	}
}

func TestPrepareSkillsRetryPreservesPromptErrors(t *testing.T) {
	readFailure := errors.Join(errors.New("retry terminal read failed"), prompt.ErrPromptInputClosed)
	for name, test := range map[string]struct {
		app    App
		want   error
		single bool
	}{
		"terminal unavailable": {app: App{}, want: prompt.ErrPromptUnavailable},
		"confirmation read failed": {
			app: App{Prompter: fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
				return prompt.ConfirmationResult{}, readFailure
			}}},
			want: prompt.ErrPromptInputClosed, single: true,
		},
		"selection read failed": {
			app: App{Prompter: fakePrompter{selectFn: func(prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
				return prompt.TargetSelectionResult{}, readFailure
			}}},
			want: prompt.ErrPromptInputClosed,
		},
	} {
		t.Run(name, func(t *testing.T) {
			cmd, _, _ := retryTestCommand(t)
			reports := reportByAgent([]agentItem{failedItem("pdf", "codex", installer.ErrTargetFactsUnavailable), failedItem("pdf", "gemini", installer.ErrTargetFactsUnavailable)})
			if test.single {
				reports = reports[:1]
			}
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
			reports := reportByAgent([]agentItem{failedItem("pdf", "codex", installer.ErrTargetFactsUnavailable), failedItem("pdf", "gemini", installer.ErrTargetFactsUnavailable)})
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
			reports := reportByAgent([]agentItem{failedItem("pdf", "codex", installer.ErrTargetFactsUnavailable), failedItem("pdf", "gemini", installer.ErrTargetFactsUnavailable)})
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
			if reflect.ValueOf(humanSkillsApplyError("json", items, syncErr, "log")) != reflect.ValueOf(syncErr) || reflect.ValueOf(humanSkillsApplyError("human", nil, syncErr, "log")) != reflect.ValueOf(syncErr) || humanSkillsApplyError("human", items, nil, "log") != nil {
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

// A group refused before any client was touched offers no retry and no log:
// nothing was attempted and the same preflight would refuse again.
func TestGroupRefusedBeforeAnyChangeIsNotRetried(t *testing.T) {
	cmd, _, errOut := retryTestCommand(t)
	app := App{Terminal: true, InstallLogDir: filepath.Join(t.TempDir(), "logs")}
	prompts := 0
	app.Prompter = fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
		prompts++
		return prompt.ConfirmationResult{Accepted: true}, nil
	}}
	cause := errors.New("group refused")
	for _, status := range []string{"planned", "preparation_failed"} {
		opts := &options{format: "human"}
		combined := addMultiResult{Status: status, Plugin: "demo", Targets: []addTargetResult{{Target: "codex", Status: string(usecase.GroupTargetExternalNotAttempted)}}}
		got := app.recordGroupOutcome(cmd, opts, &combined, cause)
		if !errors.Is(got, cause) || !errors.Is(cause, got) || opts.installResult != nil || errOut.Len() != 0 {
			t.Fatalf("%s: a refused group must retain its error without recording a retry: %v %+v %q", status, got, opts.installResult, errOut.String())
		}
		if result := retryFailedAgents(cmd, app, opts, got, func(string, []string) error {
			t.Fatal("a group refused before changes must not rerun")
			return nil
		}); !errors.Is(result, cause) || prompts != 0 {
			t.Fatalf("%s: refusal prompted or lost its cause: %v prompts=%d", status, result, prompts)
		}
		if _, err := os.Stat(app.InstallLogDir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s: refused group created a log directory: %v", status, err)
		}
	}
	opts := &options{format: "human"}
	combined := addMultiResult{Status: "managed_committed_activation_failed", Plugin: "demo", Targets: []addTargetResult{{Target: "codex", Status: string(usecase.GroupTargetExternalFailed), Output: addResultData{Result: usecase.AddResult{GroupPhase: usecase.GroupTargetExternalFailed}}}}}
	got := app.recordGroupOutcome(cmd, opts, &combined, cause)
	if opts.installResult == nil || len(retryableAgents(opts.installResult.Agents)) != 1 || opts.installResult.LogFile == "" {
		t.Fatal("a failed activation after apply must stay logged and retryable")
	}
	reruns := 0
	if result := retryFailedAgents(cmd, app, opts, got, func(string, []string) error { reruns++; return nil }); result != nil || reruns != 1 || prompts != 1 {
		t.Fatalf("post-apply retry changed: err=%v reruns=%d prompts=%d", result, reruns, prompts)
	}
}

func TestRefusedGroupKeepsUnclosedCauseWithoutLoggingOrRetry(t *testing.T) {
	private := errors.New("close failed at /private/preflight token=private-value")
	for _, status := range []string{"planned", "preparation_failed"} {
		for name, cause := range unclosedInstallErrors(private) {
			t.Run(status+"/"+name, func(t *testing.T) {
				var offered []string
				app, _ := retryAppAndOptions(t, &offered)
				app.InstallLogDir = filepath.Join(t.TempDir(), "logs")
				opts := &options{format: "human"}
				cmd, out, errOut := retryTestCommand(t)
				failed := &usecase.GroupTargetFailure{Stage: "preflight", Message: private.Error()}
				combined := addMultiResult{Status: status, Plugin: "demo", Targets: []addTargetResult{{Target: "codex", Status: string(usecase.GroupTargetExternalNotAttempted), Error: failed, RetryCommand: "agentplugins add demo --target codex"}}}
				combined.Targets[0].Output.Result.Failure = failed
				got := app.recordGroupOutcome(cmd, opts, &combined, cause)
				var original, retained *domain.UnclosedClientProbe
				if !errors.As(cause, &original) || !errors.As(got, &retained) || retained != original || !errors.Is(got, private) {
					t.Fatalf("refusal lost the exact cleanup marker or cause: %v", got)
				}
				if opts.installResult != nil || out.Len() != 0 || errOut.Len() != 0 {
					t.Fatal("a refused group must not record an install outcome or log path")
				}
				if _, err := os.Stat(app.InstallLogDir); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("refused group created a log directory: %v", err)
				}
				if result := retryFailedAgents(cmd, app, opts, got, func(string, []string) error {
					t.Fatal("a refused group with unconfirmed cleanup must not rerun")
					return nil
				}); !errors.Is(result, private) || len(offered) != 0 {
					t.Fatalf("refusal prompted or lost its cause: %v offered=%v", result, offered)
				}
				target := combined.Targets[0]
				if target.RetryCommand != "" || target.Error == nil || target.Error.Stage != "cleanup" {
					t.Fatalf("refused target lost cleanup guidance: %+v", target)
				}
				wire, err := json.Marshal(combined)
				if err != nil {
					t.Fatal(err)
				}
				if err := renderAddMultiApplySummary(out, combined, domain.PackageEnvelope{}); err != nil {
					t.Fatal(err)
				}
				for _, public := range []string{got.Error(), string(wire), out.String()} {
					if strings.Contains(public, "/private/preflight") || strings.Contains(public, "private-value") {
						t.Fatalf("provider detail reached public output: %s", public)
					}
				}
			})
		}
	}
}

func TestInstallRetriesRefuseUnclosedNativeProbe(t *testing.T) {
	for _, mode := range []string{"add", "converge"} {
		for _, wrapping := range []string{"bare", "wrapped", "joined"} {
			t.Run(mode+"/"+wrapping, func(t *testing.T) {
				closeErr := errors.New("native process cleanup is unconfirmed")
				probe := &domain.UnclosedClientProbe{Operation: "native inventory", Err: closeErr}
				var cause error = probe
				switch wrapping {
				case "wrapped":
					cause = fmt.Errorf("installation failed: %w", cause)
				case "joined":
					cause = errors.Join(errors.New("another agent failed"), cause)
				}
				var offered []string
				app, opts := retryAppAndOptions(t, &offered)
				cmd, out, errOut := retryTestCommand(t)
				before, reruns := opts.installResult, 0
				var actual error
				if mode == "add" {
					actual = retryFailedAgents(cmd, app, opts, cause, func(string, []string) error { reruns++; return nil })
				} else {
					actual = retryConverge(cmd, app, opts, cause, func() error { reruns++; return nil })
				}
				var retained *domain.UnclosedClientProbe
				if !errors.Is(actual, cause) || !errors.Is(cause, actual) || !errors.As(actual, &retained) || retained != probe || !errors.Is(actual, closeErr) {
					t.Fatalf("retry lost or changed the process-cleanup refusal: %v", actual)
				}
				if len(offered) != 0 || reruns != 0 || opts.retryAttempt != 0 || opts.installResult != before || out.Len() != 0 || errOut.Len() != 0 {
					t.Fatalf("unclosed process allowed a new attempt: offered=%v reruns=%d attempt=%d", offered, reruns, opts.retryAttempt)
				}
			})
		}
	}
}
